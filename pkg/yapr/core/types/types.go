package types

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type MatchTarget struct {
	URI     string            // 方法名
	Port    uint32            // Router 端口
	Headers map[string]string // headers
	Timeout <-chan struct{}   // 用于通知超时
}

// Endpoint 代表了一个服务的一个Endpoint，请勿直接构造，使用yaprsdk.NewEndpoint或者EndpointFromString
type Endpoint struct {
	IP   string
	Pod  string
	Port uint32
}

func (e *Endpoint) String() string {
	return fmt.Sprintf("%s#%s#%d", e.IP, e.Pod, e.Port)
}

func EndpointFromString(s string) *Endpoint {
	parts := strings.Split(s, "#")
	if len(parts) != 3 {
		return nil
	}
	portInt, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil
	}
	port := uint32(portInt)
	return &Endpoint{
		IP:   parts[0],
		Pod:  parts[1],
		Port: port,
	}
}

func EqualEndpoints(e1, e2 *Endpoint) bool {
	if e1 == nil && e2 == nil {
		return true
	}
	if e1 == nil || e2 == nil {
		return false
	}
	return e1.IP == e2.IP && e1.Port == e2.Port
}

type EndpointFilter func(*Selector, *Endpoint, *Attribute) bool

var (
	GoodEndpointFilter EndpointFilter = func(selector *Selector, endpoint *Endpoint, attribute *Attribute) bool {
		return attribute.IsGood()
	}

	FuseEndpointFilter EndpointFilter = func(selector *Selector, endpoint *Endpoint, attribute *Attribute) bool {
		return attribute.RPS == nil || selector.MaxRequests == nil || *attribute.RPS <= *selector.MaxRequests
	}
)

// Attributes 代表了一个Endpoint在服务中的所有属性
type Attributes struct {
	*CommonAttribute `yaml:"common" json:"common,omitempty"` // 通用属性

	InSelector map[string]*AttributeInSelector `yaml:"in_selector" json:"in_selector,omitempty"` // 在选择器中的属性
}

// CommonAttribute 代表了一个Endpoint的通用属性
type CommonAttribute struct {
	Available bool `yaml:"available" json:"available,omitempty"` // 是否可用
}

func (c *CommonAttribute) IsGood() bool {
	return c.Available
}

// AttributeInSelector 代表了一个Endpoint在选择器中的属性
type AttributeInSelector struct {
	Weight   *uint32 `yaml:"weight" json:"weight,omitempty"`     // 权重，默认为1
	RPS      *uint32 `yaml:"rps" json:"rps,omitempty"`           // 每秒请求数，开启熔断或选择LeastRequest策略时会自动填充
	Deadline *int64  `yaml:"deadline" json:"deadline,omitempty"` // 截止时间，单位毫秒，0表示永久，-1表示已过期，默认为0
}

// Attribute 代表了一个Endpoint在选择器中的属性，同时附带了通用属性
type Attribute struct {
	*CommonAttribute
	*AttributeInSelector
}

type BufferType string

const (
	BufferTypeNone = BufferType("none")
	BufferTypeLRU  = BufferType("lru")
)

type RuleError string

const (
	RuleErrorDefault             = RuleError("default")
	RuleErrorNoEndpoint          = RuleError("no_endpoint")
	RuleErrorSelectFailed        = RuleError("select_failed")
	RuleErrorTimeout             = RuleError("timeout")
	RuleErrorEndpointUnavailable = RuleError("unavailable")
)

type Solution string

const (
	SolutionDefault = Solution("default")
	SolutionRetry   = Solution("retry")
	SolutionPass    = Solution("pass")
	SolutionBlock   = Solution("block")
	SolutionThrow   = Solution("throw")
	SolutionPanic   = Solution("panic")
)

type ErrorHandler struct {
	Solution Solution `yaml:"solution" json:"solution,omitempty"` // #错误处理方案
	Times    *uint32  `yaml:"times" json:"times,omitempty"`       // #重试次数
	Interval *float32 `yaml:"interval" json:"interval,omitempty"` // #重试间隔，单位秒
	Timeout  *float32 `yaml:"timeout" json:"timeout,omitempty"`   // #超时时间，单位秒
	Message  *string  `yaml:"message" json:"message,omitempty"`   // #错误信息
	Code     *uint32  `yaml:"code" json:"code,omitempty"`         // #错误码
}

type Matcher struct {
	URI     string            `yaml:"uri" json:"uri,omitempty"`         // #方法名regex
	Port    uint32            `yaml:"port" json:"port,omitempty"`       // #Router 端口
	Headers map[string]string `yaml:"headers" json:"headers,omitempty"` // #对header的filters，对于所有header key都要满足指定regex

	regexURI     *regexp.Regexp
	regexHeaders map[string]*regexp.Regexp
}

func (m *Matcher) RegexURI() *regexp.Regexp {
	if m.regexURI == nil {
		m.regexURI = regexp.MustCompile(m.URI)
	}
	return m.regexURI
}

func (m *Matcher) RegexHeaders() map[string]*regexp.Regexp {
	if m.Headers == nil {
		return nil
	}
	if m.regexHeaders == nil {
		m.regexHeaders = make(map[string]*regexp.Regexp)
		for k, v := range m.Headers {
			m.regexHeaders[k] = regexp.MustCompile(v)
		}
	}
	return m.regexHeaders
}

// Rule 代表了一条路由规则，包含了匹配规则和目的地服务网格
type Rule struct {
	Matchers []*Matcher                  `yaml:"matchers" json:"matchers,omitempty"` // #匹配规则，满足任意一个规则则匹配成功
	Selector string                      `yaml:"selector" json:"selector,omitempty"` // #路由目的地选择器
	Catch    map[RuleError]*ErrorHandler `yaml:"catch" json:"catch,omitempty"`       // #错误处理
}

// Router 代表了一个服务网格的所有路由规则
type Router struct {
	Name   string  `yaml:"name" json:"name,omitempty"`     // #服务网格名
	Rules  []*Rule `yaml:"rules" json:"rules,omitempty"`   // #路由规则，按优先级从高到低排序
	Direct string  `yaml:"direct" json:"direct,omitempty"` // #直连，填写后将忽略路由选择器 TODO: Debug
}

const (
	StrategyRandom             = "random"               // 随机
	StrategyRoundRobin         = "round_robin"          // 轮询
	StrategyWeightedRandom     = "weighted_random"      // 加权随机
	StrategyWeightedRoundRobin = "weighted_round_robin" // 加权轮询
	StrategyLeastRequest       = "least_request"        // 最少请求，请求越多权重越低
	StrategyLeastCost          = "least_cost"           // 最少开销，开销越高权重越低（与最少请求类似，但是要由用户自己计算开销并上报）
	StrategyHashRing           = "hash_ring"            // 哈希环法
	StrategyDirect             = "direct"               // 指定目标路由
	StrategyCustomLua          = "custom_lua"           // 自定义
)

// Selector 全名是Endpoint Selector，指定了目标service和选择策略【以json格式存etcd】
type Selector struct {
	Name        string            `yaml:"name" json:"name,omitempty"`                 // #唯一名称
	Service     string            `yaml:"service" json:"service,omitempty"`           // #目标服务
	Port        uint32            `yaml:"port" json:"port,omitempty"`                 // #目标端口
	Headers     map[string]string `yaml:"headers" json:"headers,omitempty"`           // #路由成功后为请求添加的headers
	Strategy    string            `yaml:"strategy" json:"strategy,omitempty"`         // #路由策略，默认为random
	MaxRequests *uint32           `yaml:"max_requests" json:"max_requests,omitempty"` // #最大请求数，填写后将开启熔断，会增加上报请求数的开销（LeastRequest本来就要上报，就不会增加）
	Key         string            `yaml:"key" json:"key,omitempty"`                   // #用于从header中获取路由用的value，仅在一致性哈希和指定目标策略下有效
	CacheType   BufferType        `yaml:"cache_type" json:"cache_type,omitempty"`     // #动态键值路由缓存类型，仅在指定目标策略下有效，默认为none
	CacheSize   *uint32           `yaml:"cache_size" json:"cache_size,omitempty"`     // #动态键值路由缓存大小，仅在指定目标策略下有效，默认为4096
	Script      string            `yaml:"script" json:"script,omitempty"`             // #自定义lua脚本，仅在自定义策略下有效
}
