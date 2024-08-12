package builtin

import (
	"math/rand/v2"
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
	index1 := rand.IntN(size)
	index2 := rand.IntN(size)
	logger.Debugf("weight1: %v, weight2: %v", r.attributes[index1].Weight, r.attributes[index2].Weight)
	if r.attributes[index1].Weight < r.attributes[index2].Weight {
		return r.endpoints[index1], nil, nil
	} else {
		return r.endpoints[index2], nil, nil
	}
}

func (r *LeastCostStrategy) Update(endpoints map[types.Endpoint]*types.Attribute) {
	r.endpoints = make([]*types.Endpoint, 0, len(endpoints))
	r.attributes = make([]*types.Attribute, 0, len(endpoints))
	for endpoint, attr := range endpoints {
		if !attr.IsGood() {
			continue
		}
		r.endpoints = append(r.endpoints, &endpoint)
		r.attributes = append(r.attributes, attr)
	}
}
