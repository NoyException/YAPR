package builtin

import (
	"math/rand/v2"
	"noy/router/pkg/yapr/core/errcode"
	"noy/router/pkg/yapr/core/strategy"
	"noy/router/pkg/yapr/core/types"
)

type WeightedRandomStrategyBuilder struct{}

func (b *WeightedRandomStrategyBuilder) Build(s *types.Selector) (strategy.Strategy, error) {
	return &WeightedRandomStrategy{}, nil
}

type WeightedRandomStrategy struct {
	endpoints map[types.Endpoint]*types.Attribute
}

func (r *WeightedRandomStrategy) Select(_ *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
	if len(r.endpoints) == 0 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}

	totalWeight := uint32(0)
	for _, attr := range r.endpoints {
		weight := uint32(1)
		if attr.Weight != nil {
			weight = *attr.Weight
		}
		totalWeight += weight
	}
	rnd := rand.Uint32() % totalWeight
	totalWeight = 0
	for endpoint, attr := range r.endpoints {
		weight := uint32(1)
		if attr.Weight != nil {
			weight = *attr.Weight
		}
		totalWeight += weight
		if rnd < totalWeight {
			return &endpoint, nil, nil
		}
	}
	return nil, nil, errcode.ErrNoEndpointAvailable
}

func (r *WeightedRandomStrategy) EndpointFilters() []types.EndpointFilter {
	return []types.EndpointFilter{
		types.GoodEndpointFilter,
		types.FuseEndpointFilter,
	}
}

func (r *WeightedRandomStrategy) Update(endpoints map[types.Endpoint]*types.Attribute) {
	r.endpoints = endpoints
}
