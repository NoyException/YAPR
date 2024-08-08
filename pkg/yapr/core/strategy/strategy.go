package strategy

import "noy/router/pkg/yapr/core/types"

type Strategy interface {
	// Select 选择一个endpoint，其中headers应当基于selector.BaseHeaders()构建
	Select(match *types.MatchTarget) (endpoint *types.Endpoint, headers map[string]string, err error)
	Update(endpoints map[types.Endpoint]*types.Attribute)
}

type StrategyBuilder interface {
	Build(s *types.Selector) (Strategy, error)
}

type StatefulStrategy interface {
	NotifyRetry(headerValue string)
}
