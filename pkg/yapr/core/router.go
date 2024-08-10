package core

import (
	"errors"
	"google.golang.org/grpc/metadata"
	"noy/router/pkg/yapr/core/errcode"
	"noy/router/pkg/yapr/core/store"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
	"regexp"
	"strconv"
	"strings"
)

// Router 代表了一个服务网格的所有路由规则
type Router struct {
	*types.Router
}

var routers = make(map[string]*Router)

func GetRouter(name string) (*Router, error) {
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
	matched, err := regexp.Match(m.URI, []byte(target.URI))
	if err != nil || !matched {
		return false
	}
	if m.Port != 0 && target.Port != m.Port {
		return false
	}
	if m.Headers != nil {
		md, ok := metadata.FromIncomingContext(target.Ctx)
		if !ok {
			return false
		}
		for k, v := range m.Headers {
			regex := regexp.MustCompile(v)
			if !regex.MatchString(md[k][0]) {
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
				if h, ok := rule.ErrorHandler[types.RuleErrorNoEndpoint]; ok {
					handler = h
				}
			} else if errors.Is(selectErr, errcode.ErrEndpointUnavailable) {
				if h, ok := rule.ErrorHandler[types.RuleErrorEndpointUnavailable]; ok {
					handler = h
				}
			} else {
				if h, ok := rule.ErrorHandler[types.RuleErrorDefault]; ok {
					handler = h
				}
			}

			if handler == nil {
				logger.Errorf("selector %s select failed: %v", rule.Selector, selectErr)
				return "", nil, 0, nil, selectErr
			}

			switch *handler {
			case types.HandlerPass:
				logger.Infof("selector %s select failed: %v, move to the next rule", rule.Selector, selectErr)
				continue
			case types.HandlerBlock:
				logger.Errorf("selector %s select failed: %v, block the request", rule.Selector, selectErr)
				service, err := GetService(selector.Service)
				if err != nil {
					logger.Warnf("service %s not found", selector.Service)
					continue
				}
				if _, ok := target.Ctx.Deadline(); !ok {
					logger.Warnf("context deadline not set")
				}
				flag := true
				for flag {
					select {
					case <-target.Ctx.Done():
						return "", nil, 0, nil, errcode.ErrContextCanceled
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
		}
		port := selector.Port
		if endpoint.Port != nil {
			port = *endpoint.Port
		}
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
