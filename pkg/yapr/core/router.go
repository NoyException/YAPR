package core

import (
	"google.golang.org/grpc/metadata"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
	"regexp"
)

type Matcher struct {
	URI     string            `yaml:"uri" json:"uri,omitempty"`         // #方法名regex
	Port    uint32            `yaml:"port" json:"port,omitempty"`       // #Router 端口
	Headers map[string]string `yaml:"headers" json:"headers,omitempty"` // #对header的filters，对于所有header key都要满足指定regex
}

// Rule 代表了一条路由规则，包含了匹配规则和目的地服务网格
type Rule struct {
	Priority int32      `yaml:"priority" json:"priority,omitempty"` // #优先级，数字越小优先级越高，相同优先级按照注册顺序排序
	Matchers []*Matcher `yaml:"matchers" json:"matchers,omitempty"` // #匹配规则，满足任意一个规则则匹配成功
	Selector string     `yaml:"selector" json:"selector,omitempty"` // #路由目的地选择器
}

// Router 代表了一个服务网格的所有路由规则
type Router struct {
	Name           string               `yaml:"name" json:"name,omitempty"`   // #服务网格名
	Rules          []*Rule              `yaml:"rules" json:"rules,omitempty"` // #路由规则，按优先级从高到低排序
	SelectorByName map[string]*Selector `yaml:"-" json:"-"`                   // #所有路由选择器，用于快速查找
	ServiceByName  map[string]*Service  `yaml:"-" json:"-"`                   // #所有服务，用于快速查找
}

func (m *Matcher) Match(target *types.MatchTarget) bool {
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

func (r *Rule) Match(target *types.MatchTarget) bool {
	for _, matcher := range r.Matchers {
		if matcher.Match(target) {
			return true
		}
	}
	return false
}

func (r *Router) Route(target *types.MatchTarget) (string, *types.Endpoint, uint32, metadata.MD, error) {
	for _, rule := range r.Rules {
		if rule.Match(target) {
			if selector, ok := r.SelectorByName[rule.Selector]; ok {
				service, ok := r.ServiceByName[selector.Service]
				if !ok {
					logger.Warnf("service %s not found", selector.Service)
					continue
				}
				endpoint, err := selector.Select(service, target)
				if err != nil {
					logger.Infof("selector %s select failed: %v, move to the next rule", rule.Selector, err)
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
