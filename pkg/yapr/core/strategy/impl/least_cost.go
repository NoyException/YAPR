package builtin

import (
	"noy/router/pkg/yapr/core/errcode"
	"noy/router/pkg/yapr/core/strategy"
	"noy/router/pkg/yapr/core/types"
)

type LeastCostStrategyBuilder struct{}

func (b *LeastCostStrategyBuilder) Build(s *types.Selector) (strategy.Strategy, error) {
	return &LeastCostStrategy{}, nil
}

type LeastCostStrategy struct {
	endpoints map[types.Endpoint]*types.Attribute
}

func (r *LeastCostStrategy) Select(_ *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
	if len(r.endpoints) == 0 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}

	var endpoint *types.Endpoint
	maxWeight := uint32(0)
	for ep, attr := range r.endpoints {
		if endpoint == nil || attr.Weight > maxWeight {
			maxWeight = attr.Weight
			endpoint = &ep
		}
	}
	if endpoint == nil {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}
	return endpoint, nil, nil
}

func (r *LeastCostStrategy) Update(endpoints map[types.Endpoint]*types.Attribute) {
	r.endpoints = endpoints
}
