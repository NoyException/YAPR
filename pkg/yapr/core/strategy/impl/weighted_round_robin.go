package builtin

import (
	"noy/router/pkg/yapr/core/errcode"
	"noy/router/pkg/yapr/core/strategy"
	"noy/router/pkg/yapr/core/types"
)

type WeightedRoundRobinStrategyBuilder struct{}

func (b *WeightedRoundRobinStrategyBuilder) Build(s *types.Selector) (strategy.Strategy, error) {
	return &WeightedRoundRobinStrategy{nil, 0}, nil
}

type WeightedRoundRobinStrategy struct {
	endpoints map[types.Endpoint]*types.Attribute
	lastIdx   uint32 // 上次选择的endpoint索引
}

// Select 实现上是选取最大权重（weight = C - cost）
func (r *WeightedRoundRobinStrategy) Select(_ *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
	if len(r.endpoints) == 0 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}

	r.lastIdx++
	total := uint32(0)
	var first *types.Endpoint
	for endpoint, attr := range r.endpoints {
		if first == nil {
			first = &endpoint
		}
		total += attr.Weight
		if r.lastIdx < total {
			return &endpoint, nil, nil
		}
	}
	r.lastIdx = 0
	if first == nil {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}
	return first, nil, nil
}

func (r *WeightedRoundRobinStrategy) Update(endpoints map[types.Endpoint]*types.Attribute) {
	r.endpoints = endpoints
}
