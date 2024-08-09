package types

import (
	"context"
	"encoding/json"
	"noy/router/pkg/yapr/logger"
)

type MatchTarget struct {
	URI  string          // 方法名
	Port uint32          // Router 端口
	Ctx  context.Context // headers
}

type Endpoint struct {
	IP   string  `yaml:"ip" json:"ip,omitempty"`     // ip
	Pod  string  `yaml:"pod" json:"pod,omitempty"`   // pod
	Port *uint32 `yaml:"port" json:"port,omitempty"` // port，不填则默认为Selector的Port
}

func (e *Endpoint) String() string {
	bytes, err := json.Marshal(e)
	if err != nil {
		logger.Errorf("Endpoint marshal error: %v", err)
		return ""
	}
	return string(bytes)
}

func EndpointFromString(s string) *Endpoint {
	e := &Endpoint{}
	err := json.Unmarshal([]byte(s), e)
	if err != nil {
		logger.Errorf("Endpoint unmarshal error: %v", err)
		return nil
	}
	return e
}

func (e *Endpoint) Equal(e2 *Endpoint) bool {
	return e.IP == e2.IP && e.Port == e2.Port
}

// Attributes 代表了一个服务的属性
type Attributes struct {
	Available  bool                  `yaml:"available" json:"available,omitempty"`     // 是否可用
	InSelector map[string]*Attribute `yaml:"in_selector" json:"in_selector,omitempty"` // 在选择器中的属性
}

// Attribute 代表了一个服务中某个Endpoint的属性
type Attribute struct {
	Weight   uint32 `yaml:"weight" json:"weight,omitempty"`     // 权重，默认为1
	Deadline int64  `yaml:"deadline" json:"deadline,omitempty"` // 截止时间，单位毫秒，0表示永久，-1表示已过期
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

type ErrorHandler string

const (
	HandlerPass  = ErrorHandler("pass")
	HandlerBlock = ErrorHandler("block")
)

type Matcher struct {
	URI     string            `yaml:"uri" json:"uri,omitempty"`         // #方法名regex
	Port    uint32            `yaml:"port" json:"port,omitempty"`       // #Router 端口
	Headers map[string]string `yaml:"headers" json:"headers,omitempty"` // #对header的filters，对于所有header key都要满足指定regex
}

// Rule 代表了一条路由规则，包含了匹配规则和目的地服务网格
type Rule struct {
	Matchers     []*Matcher                  `yaml:"matchers" json:"matchers,omitempty"`           // #匹配规则，满足任意一个规则则匹配成功
	Selector     string                      `yaml:"selector" json:"selector,omitempty"`           // #路由目的地选择器
	ErrorHandler map[RuleError]*ErrorHandler `yaml:"error_handler" json:"error_handler,omitempty"` // #错误处理
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
	StrategyLeastCost          = "least_cost"           // 最少开销，开销越高权重越低
	StrategyHashRing           = "hash_ring"            // 哈希环法
	StrategyDirect             = "direct"               // 指定目标路由
	StrategyCustomLua          = "custom_lua"           // 自定义
)

// Selector 全名是Endpoint Selector，指定了目标service和选择策略【以json格式存etcd】
type Selector struct {
	Name      string            `yaml:"name" json:"name,omitempty"`             // #唯一名称
	Service   string            `yaml:"service" json:"service,omitempty"`       // #目标服务
	Port      uint32            `yaml:"port" json:"port,omitempty"`             // #目标端口
	Headers   map[string]string `yaml:"headers" json:"headers,omitempty"`       // #路由成功后为请求添加的headers
	Strategy  string            `yaml:"strategy" json:"strategy,omitempty"`     // #路由策略，默认为random
	Key       string            `yaml:"key" json:"key,omitempty"`               // #用于从header中获取路由用的value，仅在一致性哈希和指定目标策略下有效
	CacheType BufferType        `yaml:"cache_type" json:"cache_type,omitempty"` // #动态键值路由缓存类型，仅在指定目标策略下有效，默认为none
	CacheSize uint32            `yaml:"cache_size" json:"cache_size,omitempty"` // #动态键值路由缓存大小，仅在指定目标策略下有效，默认为4096
	Script    string            `yaml:"script" json:"script,omitempty"`         // #自定义lua脚本，仅在自定义策略下有效
	//DirectMap    map[string]Endpoint `yaml:"-" json:"direct_map,omitempty"`      // 指定目标路由，从redis现存现取，表名$SelectorName，键值对为header value -> Endpoint
}
