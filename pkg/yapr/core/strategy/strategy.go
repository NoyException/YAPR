package strategy

import "noy/router/pkg/yapr/core/types"

type Strategy interface {
	// Select 选择一个endpoint，其中headers应当基于selector.BaseHeaders()构建。该方法会被多个goroutine并发调用
	Select(match *types.MatchTarget) (endpoint *types.Endpoint, headers map[string]string, err error)
	// EndpointFilters 返回策略的endpoint过滤器，用于在Update时传入过滤后的endpoints
	EndpointFilters() []types.EndpointFilter
	// Update 更新endpoints，该方法在调用时不会有其他goroutine调用Select
	Update(endpoints map[types.Endpoint]*types.Attribute)
}

type Builder interface {
	Build(s *types.Selector) (Strategy, error)
}

type StatefulStrategy interface {
	NotifyRetry(headerValue string)
}
