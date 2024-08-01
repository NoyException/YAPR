package buffer

import (
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/store"
)

type DefaultBuffer struct {
	selectorName string
}

var _ types.DynamicRouteBuffer = (*DefaultBuffer)(nil)

func NewDefaultBuffer(selectorName string) *DefaultBuffer {
	return &DefaultBuffer{selectorName: selectorName}
}

func (d *DefaultBuffer) Get(headerValue string) *types.Endpoint {
	route, err := store.MustStore().GetCustomRoute(d.selectorName, headerValue)
	if err != nil {
		return nil
	}
	return route
}
