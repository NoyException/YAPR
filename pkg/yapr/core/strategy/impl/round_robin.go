package builtin

import (
	"noy/router/pkg/yapr/core/errcode"
	"noy/router/pkg/yapr/core/strategy"
	"noy/router/pkg/yapr/core/types"
)

type RoundRobinStrategyBuilder struct{}

func (b *RoundRobinStrategyBuilder) Build(s *types.Selector) (strategy.Strategy, error) {
	return &RoundRobinStrategy{nil, 0}, nil
}

type RoundRobinStrategy struct {
	endpoints []*types.Endpoint
	lastIdx   uint32 // 上次选择的endpoint索引
}

func (r *RoundRobinStrategy) Select(_ *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
	if len(r.endpoints) == 0 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}
	r.lastIdx = (r.lastIdx + 1) % uint32(len(r.endpoints))
	return r.endpoints[r.lastIdx], nil, nil
}

func (r *RoundRobinStrategy) Update(endpoints map[types.Endpoint]*types.Attribute) {
	r.endpoints = make([]*types.Endpoint, 0, len(endpoints))
	for endpoint := range endpoints {
		r.endpoints = append(r.endpoints, &endpoint)
	}
}
