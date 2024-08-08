package core

import (
	"google.golang.org/grpc/metadata"
	"noy/router/pkg/yapr/core/errcode"
	"noy/router/pkg/yapr/core/store"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
	"noy/router/pkg/yapr/metrics"
	"time"
)

// Selector 全名是Endpoint Selector，指定了目标service和选择策略【以json格式存etcd】
type Selector struct {
	*types.Selector

	strategy Strategy
	lastIdx  uint32 // 上次选择的endpoint索引
}

var selectors map[string]*Selector

func GetSelector(name string) (*Selector, error) {
	if selectors == nil {
		s, err := store.MustStore().GetSelectors()
		if err != nil {
			logger.Errorf("selectors not found")
			return nil, err
		}
		selectors = make(map[string]*Selector)
		for selectorName, raw := range s {
			selectors[selectorName] = &Selector{Selector: raw}
		}
	}
	if selector, ok := selectors[name]; ok {
		return selector, nil
	}
	return nil, errcode.ErrSelectorNotFound
}

func (s *Selector) RefreshCache(headerValue string) {
	if s.Cache == nil {
		return
	}
	s.Cache.Refresh(headerValue)
}

func (s *Selector) Endpoints() map[types.Endpoint]*types.Attribute {
	result := make(map[types.Endpoint]*types.Attribute)
	service, err := GetService(s.Service)
	if err != nil {
		logger.Errorf("get service %s failed: %v", s.Service, err)
		return result
	}
	endpoints := service.Endpoints()
	if len(endpoints) == 0 {
		return result
	}
	attributes := service.AttributesInSelector(s.Name)
	for i := 0; i < len(endpoints); i++ {
		result[*endpoints[i]] = attributes[i]
	}
	return result
}

func (s *Selector) Select(target *types.MatchTarget) (endpoint *types.Endpoint, headers map[string]string, err error) {
	start := time.Now()
	defer func() {
		metrics.ObserveSelectorDuration(s.Strategy, time.Since(start).Seconds())
	}()

	service, err := GetService(s.Service)
	if err != nil {
		logger.Errorf("get service %s failed: %v", s.Service, err)
		return nil, nil, err
	}

	if s.strategy == nil {
		s.strategy, err = GetStrategy(s)
		if err != nil {
			logger.Errorf("get strategy %s failed: %v", s.Strategy, err)
			return nil, nil, err
		}
	}
	endpoint, headers, err = s.strategy.Select(service, target)

	if endpoint != nil && err == nil && !service.AttrMap[*endpoint].Available {
		err = errcode.ErrEndpointUnavailable
	}
	return
}

func (s *Selector) BaseHeaders() map[string]string {
	headers := map[string]string{
		"yapr-strategy": s.Strategy,
		"yapr-selector": s.Name,
		"yapr-service":  s.Service,
	}
	for k, v := range s.Headers {
		headers[k] = v
	}
	return headers
}

func (s *Selector) HeaderValue(target *types.MatchTarget) (string, error) {
	if s.Key == "" {
		return "", errcode.ErrNoKeyAvailable
	}

	md, exist := metadata.FromOutgoingContext(target.Ctx)
	if !exist {
		return "", errcode.ErrNoValueAvailable
	}
	values := md.Get(s.Key)
	if len(values) == 0 {
		return "", errcode.ErrNoValueAvailable
	}
	return values[0], nil
}
