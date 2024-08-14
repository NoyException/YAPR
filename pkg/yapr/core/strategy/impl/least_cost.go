package builtin

import (
	"math/rand"
	"noy/router/pkg/yapr/core/errcode"
	"noy/router/pkg/yapr/core/strategy"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
)

type LeastCostStrategyBuilder struct{}

func (b *LeastCostStrategyBuilder) Build(s *types.Selector) (strategy.Strategy, error) {
	return &LeastCostStrategy{}, nil
}

type LeastCostStrategy struct {
	endpoints  []*types.Endpoint
	attributes []*types.Attribute
}

func (r *LeastCostStrategy) Select(_ *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
	size := len(r.endpoints)
	if size == 0 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}
	if size == 1 {
		return r.endpoints[0], nil, nil
	}

	// 随机选择两个Endpoint，比较其权重，选择权重小的
	index1 := rand.Intn(size)
	index2 := rand.Intn(size)
	weight1 := uint32(1)
	weight2 := uint32(1)
	w1 := r.attributes[index1].Weight
	w2 := r.attributes[index2].Weight
	if w1 != nil {
		weight1 = *w1
	}
	if w2 != nil {
		weight2 = *w2
	}
	logger.Debugf("weight1: %v, weight2: %v", weight1, weight2)
	if weight1 < weight2 {
		return r.endpoints[index1], nil, nil
	} else {
		return r.endpoints[index2], nil, nil
	}
}

func (r *LeastCostStrategy) EndpointFilters() []types.EndpointFilter {
	return []types.EndpointFilter{
		types.GoodEndpointFilter,
		types.FuseEndpointFilter,
	}
}

func (r *LeastCostStrategy) Update(endpoints map[types.Endpoint]*types.Attribute) {
	r.endpoints = make([]*types.Endpoint, 0, len(endpoints))
	r.attributes = make([]*types.Attribute, 0, len(endpoints))
	for endpoint, attr := range endpoints {
		r.endpoints = append(r.endpoints, &endpoint)
		r.attributes = append(r.attributes, attr)
	}
}
