package yaprsdk

import (
	"math"
	"net"
	"noy/router/pkg/yapr/core"
	"noy/router/pkg/yapr/core/config"
	_ "noy/router/pkg/yapr/core/grpc"
	"noy/router/pkg/yapr/core/store"
	"noy/router/pkg/yapr/core/store/impl"
	"noy/router/pkg/yapr/core/types"
	"sync"
)

var initOnce sync.Once

func Init(configPath string) {
	initOnce.Do(func() {
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			panic(err)
		}
		st, err := impl.NewImpl(cfg)
		store.RegisterStore(st)
		if err != nil {
			panic(err)
		}
	})
}

func GetLocalEndpoint() (*types.Endpoint, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return &types.Endpoint{
					IP: ipNet.IP.String(),
				}, nil
			}
		}
	}
	return nil, core.ErrNoEndpointAvailable
}

func RegisterService(serviceName string, endpoints []*types.Endpoint) error {
	return store.MustStore().RegisterService(serviceName, endpoints)
}

func SetEndpointAttribute(endpoint *types.Endpoint, selector string, attribute *types.Attribute) error {
	return store.MustStore().SetEndpointAttribute(endpoint, selector, attribute)
}

func ReportCost(endpoint *types.Endpoint, selector string, cost uint32) error {
	return SetEndpointAttribute(endpoint, selector, &types.Attribute{
		Weight: math.MaxUint32 - cost,
	})
}

func SetCustomRoute(selectorName, headerValue string, endpoint *types.Endpoint, timeout int64, ignoreExisting bool) (bool, *types.Endpoint, error) {
	return store.MustStore().SetCustomRoute(selectorName, headerValue, endpoint, timeout, ignoreExisting)
}
