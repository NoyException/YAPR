package core

import "fmt"

var (
	ErrRouteAlreadyExists  = fmt.Errorf("route already exists")
	ErrRouterNotFound      = fmt.Errorf("router not found")
	ErrSelectorNotFound    = fmt.Errorf("selector not found")
	ErrServiceNotFound     = fmt.Errorf("service not found")
	ErrNoEndpointAvailable = fmt.Errorf("no endpoint available")
	ErrNoRuleMatched       = fmt.Errorf("no rule matched")
	ErrNoCustomRoute       = fmt.Errorf("no custom route")
	ErrNoKeyAvailable      = fmt.Errorf("no key available (for stateful routing)")
	ErrNoValueAvailable    = fmt.Errorf("no value available (for stateful routing)")
	ErrBufferNotFound      = fmt.Errorf("buffer not found")
)
