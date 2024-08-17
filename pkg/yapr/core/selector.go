package core

import (
	"noy/router/pkg/yapr/core/errcode"
	"noy/router/pkg/yapr/core/store"
	"noy/router/pkg/yapr/core/strategy"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
	"noy/router/pkg/yapr/metrics"
	"sync"
	"time"
)

// Selector 全名是Endpoint Selector，指定了目标service和选择策略【以json格式存etcd】
type Selector struct {
	*types.Selector

	mu          sync.Mutex
	lastVersion uint64
	lastUpdate  time.Time
	service     *Service
	strategy    strategy.Strategy
}

var selectors map[string]*Selector
var selectorMu = &sync.Mutex{}

func GetSelector(name string) (*Selector, error) {
	selectorMu.Lock()
	defer selectorMu.Unlock()

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

func (s *Selector) MustService() *Service {
	if s.service != nil {
		return s.service
	}
	s.mu.Lock()
	if s.service == nil {
		service, err := GetService(s.Service)
		if err != nil {
			panic(err)
		}
		s.service = service
	}
	s.mu.Unlock()
	return s.service
}

func (s *Selector) NotifyRetry(headerValue string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ss, ok := s.strategy.(strategy.StatefulStrategy); ok {
		ss.NotifyRetry(headerValue)
	}
}

func (s *Selector) Endpoints() []*types.Endpoint {
	return s.MustService().Endpoints()
}

func (s *Selector) EndpointsWithAttribute(filters ...types.EndpointFilter) map[types.Endpoint]*types.Attribute {
	result := make(map[types.Endpoint]*types.Attribute)
	endpoints, attributes := s.MustService().Attributes(s.Name)
OUTER:
	for i := 0; i < len(endpoints); i++ {
		for _, filter := range filters {
			if !filter(s.Selector, endpoints[i], attributes[i]) {
				continue OUTER
			}
		}
		result[*endpoints[i]] = attributes[i]
	}
	return result
}

func (s *Selector) Select(target *types.MatchTarget) (endpoint *types.Endpoint, headers map[string]string, err error) {
	start := time.Now()
	defer func() {
		metrics.ObserveSelectorDuration(s.Strategy, time.Since(start).Seconds())
	}()

	service := s.MustService()
	if s.strategy == nil {
		s.mu.Lock()
		if s.strategy == nil {
			s.strategy, err = GetStrategy(s)
			if err != nil {
				logger.Errorf("get strategy %s failed: %v", s.Strategy, err)
				return nil, nil, err
			}
		}
		s.mu.Unlock()
	}

	if time.Since(s.lastUpdate) > 100*time.Millisecond {
		version := service.Version()
		if version > s.lastVersion {
			s.mu.Lock()
			if version > s.lastVersion {
				s.strategy.Update(s.EndpointsWithAttribute(s.strategy.EndpointFilters()...))
				s.lastVersion = version
				s.lastUpdate = time.Now()
				metrics.IncUpdateSelectorCnt(s.Strategy)
			}
			s.mu.Unlock()
		}
	}

	headers = s.baseHeaders()
	endpoint, appendHeaders, err := s.strategy.Select(target)
	if endpoint != nil && err == nil && !s.MustService().IsAvailable(endpoint) {
		err = errcode.ErrEndpointUnavailable
	}
	if appendHeaders != nil {
		for k, v := range appendHeaders {
			headers[k] = v
		}
	}
	return
}

func (s *Selector) baseHeaders() map[string]string {
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
