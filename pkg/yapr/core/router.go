package core

import (
	"google.golang.org/grpc/metadata"
	"noy/router/pkg/yapr/logger"
	"regexp"
)

func (m *Matcher) Match(target *MatchTarget) bool {
	matched, err := regexp.Match(m.URI, []byte(target.Uri))
	if err != nil || !matched {
		return false
	}
	if m.Port != 0 && target.Port != m.Port {
		return false
	}
	if m.Headers != nil {
		md, ok := metadata.FromOutgoingContext(target.Ctx)
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

func (r *Rule) Match(target *MatchTarget) bool {
	for _, matcher := range r.Matchers {
		if matcher.Match(target) {
			return true
		}
	}
	return false
}

func (r *Router) Route(target *MatchTarget) (string, *Endpoint, uint32, metadata.MD, error) {
	for _, rule := range r.Rules {
		if rule.Match(target) {
			if selector, ok := r.SelectorByName[rule.Selector]; ok {
				service, ok := r.ServiceByName[selector.Service]
				if !ok {
					logger.Warnf("service %s not found", selector.Service)
					continue
				}
				endpoint, err := selector.Select(service)
				if err != nil {
					logger.Warnf("selector %s select failed: %v", rule.Selector, err)
					continue
				}
				return selector.Service, endpoint, selector.Port, metadata.New(selector.Headers), nil
			} else {
				logger.Warnf("selector %s not found", rule.Selector)
			}
		}
	}
	return "", nil, 0, nil, ErrNoRuleMatched
}

func (r *Router) Selectors() []*Selector {
	selectorNames := make(map[string]bool)
	for _, rule := range r.Rules {
		selectorNames[rule.Selector] = true
	}
	selectors := make([]*Selector, 0, len(selectorNames))
	for name, _ := range selectorNames {
		selectors = append(selectors, r.SelectorByName[name])
	}
	return selectors
}
