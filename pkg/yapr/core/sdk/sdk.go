package yaprsdk

import (
	"math"
	"net"
	"noy/router/pkg/yapr/config"
	"noy/router/pkg/yapr/core"
	_ "noy/router/pkg/yapr/core/grpc"
	"noy/router/pkg/yapr/store"
	"sync"
)

var initOnce sync.Once

func Init(configPath string) {
	initOnce.Do(func() {
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			panic(err)
		}
		st, err := store.NewImpl(cfg)
		core.RegisterStore(st)
		if err != nil {
			panic(err)
		}
	})
}

func GetLocalEndpoint() (*core.Endpoint, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return &core.Endpoint{
					IP: ipNet.IP.String(),
				}, nil
			}
		}
	}
	return nil, core.ErrNoEndpointAvailable
}

func RegisterService(serviceName string, endpoints []*core.Endpoint) error {
	return core.MustStore().RegisterService(serviceName, endpoints)
}

func SetEndpointAttribute(endpoint *core.Endpoint, selector string, attribute *core.Attribute) error {
	return core.MustStore().SetEndpointAttribute(endpoint, selector, attribute)
}

func ReportCost(endpoint *core.Endpoint, selector string, cost uint32) error {
	return SetEndpointAttribute(endpoint, selector, &core.Attribute{
		Weight: math.MaxUint32 - cost,
	})
}

func SetCustomRoute(selectorName, headerValue string, endpoint *core.Endpoint, timeout int64) error {
	return core.MustStore().SetCustomRoute(selectorName, headerValue, endpoint, timeout)
}
