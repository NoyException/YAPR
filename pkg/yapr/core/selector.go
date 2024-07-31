package core

import (
	"fmt"
	"math/rand/v2"
)

func (s *Selector) Select(service *Service) (*Endpoint, error) {
	switch s.Strategy {
	case StrategyRandom:
		return s.randomSelect(service)
	case StrategyRoundRobin:
		return s.roundRobinSelect(service)
	case StrategyWeightedRandom:
		return s.weightedRandomSelect(service)
	case StrategyWeightedRoundRobin:
		return s.weightedRoundRobinSelect(service)
	case StrategyLeastCost:
		return s.leastCostSelect(service)
	default:
		return nil, fmt.Errorf("unknown strategy: %s", s.Strategy)
	}
}

func (s *Selector) randomSelect(service *Service) (*Endpoint, error) {
	endpoints := service.Endpoints()
	if len(endpoints) == 0 {
		return nil, ErrNoEndpointAvailable
	}
	return endpoints[rand.IntN(len(endpoints))], nil
}

func (s *Selector) roundRobinSelect(service *Service) (*Endpoint, error) {
	endpoints := service.Endpoints()
	if len(endpoints) == 0 {
		return nil, ErrNoEndpointAvailable
	}
	s.lastIdx = (s.lastIdx + 1) % uint32(len(endpoints))
	return endpoints[s.lastIdx], nil
}

func (s *Selector) weightedRandomSelect(service *Service) (*Endpoint, error) {
	endpoints := service.Endpoints()
	if len(endpoints) == 0 {
		return nil, ErrNoEndpointAvailable
	}
	attributes := service.Attributes(s.Name)

	totalWeight := uint32(0)
	for _, attr := range attributes {
		totalWeight += attr.Weight
	}
	r := rand.Uint32() % totalWeight
	totalWeight = 0
	for i, attr := range attributes {
		totalWeight += attr.Weight
		if r < totalWeight {
			return endpoints[i], nil
		}
	}
	return nil, ErrNoEndpointAvailable
}

func (s *Selector) weightedRoundRobinSelect(service *Service) (*Endpoint, error) {
	endpoints := service.Endpoints()
	if len(endpoints) == 0 {
		return nil, ErrNoEndpointAvailable
	}
	attributes := service.Attributes(s.Name)

	s.lastIdx++
	total := uint32(0)
	for i, attr := range attributes {
		total += attr.Weight
		if s.lastIdx < total {
			return endpoints[i], nil
		}
	}
	s.lastIdx = 0
	if total == 0 {
		return nil, ErrNoEndpointAvailable
	}
	return endpoints[0], nil
}

// 实现上是选取最大权重（weight = C - cost）
func (s *Selector) leastCostSelect(service *Service) (*Endpoint, error) {
	endpoints := service.Endpoints()
	if len(endpoints) == 0 {
		return nil, ErrNoEndpointAvailable
	}
	attributes := service.Attributes(s.Name)

	idx := -1
	maxWeight := uint32(0)
	for i, attr := range attributes {
		if idx == -1 || attr.Weight > maxWeight {
			maxWeight = attr.Weight
			idx = i
		}
	}
	if idx == -1 {
		return nil, ErrNoEndpointAvailable
	}
	return endpoints[idx], nil
}
