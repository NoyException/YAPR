package cache

import "noy/router/pkg/yapr/core/types"

// DirectCache 动态键值路由缓存
type DirectCache interface {
	Get(headerValue string) (*types.Endpoint, error)
	Refresh(headerValue string)
	Clear()
}
