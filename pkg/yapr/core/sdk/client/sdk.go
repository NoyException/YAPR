package yaprsdk

import (
	"context"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"noy/router/pkg/yapr/core"
	"noy/router/pkg/yapr/core/config"
	"noy/router/pkg/yapr/core/errcode"
	yaprgrpc "noy/router/pkg/yapr/core/grpc"
	"noy/router/pkg/yapr/core/store"
	"noy/router/pkg/yapr/core/store/impl"
	"noy/router/pkg/yapr/core/strategy"
	"noy/router/pkg/yapr/core/strategy/impl"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
	"time"
)

var yaprSDK *YaprSDK

type YaprSDK struct {
	pod string
}

// Init 初始化路由配置
func Init(configPath string, initGrpc bool) *YaprSDK {
	if yaprSDK != nil {
		return yaprSDK
	}
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		panic(err)
	}
	pod := uuid.New().String()
	st, err := store_impl.NewImpl(cfg, pod)
	store.RegisterStore(st)
	if err != nil {
		panic(err)
	}
	core.RegisterStrategyBuilder(types.StrategyRandom, &builtin.RandomStrategyBuilder{})
	core.RegisterStrategyBuilder(types.StrategyRoundRobin, &builtin.RoundRobinStrategyBuilder{})
	core.RegisterStrategyBuilder(types.StrategyWeightedRandom, &builtin.WeightedRandomStrategyBuilder{})
	core.RegisterStrategyBuilder(types.StrategyWeightedRoundRobin, &builtin.WeightedRoundRobinStrategyBuilder{})
	core.RegisterStrategyBuilder(types.StrategyLeastRequest, &builtin.LeastRequestStrategyBuilder{})
	core.RegisterStrategyBuilder(types.StrategyLeastCost, &builtin.LeastCostStrategyBuilder{})
	core.RegisterStrategyBuilder(types.StrategyHashRing, &builtin.HashRingStrategyBuilder{})
	core.RegisterStrategyBuilder(types.StrategyDirect, &builtin.DirectStrategyBuilder{})
	core.RegisterStrategyBuilder(types.StrategyCustomLua, &builtin.CustomLuaStrategyBuilder{})

	if initGrpc {
		yaprgrpc.InitResolver()
		yaprgrpc.InitBalancer()
	}

	yaprSDK = &YaprSDK{
		pod: pod,
	}
	return yaprSDK
}

func MustInstance() *YaprSDK {
	if yaprSDK == nil {
		panic("yapr sdk not initialized")
	}
	return yaprSDK
}

func (y *YaprSDK) GRPCClientInterceptor(
	ctx context.Context,
	method string,
	req, reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	return y.invoke(func(headers map[string]string) error {
		return invoker(ctx, method, req, reply, cc, opts...)
	})
}

func (y *YaprSDK) invoke(invoker func(headers map[string]string) error) error {
	headers := make(map[string]string)
	for i := 0; i < 3; i++ {
		err := invoker(headers)
		if err == nil {
			return nil
		}

		errWithCode := errcode.FromGRPCError(err)
		if errWithCode == nil || errWithCode.Code != errcode.ErrWrongEndpoint.Code {
			return err
		}

		logger.Warnf("wrong endpoint: %v", err)

		switch i {
		case 1:
			time.Sleep(100 * time.Millisecond)
		case 2:
			time.Sleep(500 * time.Millisecond)
		}

		// 使缓存失效
		selectorName := errWithCode.Data["selectorName"]
		headerValue := errWithCode.Data["headerValue"]

		selector, err := core.GetSelector(selectorName)
		if err != nil {
			logger.Errorf("get selector failed: %v", err)
			return err
		}
		selector.NotifyRetry(headerValue)

		// 记录上次被打回的来源
		headers["yapr-last-endpoint"] = errWithCode.Data["endpoint"]
		logger.Debugf("retry %d times", i+1)
	}
	return errcode.ErrMaxRetries
}

// Invoke 当不使用gRPC时，需要使用Invoke方法进行调用。请确保headers要原封不动传递给服务端
func (y *YaprSDK) Invoke(routerName string, match *types.MatchTarget, requestSender func(serviceName string, endpoint *types.Endpoint, port uint32, headers map[string]string) error) error {
	serviceName, endpoint, port, headers, err := y.route(routerName, match)
	if err != nil {
		return err
	}
	return y.invoke(func(h map[string]string) error {
		for k, v := range h {
			headers[k] = v
		}
		return requestSender(serviceName, endpoint, port, headers)
	})
}

func (y *YaprSDK) route(routerName string, match *types.MatchTarget) (serviceName string, endpoint *types.Endpoint, port uint32, headers map[string]string, err error) {
	router, err := core.GetRouter(routerName)
	if err != nil {
		return
	}
	serviceName, endpoint, port, headers, err = router.Route(match)
	return
}

// SetCustomRoute 设置自定义路由，old为nil则表示只在没被设置时设置，timeout为0则表示永不超时
func (y *YaprSDK) SetCustomRoute(selectorName, headerValue string, endpoint, old *types.Endpoint, timeout int64, ignoreExisting bool) (*types.Endpoint, error) {
	st := store.MustStore()
	realOld, err := st.SetCustomRoute(selectorName, headerValue, endpoint, old, timeout)
	if err != nil {
		return nil, err
	}
	if old != nil && realOld == nil {
		if err := st.NotifyMigration(selectorName, headerValue, old, endpoint); err != nil {
			logger.Errorf("notify migration failed: %v", err)
		}
	}
	return realOld, nil
}

// GetEndpoints 获取路由配置，用于自定义路由
func (y *YaprSDK) GetEndpoints(serviceName string) []*types.Endpoint {
	service, err := core.GetService(serviceName)
	if err != nil {
		logger.Errorf("get service failed: %v", err)
		return nil
	}
	return service.Endpoints()
}

// GetEndpointsWithAttribute 获取路由配置，用于自定义路由
func (y *YaprSDK) GetEndpointsWithAttribute(selectorName string) map[types.Endpoint]*types.Attribute {
	selector, err := core.GetSelector(selectorName)
	if err != nil {
		logger.Errorf("get selector failed: %v", err)
		return nil
	}
	return selector.EndpointsWithAttribute()
}

// RegisterRoutingStrategy 注册自定义路由策略
func (y *YaprSDK) RegisterRoutingStrategy(name string, strategyBuilder strategy.Builder) {
	core.RegisterStrategyBuilder(name, strategyBuilder)
}
