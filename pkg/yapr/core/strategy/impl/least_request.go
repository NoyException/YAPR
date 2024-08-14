package builtin

import (
	"math/rand/v2"
	"noy/router/pkg/yapr/core/errcode"
	"noy/router/pkg/yapr/core/strategy"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
)

type LeastRequestStrategyBuilder struct{}

func (b *LeastRequestStrategyBuilder) Build(s *types.Selector) (strategy.Strategy, error) {
	return &LeastRequestStrategy{}, nil
}

type LeastRequestStrategy struct {
	endpoints  []*types.Endpoint
	attributes []*types.Attribute
}

func (r *LeastRequestStrategy) Select(_ *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
	size := len(r.endpoints)
	if size == 0 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}
	if size == 1 {
		return r.endpoints[0], nil, nil
	}

	// 随机选择两个Endpoint，比较其RPS，选择RPS小的
	index1 := rand.IntN(size)
	index2 := rand.IntN(size)
	rps1 := uint32(0)
	rps2 := uint32(0)
	rps1p := r.attributes[index1].RPS
	rps2p := r.attributes[index2].RPS
	if rps1p != nil {
		rps1 = *rps1p
	}
	if rps2p != nil {
		rps2 = *rps2p
	}
	logger.Debugf("rps1: %v, rps2: %v", rps1, rps2)
	if rps1 < rps2 {
		return r.endpoints[index1], nil, nil
	} else {
		return r.endpoints[index2], nil, nil
	}
}

func (r *LeastRequestStrategy) EndpointFilters() []types.EndpointFilter {
	return []types.EndpointFilter{
		types.GoodEndpointFilter,
		types.FuseEndpointFilter,
	}
}

func (r *LeastRequestStrategy) Update(endpoints map[types.Endpoint]*types.Attribute) {
	r.endpoints = make([]*types.Endpoint, 0, len(endpoints))
	r.attributes = make([]*types.Attribute, 0, len(endpoints))
	for endpoint, attr := range endpoints {
		r.endpoints = append(r.endpoints, &endpoint)
		r.attributes = append(r.attributes, attr)
	}
}
