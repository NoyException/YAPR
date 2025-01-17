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

	routingTable *RoutingTable // 与本地服务端的Endpoint有关的路由表

	requestCounter         sync.Map // selector_endpoint -> atomic.Int32
	leastRequestReportMark sync.Map // selector_endpoint -> struct{}

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
	}
	yaprSDK.cancel = st.RegisterMigrationListener(yaprSDK.onMigration)
	return yaprSDK
}

func MustInstance() *YaprSDK {
	if yaprSDK == nil {
		panic("yapr sdk not initialized")
	}
	return yaprSDK
}

func (y *YaprSDK) onMigration(selectorName, headerValue string, from, to *types.Endpoint) {
	if types.EqualEndpoints(from, to) {
		return
	}
	relative := false

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
	headers := make(map[string]string)
	for k, v := range md {
		headers[k] = v[0]
	}
	sent, err := y.OnRequestReceived(headers)
	if err != nil {
		return nil, err
	}
	if sent != nil {
		defer sent()
	}
	return handler(ctx, req)
}

// OnRequestReceived 处理请求，根据请求头中的路由信息，判断请求是否合法。如果不用gRPC则需要在收到请求后，处理请求前调用此方法，如果返回错误，需要将错误返回给客户端
func (y *YaprSDK) OnRequestReceived(headers map[string]string) (onResponseSent func(), err error) {
	strategy, ok := headers["yapr-strategy"]
	if !ok {
		return nil, errcode.ErrInvalidMetadata
	}
	selectorName := headers["yapr-selector"]
	rawEndpoint := headers["yapr-endpoint"]
	endpoint := types.EndpointFromString(rawEndpoint)

	selector, err := core.GetSelector(selectorName)
	if err != nil {
		logger.Errorf("selector not found: %v", selectorName)
		return nil, errcode.ErrSelectorNotFound
	}
	shouldReportRPS := selector.MaxRequests != nil

	switch strategy {
	case types.StrategyDirect:
		headerValue := headers["yapr-header-value"]
		lastEndpoint := headers["yapr-last-endpoint"]
		expectEndpoint := y.routingTable.GetRoute(selectorName, headerValue)
		if expectEndpoint == nil || types.EqualEndpoints(types.EndpointFromString(lastEndpoint), endpoint) {
			expectEndpoint = y.routingTable.Refresh(selectorName, headerValue)
		}

		if !types.EqualEndpoints(endpoint, expectEndpoint) {
			logger.Errorf("wrong endpoint: %v, expect: %v, ok: %v", endpoint, expectEndpoint, ok)
			return nil, errcode.WithData(errcode.ErrWrongEndpoint, map[string]string{
				"selectorName": selectorName,
				"headerValue":  headerValue,
				"endpoint":     rawEndpoint,
			}).ToGRPCError()
		}
	case types.StrategyLeastRequest:
		shouldReportRPS = true
	}

	//logger.Infof("selector: %v, endpoint: %v", selectorName, rawEndpoint)
	if shouldReportRPS {
		key := selectorName + "_" + rawEndpoint
		actual, _ := y.requestCounter.LoadOrStore(key, &atomic.Int32{})
		counter := actual.(*atomic.Int32)
		counter.Add(1)
		checkUpdate := func() {
			_, loaded := y.leastRequestReportMark.LoadOrStore(key, struct{}{})
			if !loaded {
				go func() {
					<-time.After(time.Second)
					y.leastRequestReportMark.Delete(key)
					err := y.reportRPS(endpoint, selectorName, uint32(counter.Load()))
					//logger.Infof("report rps: %v->%v", key, counter.Load())
					if err != nil {
						logger.Errorf("report rps failed: %v", err)
					}
				}()
			}
		}
		onResponseSent = func() {
			counter.Add(-1)
			checkUpdate()
		}
		checkUpdate()
	}
	err = nil
	return
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

// ReportWeight 上报服务端某Endpoint的开销，包括least_cost（只不过least_cost是weight越小越优先）
func (y *YaprSDK) ReportWeight(endpoint *types.Endpoint, selector string, cost uint32) error {
	return y.SetEndpointAttribute(endpoint, selector, &types.AttributeInSelector{
		Weight: &cost,
	})
}

func (y *YaprSDK) reportRPS(endpoint *types.Endpoint, selector string, rps uint32) error {
	return y.SetEndpointAttribute(endpoint, selector, &types.AttributeInSelector{
		RPS: &rps,
	})
}

// SetCustomRoute 设置自定义路由，old为nil则表示只在没被设置时设置，timeout为0则表示永不超时
func (y *YaprSDK) SetCustomRoute(selectorName, headerValue string, endpoint, old *types.Endpoint, timeout int64) (*types.Endpoint, error) {
	st := store.MustStore()
	realOld, err := st.SetCustomRoute(selectorName, headerValue, endpoint, old, timeout)
	if err != nil {
		return nil, err
	}
	if old != nil && realOld == nil {
		y.onMigration(selectorName, headerValue, old, endpoint)
		if err := st.NotifyMigration(selectorName, headerValue, old, endpoint); err != nil {
			logger.Errorf("notify migration failed: %v", err)
		}
	}
	return realOld, nil
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
		Port: port,
	}
}
