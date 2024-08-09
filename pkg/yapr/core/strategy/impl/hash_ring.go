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

func (r *HashRingStrategy) Select(match *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
	if r.ring.Size() == 0 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}

	value, err := strategy.HeaderValue(r.headerKey, match)
	if err != nil {
		return nil, nil, err
	}
	hashed := int(strategy.HashString(value))
	//logger.Debugf("hash ring: %d", hashed)
	k, v := r.ring.Ceiling(hashed)
	if k == nil {
		k, v = r.ring.Min()
		if k == nil {
			return nil, nil, errcode.ErrNoEndpointAvailable
		}
	}
	return v.(*types.Endpoint), nil, nil
}

func (r *HashRingStrategy) Update(endpoints map[types.Endpoint]*types.Attribute) {
	r.ring = treemap.NewWithIntComparator()
	for endpoint, attr := range endpoints {
		for i := 0; i < int(attr.Weight)*8; i++ {
			hashCode := int(strategy.HashString(endpoint.String() + strconv.Itoa(i)))
			//logger.Debugf("hash ring: %d -> %s", hashCode, endpoint.String())
			r.ring.Put(hashCode, &endpoint)
		}
	}
}
