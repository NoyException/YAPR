package buffer

import (
	"noy/router/pkg/yapr/core/store"
	"noy/router/pkg/yapr/core/types"
)

type DefaultBuffer struct {
	selectorName string
}

var _ types.DynamicRouteBuffer = (*DefaultBuffer)(nil)

func NewDefaultBuffer(selectorName string) *DefaultBuffer {
	return &DefaultBuffer{selectorName: selectorName}
}

func (d *DefaultBuffer) Get(headerValue string) (*types.Endpoint, error) {
	return store.MustStore().GetCustomRoute(d.selectorName, headerValue)
}

//func (d *DefaultBuffer) Set(headerValue string, endpoint *types.Endpoint, timeout int64) error {
//	return store.MustStore().SetCustomRoute(d.selectorName, headerValue, endpoint, timeout)
//}
//
//func (d *DefaultBuffer) Remove(headerValue string) error {
//	return store.MustStore().RemoveCustomRoute(d.selectorName, headerValue)
//}

func (d *DefaultBuffer) Clear() {
}
