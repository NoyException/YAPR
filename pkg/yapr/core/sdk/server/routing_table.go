package yaprsdk

import "noy/router/pkg/yapr/core/types"

type RoutingTable map[string]map[string]*types.Endpoint

func NewRoutingTable() RoutingTable {
	return make(RoutingTable)
}

func (rt RoutingTable) AddRoute(selectorName, headerValue string, endpoint *types.Endpoint) {
	if _, ok := rt[selectorName]; !ok {
		rt[selectorName] = make(map[string]*types.Endpoint)
	}
	rt[selectorName][headerValue] = endpoint
}

func (rt RoutingTable) GetRoute(selectorName, headerValue string) *types.Endpoint {
	if _, ok := rt[selectorName]; !ok {
		return nil
	}
	return rt[selectorName][headerValue]
}

func (rt RoutingTable) DeleteRoute(selectorName, headerValue string) {
	if _, ok := rt[selectorName]; !ok {
		return
	}
	delete(rt[selectorName], headerValue)
}
