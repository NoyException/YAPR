package core

import (
	"errors"
	"noy/router/pkg/yapr/core/errcode"
	"noy/router/pkg/yapr/core/store"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Router 代表了一个服务网格的所有路由规则
type Router struct {
	*types.Router
}

var routers = make(map[string]*Router)
var routerMu = &sync.Mutex{}

func GetRouter(name string) (*Router, error) {
	routerMu.Lock()
	defer routerMu.Unlock()

	if router, ok := routers[name]; ok {
		return router, nil
	}
	raw, err := store.MustStore().GetRouter(name)
	if err != nil {
		logger.Errorf("router %s not found", name)
		return nil, err
	}
	router := &Router{Router: raw}
	routers[name] = router
	return router, nil
}

func Match(m *types.Matcher, target *types.MatchTarget) bool {
	uri := m.RegexURI()
	matched := uri.Match([]byte(target.URI))
	if !matched {
		return false
	}
	if m.Port != 0 && target.Port != m.Port {
		return false
	}
	if m.Headers != nil {
		for k, regex := range m.RegexHeaders() {
			headerValue, ok := target.Headers[k]
			if !ok {
				return false
			}
			if !regex.MatchString(headerValue) {
				return false
			}
		}
	}
	return true
}

func MatchRule(r *types.Rule, target *types.MatchTarget) bool {
	for _, matcher := range r.Matchers {
		if Match(matcher, target) {
			return true
		}
	}
	return false
}

func parseTarget(target string) (string, uint32, error) {
	splits := strings.Split(target, ":")
	if len(splits) > 2 {
		return "", 0, errors.New("error format routerName")
	}
	if len(splits) == 1 {
		return target, 9090, nil
	}
	port, err := strconv.ParseUint(splits[1], 10, 32)
	if err != nil {
		return "", 0, err
	}
	return splits[0], uint32(port), nil
}

func (r *Router) Route(target *types.MatchTarget) (string, *types.Endpoint, uint32, map[string]string, error) {
	for _, rule := range r.Rules {
		if !MatchRule(rule, target) {
			continue
		}

		selector, err := GetSelector(rule.Selector)
		if err != nil {
			logger.Warnf("selector %s not found", rule.Selector)
		}

		endpoint, headers, selectErr := selector.Select(target)

		if selectErr != nil {
			var handler *types.ErrorHandler
			if errors.Is(selectErr, errcode.ErrNoEndpointAvailable) || errors.Is(selectErr, errcode.ErrNoCustomRoute) {
				if h, ok := rule.Catch[types.RuleErrorNoEndpoint]; ok {
					handler = h
				}
			} else if errors.Is(selectErr, errcode.ErrEndpointUnavailable) {
				if h, ok := rule.Catch[types.RuleErrorEndpointUnavailable]; ok {
					handler = h
				}
			} else {
				if h, ok := rule.Catch[types.RuleErrorDefault]; ok {
					handler = h
				}
			}

			if handler == nil {
				logger.Errorf("selector %s select failed: %v", rule.Selector, selectErr)
				return "", nil, 0, nil, selectErr
			}

			switch handler.Solution {
			case types.SolutionPass:
				logger.Infof("selector %s select failed: %v, move to the next rule", rule.Selector, selectErr)
				continue
			case types.SolutionThrow:
				if handler.Code != nil || handler.Message != nil {
					code := 0
					if handler.Code != nil {
						code = int(*handler.Code)
					}
					message := ""
					if handler.Message != nil {
						message = *handler.Message
					}
					return "", nil, 0, nil, errcode.New(code, message)
				}
				return "", nil, 0, nil, selectErr
			case types.SolutionPanic:
				if handler.Code != nil || handler.Message != nil {
					code := 0
					if handler.Code != nil {
						code = int(*handler.Code)
					}
					message := ""
					if handler.Message != nil {
						message = *handler.Message
					}
					panic(errcode.New(code, message))
				}
				panic(selectErr)
			case types.SolutionRetry:
				times := 3
				if handler.Times != nil {
					times = int(*handler.Times)
				} else {
					logger.Warnf("retry times not set, use default 3")
				}
				interval := float32(1)
				if handler.Interval != nil {
					interval = *handler.Interval
				} else {
					logger.Warnf("retry interval not set, use default 1")
				}
				for i := 0; i < times; i++ {
					after := time.After(time.Duration(interval*1000) * time.Millisecond)
					<-after
					endpoint, headers, selectErr = selector.Select(target)
					if selectErr == nil {
						break
					}
					logger.Warnf("retry %d times, selector %s select failed: %v", i+1, rule.Selector, selectErr)
				}
			case types.SolutionBlock:
				logger.Errorf("selector %s select failed: %v, block the request", rule.Selector, selectErr)
				service, err := GetService(selector.Service)
				if err != nil {
					logger.Warnf("service %s not found", selector.Service)
					continue
				}
				timeout := float32(5)
				if handler.Timeout != nil {
					timeout = *handler.Timeout
				}
				after := time.After(time.Duration(timeout*1000) * time.Millisecond)

				if target.Timeout == nil {
					target.Timeout = make(chan struct{})
				}

				flag := true
				for flag {
					select {
					case <-target.Timeout:
						return "", nil, 0, nil, errcode.ErrContextCanceled
					case <-after:
						return "", nil, 0, nil, errcode.ErrBlockTimeout
					case <-service.UpdateNTF:
						if endpoint == nil {
							break
						}
						if _, ok := service.EndpointsSet()[*endpoint]; ok {
							flag = false
							break
						}
					}
				}
			}
		}

		if headers != nil {
			headers["yapr-router"] = r.Name
			headers["yapr-endpoint"] = endpoint.String()
		}
		port := selector.Port
		if endpoint.Port != 0 {
			port = endpoint.Port
		}
		logger.Debugf("select %v by strategy %s", endpoint, selector.Strategy)
		return selector.Service, endpoint, port, headers, nil
	}
	return "", nil, 0, nil, errcode.ErrNoRuleMatched
}

func (r *Router) Selectors() []*Selector {
	selectorNames := make(map[string]bool)
	for _, rule := range r.Rules {
		selectorNames[rule.Selector] = true
	}
	selectors := make([]*Selector, 0, len(selectorNames))
	for name, _ := range selectorNames {
		selector, err := GetSelector(name)
		if err != nil {
			logger.Warnf("selector %s not found", name)
			continue
		}
		selectors = append(selectors, selector)
	}
	return selectors
}
