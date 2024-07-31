package store

import (
	"noy/router/pkg/yapr/config"
	"noy/router/pkg/yapr/core"
	"noy/router/pkg/yapr/logger"
)

type Store interface {
	LoadConfig(config *core.Config) error
	GetRouter(name string) (*core.Router, error)
	GetSelectors() (map[string]*core.Selector, error)
	GetServices() (map[string]*core.Service, error)

	RegisterService(service string, endpoints []*core.Endpoint) error
	RegisterServiceChangeListener(listener func(service string, isPut bool, pod string, endpoints []*core.Endpoint)) // 如果autoupdate为true则不用监听
	SetEndpointAttribute(endpoint *core.Endpoint, selector string, attribute *core.Attr) error
	RegisterAttributeChangeListener(listener func(endpoint *core.Endpoint, selector string, attribute *core.Attr)) // 如果autoupdate为true则不用监听

	SetCustomRoute(selectorName, headerValue string, endpoint *core.Endpoint, timeout int64) error

	Close()
}

var instance Store

func Init(cfg *config.Config) (Store, error) {
	impl, err := NewImpl(cfg)
	if err != nil {
		return nil, err
	}
	instance = impl
	// 将数据存入etcd
	err = impl.LoadConfig(cfg.Yapr)
	if err != nil {
		panic(err)
	}
	return impl, nil
}

func MustStore() Store {
	return instance
}

var routers = make(map[string]*core.Router)

func GetRouter(name string) (*core.Router, error) {
	if router, ok := routers[name]; ok {
		return router, nil
	}
	s := MustStore()
	router, err := s.GetRouter(name)
	if err != nil {
		logger.Errorf("router %s not found", name)
		return nil, err
	}
	routers[name] = router
	return router, nil
}
