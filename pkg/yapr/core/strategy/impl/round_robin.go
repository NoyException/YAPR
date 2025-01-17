package builtin

import (
	"noy/router/pkg/yapr/core/errcode"
	"noy/router/pkg/yapr/core/strategy"
	"noy/router/pkg/yapr/core/types"
	"sync/atomic"
)

type RoundRobinStrategyBuilder struct{}

func (b *RoundRobinStrategyBuilder) Build(s *types.Selector) (strategy.Strategy, error) {
	return &RoundRobinStrategy{nil, 0}, nil
}

type RoundRobinStrategy struct {
	endpoints []types.Endpoint
	lastIdx   uint32 // 上次选择的endpoint索引
}

func (r *RoundRobinStrategy) Select(_ *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
	size := uint32(len(r.endpoints))
	if size == 0 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}
	atomic.AddUint32(&r.lastIdx, 1)
	return &r.endpoints[r.lastIdx%size], nil, nil
}

func (r *RoundRobinStrategy) EndpointFilters() []types.EndpointFilter {
	return []types.EndpointFilter{
		types.GoodEndpointFilter,
		types.FuseEndpointFilter,
	}
}

func (r *RoundRobinStrategy) Update(endpoints map[types.Endpoint]*types.Attribute) {
	size := len(endpoints)
	r.endpoints = make([]types.Endpoint, 0, size)
	for endpoint := range endpoints {
		r.endpoints = append(r.endpoints, endpoint)
	}
}
