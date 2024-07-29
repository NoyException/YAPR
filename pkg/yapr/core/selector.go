package core

import (
	"fmt"
)

func (s *Selector) Select(target *MatchTarget) (*Endpoint, error) {
	switch s.Strategy {
	case StrategyRandom:
		return s.randomSelect(target)
	default:
		return nil, fmt.Errorf("unknown strategy: %s", s.Strategy)
	}
}

func (s *Selector) randomSelect(target *MatchTarget) (*Endpoint, error) {
	//return &s.Endpoints[rand.IntN(len(s.Endpoints))], nil
	return nil, nil
}
