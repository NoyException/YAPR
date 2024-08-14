package builtin

import (
	"math/rand/v2"
	"noy/router/pkg/yapr/core/errcode"
	"noy/router/pkg/yapr/core/strategy"
	"noy/router/pkg/yapr/core/types"
)

type WeightedRoundRobinStrategyBuilder struct{}

func (b *WeightedRoundRobinStrategyBuilder) Build(s *types.Selector) (strategy.Strategy, error) {
	return &WeightedRoundRobinStrategy{
		lastIdx: 0,
	}, nil
}

type WeightedRoundRobinStrategy struct {
	endpoints  []*types.Endpoint
	attributes []*types.Attribute
	lastIdx    uint32 // 上次选择的endpoint索引
}

// Select 实现上是选取最大权重（weight = C - cost）
func (r *WeightedRoundRobinStrategy) Select(_ *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
	if len(r.endpoints) == 0 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}

	r.lastIdx++
	total := uint32(0)
	var first *types.Endpoint
	for i, endpoint := range r.endpoints {
		if first == nil {
			first = endpoint
		}
		weight := uint32(1)
		w := r.attributes[i].Weight
		if w != nil {
			weight = *w
		}
		total += weight
		if r.lastIdx < total {
			return endpoint, nil, nil
		}
	}
	r.lastIdx = 0
	if first == nil {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}
	return first, nil, nil
}

func (r *WeightedRoundRobinStrategy) EndpointFilters() []types.EndpointFilter {
	return []types.EndpointFilter{
		types.GoodEndpointFilter,
		types.FuseEndpointFilter,
	}
}

func (r *WeightedRoundRobinStrategy) Update(endpoints map[types.Endpoint]*types.Attribute) {
	size := len(endpoints)
	r.endpoints = make([]*types.Endpoint, 0, size)
	r.attributes = make([]*types.Attribute, 0, size)
	for endpoint, attr := range endpoints {
		r.endpoints = append(r.endpoints, &endpoint)
		r.attributes = append(r.attributes, attr)
	}
	if len(r.endpoints) > 0 {
		r.lastIdx = uint32(rand.IntN(len(r.endpoints)))
	}
}
