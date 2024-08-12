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
func Init(configPath string) *YaprSDK {
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
	core.RegisterStrategyBuilder(types.StrategyLeastRequest, &builtin.LeastCostStrategyBuilder{}) // LeastRequest 与 LeastCost 共用一个 builder
	core.RegisterStrategyBuilder(types.StrategyLeastCost, &builtin.LeastCostStrategyBuilder{})
	core.RegisterStrategyBuilder(types.StrategyHashRing, &builtin.HashRingStrategyBuilder{})
	core.RegisterStrategyBuilder(types.StrategyDirect, &builtin.DirectStrategyBuilder{})
	core.RegisterStrategyBuilder(types.StrategyCustomLua, &builtin.CustomLuaStrategyBuilder{})

	yaprgrpc.InitResolver()
	yaprgrpc.InitBalancer()

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
	for i := 0; i < 3; i++ {
		err := invoker(ctx, method, req, reply, cc, opts...)
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
		logger.Debugf("retry %d times", i+1)
	}
	return errcode.ErrMaxRetries
}

// SetCustomRoute 设置自定义路由
func (y *YaprSDK) SetCustomRoute(selectorName, headerValue string, endpoint *types.Endpoint, timeout int64, ignoreExisting bool) (bool, *types.Endpoint, error) {
	st := store.MustStore()
	success, old, err := st.SetCustomRoute(selectorName, headerValue, endpoint, timeout, ignoreExisting)
	if err != nil {
		return false, nil, err
	}
	if success && old != nil && !types.EqualEndpoints(old, endpoint) {
		if err := st.NotifyMigration(selectorName, headerValue, old, endpoint); err != nil {
			logger.Errorf("notify migration failed: %v", err)
		}
	}
	return success, old, nil
}

// GetEndpoints 获取路由配置，用于自定义路由
func (y *YaprSDK) GetEndpoints(selectorName string) map[types.Endpoint]*types.Attribute {
	selector, err := core.GetSelector(selectorName)
	if err != nil {
		return nil
	}
	return selector.EndpointsWithAttribute()
}

// RegisterRoutingStrategy 注册自定义路由策略
func (y *YaprSDK) RegisterRoutingStrategy(name string, strategyBuilder strategy.StrategyBuilder) {
	core.RegisterStrategyBuilder(name, strategyBuilder)
}