package core

import (
	"noy/router/pkg/yapr/core/errcode"
	"noy/router/pkg/yapr/core/store"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
	"sync"
)

type Service struct {
	*types.Service
}

var services map[string]*Service
var serviceMu = &sync.Mutex{}

func GetService(name string) (*Service, error) {
	serviceMu.Lock()
	defer serviceMu.Unlock()

	if services == nil {
		s, err := store.MustStore().GetServices()
		if err != nil {
			logger.Errorf("services not found")
			return nil, err
		}
		services = make(map[string]*Service)
		for serviceName, raw := range s {
			service := &Service{Service: raw}
			services[serviceName] = service
		}
	}
	if service, ok := services[name]; ok {
		return service, nil
	}
	logger.Errorf("service not found: %s", name)
	return nil, errcode.ErrServiceNotFound
}
