package builtin

import (
	"github.com/emirpasic/gods/maps/treemap"
	"noy/router/pkg/yapr/core/errcode"
	"noy/router/pkg/yapr/core/strategy"
	"noy/router/pkg/yapr/core/types"
	"strconv"
)

type HashRingStrategyBuilder struct{}

func (b *HashRingStrategyBuilder) Build(s *types.Selector) (strategy.Strategy, error) {
	return &HashRingStrategy{headerKey: s.Key}, nil
}

type HashRingStrategy struct {
	headerKey string
	ring      *treemap.Map
}

func (r *HashRingStrategy) Select(_ *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
	if r.ring.Size() == 0 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}

	value, err := strategy.HeaderValue(r.headerKey, nil)
	if err != nil {
		return nil, nil, err
	}
	hashed := strategy.HashString(value)
	k, v := r.ring.Floor(hashed)
	if k == nil {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}
	return v.(*types.Endpoint), nil, nil
}

func (r *HashRingStrategy) Update(endpoints map[types.Endpoint]*types.Attribute) {
	r.ring = treemap.NewWithIntComparator()
	for endpoint, attr := range endpoints {
		for i := 0; i < int(attr.Weight); i++ {
			r.ring.Put(int(strategy.HashString(endpoint.String()+strconv.Itoa(i))), &endpoint)
		}
	}
}
