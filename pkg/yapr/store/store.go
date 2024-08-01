package store

import (
	"noy/router/pkg/yapr/config"
	"noy/router/pkg/yapr/core"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
)

type Store interface {
	LoadConfig(config *config.YaprConfig) error
	GetRouter(name string) (*core.Router, error)
	GetSelectors() (map[string]*core.Selector, error)
	GetServices() (map[string]*core.Service, error)

	RegisterService(service string, endpoints []*types.Endpoint) error
	RegisterServiceChangeListener(listener func(service string, isPut bool, pod string, endpoints []*types.Endpoint)) // 如果autoupdate为true则不用监听
	SetEndpointAttribute(endpoint *types.Endpoint, selector string, attribute *types.Attribute) error
	RegisterAttributeChangeListener(listener func(endpoint *types.Endpoint, selector string, attribute *types.Attribute)) // 如果autoupdate为true则不用监听

	SetCustomRoute(selectorName, headerValue string, endpoint *types.Endpoint, timeout int64) error
	GetCustomRoute(selectorName, headerValue string) (*types.Endpoint, error)
	RemoveCustomRoute(selectorName, headerValue string) error

	Close()
}

var instance Store

func MustStore() Store {
	return instance
}

func RegisterStore(store Store) {
	if instance != nil {
		panic("store already registered")
	}
	instance = store
}

var routers = make(map[string]*core.Router)

func GetRouter(name string) (*core.Router, error) {
	if router, ok := routers[name]; ok {
		return router, nil
	}
	s := instance
	router, err := s.GetRouter(name)
	if err != nil {
		logger.Errorf("router %s not found", name)
		return nil, err
	}
	routers[name] = router
	return router, nil
}
