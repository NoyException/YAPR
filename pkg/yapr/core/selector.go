package core

import (
	"fmt"
	"google.golang.org/grpc/metadata"
	"hash/fnv"
	"math/rand/v2"
	"noy/router/pkg/yapr/logger"
)

func (s *Selector) Select(service *Service, target *MatchTarget) (*Endpoint, error) {
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
	case StrategyConsistentHash:
		return s.consistentHashSelect(service, target)
	case StrategyDirect:
		return s.directSelect(service, target)
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

func JumpConsistentHash(key uint64, numBuckets int32) int32 {
	if numBuckets <= 0 {
		panic("numBuckets must be greater than 0")
	}
	var b, j int64 = -1, 0
	for j < int64(numBuckets) {
		b = j
		key = key*2862933555777941757 + 1
		j = int64(float64(b+1) * (float64(1<<31) / float64((key>>33)+1)))
	}
	return int32(b)
}

func hashString(s string) uint64 {
	h := fnv.New64a()
	_, err := h.Write([]byte(s))
	if err != nil {
		logger.Errorf("hash string failed: %v", err)
		return 0
	}
	return h.Sum64()
}

func (s *Selector) headerValue(target *MatchTarget) (string, error) {
	if s.Key == "" {
		return "", ErrNoKeyAvailable
	}

	md, exist := metadata.FromOutgoingContext(target.Ctx)
	if !exist {
		return "", ErrNoValueAvailable
	}
	values := md.Get(s.Key)
	if len(values) == 0 {
		return "", ErrNoValueAvailable
	}
	return values[0], nil
}

func (s *Selector) consistentHashSelect(service *Service, target *MatchTarget) (*Endpoint, error) {
	if s.Key == "" {
		return nil, ErrNoKeyAvailable
	}

	endpoints := service.Endpoints()
	if len(endpoints) == 0 {
		return nil, ErrNoEndpointAvailable
	}

	value, err := s.headerValue(target)
	if err != nil {
		return nil, err
	}
	hashed := hashString(value)
	idx := JumpConsistentHash(hashed, int32(len(endpoints)))
	//logger.Debugf("select idx %d by value %v with hashed %v", idx, value, hashed)
	return endpoints[idx], nil
}

func (s *Selector) directSelect(service *Service, target *MatchTarget) (*Endpoint, error) {
	if s.Key == "" {
		return nil, ErrNoKeyAvailable
	}

	endpoints := service.Endpoints()
	if len(endpoints) == 0 {
		return nil, ErrNoEndpointAvailable
	}

	value, err := s.headerValue(target)
	if err != nil {
		return nil, err
	}

	endpoint, err := MustStore().GetCustomRoute(s.Name, value)
	if err != nil || endpoint == nil {
		return nil, err
	}
	logger.Debugf("direct select endpoint %v by value %v", endpoint, value)
	return endpoint, nil
}
