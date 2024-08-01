package core

type Store interface {
	LoadConfig(config *Config) error
	GetRouter(name string) (*Router, error)
	GetSelectors() (map[string]*Selector, error)
	GetServices() (map[string]*Service, error)

	RegisterService(service string, endpoints []*Endpoint) error
	RegisterServiceChangeListener(listener func(service string, isPut bool, pod string, endpoints []*Endpoint)) // 如果autoupdate为true则不用监听
	SetEndpointAttribute(endpoint *Endpoint, selector string, attribute *Attribute) error
	RegisterAttributeChangeListener(listener func(endpoint *Endpoint, selector string, attribute *Attribute)) // 如果autoupdate为true则不用监听

	SetCustomRoute(selectorName, headerValue string, endpoint *Endpoint, timeout int64) error
	GetCustomRoute(selectorName, headerValue string) (*Endpoint, error)

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
