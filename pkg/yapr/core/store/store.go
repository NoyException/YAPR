package store

import (
	"noy/router/pkg/yapr/core/config"
	"noy/router/pkg/yapr/core/types"
)

type CancelFunc func()

type Store interface {
	LoadConfig(config *config.YaprConfig) error
	GetRouter(name string) (*types.Router, error)
	GetSelectors() (map[string]*types.Selector, error)
	GetServices() (map[string]*types.Service, error)

	RegisterService(service string, endpoints []*types.Endpoint) (chan struct{}, error)
	UnregisterService(service string) error
	//RegisterServiceChangeListener(listener func(service string, isPut bool, pod string, endpoints []*types.Endpoint))
	SetEndpointAttribute(endpoint *types.Endpoint, selector string, attribute *types.AttributeInSelector) error
	//RegisterAttributeChangeListener(listener func(endpoint *types.Endpoint, selector string, attribute *types.AttributeInSelector))

	SetCustomRoute(selectorName, headerValue string, endpoint, old *types.Endpoint, timeout int64) (*types.Endpoint, error)
	GetCustomRoute(selectorName, headerValue string) (*types.Endpoint, error)
	RemoveCustomRoute(selectorName, headerValue string) error

	RegisterMigrationListener(listener func(selectorName, headerValue string, from, to *types.Endpoint)) CancelFunc
	NotifyMigration(selectorName, headerValue string, from, to *types.Endpoint) error

	Close()
}

var instance Store

func MustStore() Store {
	if instance == nil {
		panic("store not registered")
	}
	return instance
}

func RegisterStore(store Store) {
	if instance != nil {
		panic("store already registered")
	}
	instance = store
}
