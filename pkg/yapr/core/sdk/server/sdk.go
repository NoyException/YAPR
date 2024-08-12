package yaprsdk

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"noy/router/pkg/yapr/core"
	"noy/router/pkg/yapr/core/config"
	"noy/router/pkg/yapr/core/errcode"
	_ "noy/router/pkg/yapr/core/grpc"
	"noy/router/pkg/yapr/core/store"
	"noy/router/pkg/yapr/core/store/impl"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
	"sync"
	"sync/atomic"
	"time"
)

var yaprSDK *YaprSDK

type YaprSDK struct {
	pod                string
	endpoints          map[types.Endpoint]struct{} // 本地服务端的所有Endpoint
	endpointsByService map[string][]*types.Endpoint

	routingTable   RoutingTable // 与本地服务端的Endpoint有关的路由表
	routingTableMu sync.RWMutex

	requestCounter sync.Map // selector_endpoint -> atomic.Uint32

	migrationListener func(selectorName, headerValue string, from, to *types.Endpoint)
	cancel            store.CancelFunc
}

// Init 初始化路由配置
func Init(configPath, pod string) *YaprSDK {
	if yaprSDK != nil {
		return yaprSDK
	}
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		panic(err)
	}
	st, err := store_impl.NewImpl(cfg, pod)
	store.RegisterStore(st)
	if err != nil {
		panic(err)
	}

	yaprSDK = &YaprSDK{
		endpoints:          make(map[types.Endpoint]struct{}),
		endpointsByService: make(map[string][]*types.Endpoint),
		routingTable:       NewRoutingTable(),
		pod:                pod,

		routingTableMu: sync.RWMutex{},

		cancel: st.RegisterMigrationListener(yaprSDK.onMigration),
	}
	return yaprSDK
}

func MustInstance() *YaprSDK {
	if yaprSDK == nil {
		panic("yapr sdk not initialized")
	}
	return yaprSDK
}

//func GetLocalEndpoint() (*types.Endpoint, error) {
//	addrs, err := net.InterfaceAddrs()
//	if err != nil {
//		return nil, err
//	}
//	for _, addr := range addrs {
//		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
//			if ipNet.IP.To4() != nil {
//				return &types.Endpoint{
//					IP: ipNet.IP.String(),
//				}, nil
//			}
//		}
//	}
//	return nil, core.ErrNoEndpointAvailable
//}

func (y *YaprSDK) onMigration(selectorName, headerValue string, from, to *types.Endpoint) {
	if types.EqualEndpoints(from, to) {
		return
	}
	relative := false

	y.routingTableMu.Lock()
	if from != nil {
		if _, ok := y.endpoints[*from]; ok {
			relative = true
			y.routingTable.DeleteRoute(selectorName, headerValue)
		}
	}
	if to != nil {
		if _, ok := y.endpoints[*to]; ok && !types.EqualEndpoints(to, y.routingTable.GetRoute(selectorName, headerValue)) {
			relative = true
			y.routingTable.AddRoute(selectorName, headerValue, to)
		}
	}
	y.routingTableMu.Unlock()

	if relative && y.migrationListener != nil {
		y.migrationListener(selectorName, headerValue, from, to)
	}
}

func (y *YaprSDK) GRPCServerInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	// 从ctx中获取路由信息selectorName, headerValue
	md, exists := metadata.FromIncomingContext(ctx)
	if !exists {
		logger.Errorf("metadata not found")
		return nil, errcode.ErrMetadataNotFound
	}
	strategy := md.Get("yapr-strategy")
	if strategy == nil || len(strategy) == 0 {
		logger.Errorf("strategy not found")
		return nil, errcode.ErrInvalidMetadata
	}
	selectorName := md.Get("yapr-selector")[0]
	rawEndpoint := md.Get("yapr-endpoint")[0]
	endpoint := types.EndpointFromString(rawEndpoint)

	switch strategy[0] {
	case types.StrategyDirect:
		headerValue := md.Get("yapr-header-value")[0]

		y.routingTableMu.RLock()
		expectEndpoint := y.routingTable.GetRoute(selectorName, headerValue)
		y.routingTableMu.RUnlock()

		if _, ok := y.endpoints[*endpoint]; !ok || !types.EqualEndpoints(endpoint, expectEndpoint) {
			logger.Errorf("wrong endpoint: %v", endpoint)
			return nil, errcode.WithData(errcode.ErrWrongEndpoint, map[string]string{
				"selectorName": selectorName,
				"headerValue":  headerValue,
				"endpoint":     rawEndpoint,
			}).ToGRPCError()
		}
	case types.StrategyLeastRequest:
		key := selectorName + "_" + rawEndpoint
		actual, loaded := y.requestCounter.LoadOrStore(key, &atomic.Uint32{})
		counter := actual.(*atomic.Uint32)
		counter.Add(1)
		if loaded {
			go func() {
				<-time.After(time.Second)
				y.requestCounter.Delete(key)
				err := y.ReportCost(endpoint, selectorName, counter.Load())
				logger.Debugf("report cost: %v->%v", key, counter.Load())
				if err != nil {
					logger.Errorf("report cost failed: %v", err)
				}
			}()
		}
	}
	return handler(ctx, req)
}

// RegisterService 服务注册，对某个serviceName只能注册一次。当返回的 channel 被关闭时，表示服务掉线，需要重新注册
func (y *YaprSDK) RegisterService(serviceName string, endpoints []*types.Endpoint) (chan struct{}, error) {
	ch, err := store.MustStore().RegisterService(serviceName, endpoints)
	if err != nil {
		return nil, err
	}
	for _, endpoint := range endpoints {
		y.endpoints[*endpoint] = struct{}{}
	}
	y.endpointsByService[serviceName] = endpoints
	return ch, nil
}

// UnregisterService 服务注销
func (y *YaprSDK) UnregisterService(serviceName string) error {
	err := store.MustStore().UnregisterService(serviceName)
	if err != nil {
		return err
	}
	endpoints := y.endpointsByService[serviceName]
	for _, endpoint := range endpoints {
		delete(y.endpoints, *endpoint)
	}
	delete(y.endpointsByService, serviceName)
	return nil
}

// SetEndpointAttribute 设置服务端某Endpoint属性
func (y *YaprSDK) SetEndpointAttribute(endpoint *types.Endpoint, selector string, attribute *types.AttributeInSelector) error {
	return store.MustStore().SetEndpointAttribute(endpoint, selector, attribute)
}

// ReportCost 上报服务端某Endpoint的开销，仅限least_cost/least_request路由策略使用
func (y *YaprSDK) ReportCost(endpoint *types.Endpoint, selector string, cost uint32) error {
	return y.SetEndpointAttribute(endpoint, selector, &types.AttributeInSelector{
		Weight: cost,
	})
}

// SetCustomRoute 设置自定义路由
func (y *YaprSDK) SetCustomRoute(selectorName, headerValue string, endpoint *types.Endpoint, timeout int64, ignoreExisting bool) (bool, *types.Endpoint, error) {
	st := store.MustStore()
	success, old, err := st.SetCustomRoute(selectorName, headerValue, endpoint, timeout, ignoreExisting)
	if err != nil {
		return false, nil, err
	}
	if success && old != nil && !types.EqualEndpoints(old, endpoint) {
		y.onMigration(selectorName, headerValue, old, endpoint)
		if err := st.NotifyMigration(selectorName, headerValue, old, endpoint); err != nil {
			logger.Errorf("notify migration failed: %v", err)
		}
	}
	return success, old, nil
}

func (y *YaprSDK) SetMigrationListener(listener func(selectorName, headerValue string, from, to *types.Endpoint)) {
	y.migrationListener = listener
}

func (y *YaprSDK) GetEndpoints(selectorName string) map[types.Endpoint]*types.Attribute {
	selector, err := core.GetSelector(selectorName)
	if err != nil {
		return nil
	}
	return selector.EndpointsWithAttribute()
}

func (y *YaprSDK) NewEndpoint(ip string) *types.Endpoint {
	return &types.Endpoint{
		IP:  ip,
		Pod: y.pod,
	}
}

func (y *YaprSDK) NewEndpointWithPort(ip string, port uint32) *types.Endpoint {
	return &types.Endpoint{
		IP:   ip,
		Pod:  y.pod,
		Port: &port,
	}
}