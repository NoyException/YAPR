package core

import (
	"fmt"
	"google.golang.org/grpc/metadata"
	"regexp"
)

var router *Router

func MustRouter(name string) *Router {
	if router == nil {
		//router = NewRouter()
	}
	return router
}

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

func (r *Router) Route(target *MatchTarget) (*Selector, error) {
	for _, rule := range r.Rules {
		if rule.Match(target) {
			return r.SelectorByName[rule.Selector], nil
		}
	}
	return nil, fmt.Errorf("no rule matched")
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
