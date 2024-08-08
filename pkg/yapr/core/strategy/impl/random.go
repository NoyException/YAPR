package builtin

import (
	"math/rand/v2"
	"noy/router/pkg/yapr/core/errcode"
	"noy/router/pkg/yapr/core/strategy"
	"noy/router/pkg/yapr/core/types"
)

type RandomStrategyBuilder struct{}

func (b *RandomStrategyBuilder) Build(s *types.Selector) (strategy.Strategy, error) {
	return &RandomStrategy{}, nil
}

type RandomStrategy struct {
	endpoints []*types.Endpoint
}

func (r *RandomStrategy) Select(_ *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
	if len(r.endpoints) == 0 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}
	return r.endpoints[rand.IntN(len(r.endpoints))], nil, nil
}

func (r *RandomStrategy) Update(endpoints map[types.Endpoint]*types.Attribute) {
	r.endpoints = make([]*types.Endpoint, 0, len(endpoints))
	for endpoint := range endpoints {
		r.endpoints = append(r.endpoints, &endpoint)
	}
}
