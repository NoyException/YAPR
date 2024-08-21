package yaprsdk

import (
	"noy/router/pkg/yapr/core/store"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
	"sync"
)

type RoutingTable struct {
	table map[string]map[string]*types.Endpoint
	mu    sync.RWMutex
}

func NewRoutingTable() *RoutingTable {
	return &RoutingTable{
		table: make(map[string]map[string]*types.Endpoint),
		mu:    sync.RWMutex{},
	}
}

func (rt *RoutingTable) AddRoute(selectorName, headerValue string, endpoint *types.Endpoint) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if _, ok := rt.table[selectorName]; !ok {
		rt.table[selectorName] = make(map[string]*types.Endpoint)
	}
	rt.table[selectorName][headerValue] = endpoint
}

func (rt *RoutingTable) GetRoute(selectorName, headerValue string) *types.Endpoint {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	if _, ok := rt.table[selectorName]; !ok {
		return nil
	}
	return rt.table[selectorName][headerValue]
}

func (rt *RoutingTable) DeleteRoute(selectorName, headerValue string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if _, ok := rt.table[selectorName]; !ok {
		return
	}
	delete(rt.table[selectorName], headerValue)
}

func (rt *RoutingTable) Refresh(selectorName, headerValue string) *types.Endpoint {
	expectEndpoint, err := store.MustStore().GetCustomRoute(selectorName, headerValue)
	if err != nil {
		logger.Errorf("get custom route failed: %v", err)
		return nil
	}
	rt.AddRoute(selectorName, headerValue, expectEndpoint)
	return expectEndpoint
}
