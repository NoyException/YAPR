package builtin

import (
	"github.com/emirpasic/gods/maps/treemap"
	"noy/router/pkg/yapr/core/errcode"
	"noy/router/pkg/yapr/core/strategy"
	"noy/router/pkg/yapr/core/types"
	"sync/atomic"
)

type WeightedRoundRobinStrategyBuilder struct{}

func (b *WeightedRoundRobinStrategyBuilder) Build(s *types.Selector) (strategy.Strategy, error) {
	return &WeightedRoundRobinStrategy{
		lastIdx: 0,
	}, nil
}

type WeightedRoundRobinStrategy struct {
	weightStageToEndpoint *treemap.Map
	totalWeight           uint32
	lastIdx               uint32 // 上次选择的endpoint索引
}

// Select 实现上是选取最大权重（weight = C - cost）
func (r *WeightedRoundRobinStrategy) Select(_ *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
	if r.totalWeight == 0 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}

	atomic.AddUint32(&r.lastIdx, 1)
	_, rawEndpoint := r.weightStageToEndpoint.Floor(int(r.lastIdx % r.totalWeight))
	endpoint := rawEndpoint.(types.Endpoint)
	return &endpoint, nil, nil
}

func (r *WeightedRoundRobinStrategy) EndpointFilters() []types.EndpointFilter {
	return []types.EndpointFilter{
		types.GoodEndpointFilter,
		types.FuseEndpointFilter,
	}
}

func (r *WeightedRoundRobinStrategy) Update(endpoints map[types.Endpoint]*types.Attribute) {
	r.weightStageToEndpoint = treemap.NewWithIntComparator()
	r.totalWeight = 0
	for endpoint, attr := range endpoints {
		weight := uint32(1)
		if attr.Weight != nil {
			weight = *attr.Weight
		}
		r.weightStageToEndpoint.Put(int(r.totalWeight), endpoint)
		r.totalWeight += weight
	}
}
