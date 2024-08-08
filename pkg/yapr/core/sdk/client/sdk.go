package yaprsdk

import (
	"context"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"noy/router/pkg/yapr/core"
	"noy/router/pkg/yapr/core/config"
	"noy/router/pkg/yapr/core/errcode"
	_ "noy/router/pkg/yapr/core/grpc"
	"noy/router/pkg/yapr/core/store"
	"noy/router/pkg/yapr/core/store/impl"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
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
	st, err := impl.NewImpl(cfg, pod)
	store.RegisterStore(st)
	if err != nil {
		panic(err)
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
		// 使缓存失效
		selectorName := errWithCode.Data["selectorName"]
		headerValue := errWithCode.Data["headerValue"]

		selector, err := core.GetSelector(selectorName)
		if err != nil {
			logger.Errorf("get selector failed: %v", err)
			return err
		}
		selector.RefreshCache(headerValue)

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
	if success && old != nil && !old.Equal(endpoint) {
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
	return selector.Endpoints()
}

// RegisterRoutingStrategy 注册自定义路由策略
func (y *YaprSDK) RegisterRoutingStrategy(name string, strategyBuilder core.StrategyBuilder) {
	core.RegisterStrategyBuilder(name, strategyBuilder)
}
