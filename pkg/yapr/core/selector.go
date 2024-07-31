package core

import (
	"fmt"
	"math/rand/v2"
)

func (s *Selector) Select(service *Service) (*Endpoint, error) {
	switch s.Strategy {
	case StrategyRandom:
		return s.randomSelect(service)
	default:
		return nil, fmt.Errorf("unknown strategy: %s", s.Strategy)
	}
}

func (s *Selector) randomSelect(service *Service) (*Endpoint, error) {
	endpoints := make([]*Endpoint, 0)
	for endpoint, _ := range service.AttrMap {
		endpoints = append(endpoints, &endpoint)
	}
	return endpoints[rand.IntN(len(endpoints))], nil
}
