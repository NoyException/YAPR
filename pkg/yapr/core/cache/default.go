package cache

import (
	"noy/router/pkg/yapr/core/store"
	"noy/router/pkg/yapr/core/types"
)

type DefaultCache struct {
	selectorName string
}

var _ types.DirectCache = (*DefaultCache)(nil)

func NewDefaultBuffer(selectorName string) *DefaultCache {
	return &DefaultCache{selectorName: selectorName}
}

func (d *DefaultCache) Get(headerValue string) (*types.Endpoint, error) {
	return store.MustStore().GetCustomRoute(d.selectorName, headerValue)
}

func (d *DefaultCache) Refresh(headerValue string) {
}

//func (d *DefaultCache) Set(headerValue string, endpoint *types.Endpoint, timeout int64) error {
//	return store.MustStore().SetCustomRoute(d.selectorName, headerValue, endpoint, timeout)
//}
//
//func (d *DefaultCache) Remove(headerValue string) error {
//	return store.MustStore().RemoveCustomRoute(d.selectorName, headerValue)
//}

func (d *DefaultCache) Clear() {
}
