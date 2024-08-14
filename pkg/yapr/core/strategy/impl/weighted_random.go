package builtin

import (
	"github.com/emirpasic/gods/maps/treemap"
	"math/rand"
	"noy/router/pkg/yapr/core/errcode"
	"noy/router/pkg/yapr/core/strategy"
	"noy/router/pkg/yapr/core/types"
)

type WeightedRandomStrategyBuilder struct{}

func (b *WeightedRandomStrategyBuilder) Build(s *types.Selector) (strategy.Strategy, error) {
	return &WeightedRandomStrategy{}, nil
}

type WeightedRandomStrategy struct {
	totalWeight           int
	weightStageToEndpoint *treemap.Map
}

func (r *WeightedRandomStrategy) Select(_ *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
	rnd := rand.Int() % r.totalWeight
	_, rawEndpoint := r.weightStageToEndpoint.Floor(rnd)
	if rawEndpoint == nil {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}
	endpoint := rawEndpoint.(*types.Endpoint)
	return endpoint, nil, nil
}

func (r *WeightedRandomStrategy) EndpointFilters() []types.EndpointFilter {
	return []types.EndpointFilter{
		types.GoodEndpointFilter,
		types.FuseEndpointFilter,
	}
}

func (r *WeightedRandomStrategy) Update(endpoints map[types.Endpoint]*types.Attribute) {
	r.weightStageToEndpoint = treemap.NewWithIntComparator()
	r.totalWeight = 0
	for endpoint, attr := range endpoints {
		weight := 1
		if attr.Weight != nil {
			weight = int(*attr.Weight)
		}
		r.weightStageToEndpoint.Put(r.totalWeight, endpoint)
		r.totalWeight += weight
	}
}
