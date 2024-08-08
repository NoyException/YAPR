package builtin

import (
	"noy/router/pkg/yapr/core/errcode"
	"noy/router/pkg/yapr/core/strategy"
	"noy/router/pkg/yapr/core/strategy/impl/cache"
	"noy/router/pkg/yapr/core/strategy/impl/cache/impl"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
)

type DirectStrategyBuilder struct{}

func (b *DirectStrategyBuilder) Build(s *types.Selector) (strategy.Strategy, error) {
	size := s.CacheSize
	if size == 0 {
		size = 4096
	}
	var c cache.DirectCache
	switch s.CacheType {
	case types.BufferTypeLRU:
		c = cache_impl.NewLRUBuffer(s.Name, size)
	case types.BufferTypeNone:
		fallthrough
	default:
		c = cache_impl.NewDefaultBuffer(s.Name)
	}
	return &DirectStrategy{
		headerKey: s.Key,
		cache:     c,
		endpoints: nil,
	}, nil
}

type DirectStrategy struct {
	headerKey string
	cache     cache.DirectCache
	endpoints map[types.Endpoint]*types.Attribute
}

func (r *DirectStrategy) Select(match *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
	if r.headerKey == "" {
		return nil, nil, errcode.ErrNoKeyAvailable
	}

	if len(r.endpoints) == 0 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}

	value, err := strategy.HeaderValue(r.headerKey, match)
	if err != nil {
		return nil, nil, err
	}

	if r.cache == nil {
		return nil, nil, errcode.ErrCacheNotFound
	}

	endpoint, err := r.cache.Get(value)
	if err != nil {
		return nil, nil, err
	}
	if endpoint == nil {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}
	// endpoint有可能已经被删除，需要检查
	headers := make(map[string]string)
	headers["yapr-header-value"] = value
	if _, ok := r.endpoints[*endpoint]; !ok {
		return endpoint, headers, errcode.ErrEndpointUnavailable
	}
	logger.Debugf("direct select endpoint %v by value %v", endpoint, value)
	return endpoint, headers, nil
}

func (r *DirectStrategy) Update(endpoints map[types.Endpoint]*types.Attribute) {
	r.endpoints = endpoints
}

func (r *DirectStrategy) NotifyRetry(headerValue string) {
	if r.cache != nil {
		r.cache.Refresh(headerValue)
	}
}
