package builtin

import (
	"math/rand"
	"noy/router/pkg/yapr/core/errcode"
	"noy/router/pkg/yapr/core/strategy"
	"noy/router/pkg/yapr/core/types"
)

type RandomStrategyBuilder struct{}

func (b *RandomStrategyBuilder) Build(s *types.Selector) (strategy.Strategy, error) {
	return &RandomStrategy{}, nil
}

type RandomStrategy struct {
	endpoints []types.Endpoint
}

func (r *RandomStrategy) Select(_ *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
	size := len(r.endpoints)
	if size == 0 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}
	return &r.endpoints[rand.Intn(size)], nil, nil
}

func (r *RandomStrategy) EndpointFilters() []types.EndpointFilter {
	return []types.EndpointFilter{
		types.GoodEndpointFilter,
		types.FuseEndpointFilter,
	}
}

func (r *RandomStrategy) Update(endpoints map[types.Endpoint]*types.Attribute) {
	r.endpoints = make([]types.Endpoint, 0, len(endpoints))
	for endpoint := range endpoints {
		r.endpoints = append(r.endpoints, endpoint)
	}
}
