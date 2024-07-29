package core

import "context"

// 带有*的字段表示是可以被更新的字段，需要在etcd中监听变化
// 带有#的字段表示是会从配置文件中加载的字段

type MatchTarget struct {
	Uri  string          // 方法名
	Port uint32          // Router 端口
	Ctx  context.Context // headers
}

type Matcher struct {
	URI     string            `yaml:"uri" json:"uri,omitempty"`         // #方法名regex
	Port    uint32            `yaml:"port" json:"port,omitempty"`       // #Router 端口
	Headers map[string]string `yaml:"headers" json:"headers,omitempty"` // #对header的filters，对于所有header key都要满足指定regex
}

type Strategy string

const (
	StrategyRandom             = Strategy("random")               // 随机
	StrategyRoundRobin         = Strategy("round_robin")          // 轮询
	StrategyWeightedRandom     = Strategy("weighted_random")      // 加权随机
	StrategyWeightedRoundRobin = Strategy("weighted_round_robin") // 加权轮询
	StrategyConsistentHash     = Strategy("consistent_hash")      // 一致性哈希
	StrategyLeastActive        = Strategy("least_active")         // 最少活跃度，权重越高活跃度越低
	StrategyDirect             = Strategy("direct")               // 指定目标路由
)

type Endpoint struct {
	//Pod  string `yaml:"pod" json:"pod,omitempty"`   // pod
	IP string `yaml:"ip" json:"ip,omitempty"` // ip
}

type Attr struct {
	Weight   uint32 `yaml:"weight" json:"weight,omitempty"`     // 权重，默认为1
	Deadline int64  `yaml:"deadline" json:"deadline,omitempty"` // 截止时间，单位毫秒，0表示永久
}

type Service struct {
	Name    string                        `yaml:"name" json:"name,omitempty"`  // #唯一名称
	AttrMap map[Endpoint]*map[string]Attr `yaml:"-" json:"attr_map,omitempty"` // *每个endpoint和他在某个 Selector 中的属性【etcd中保存为Selector_$Name/AttrMap_$Endpoint_$Selector -> $Attr】
}

// Selector 全名是Endpoint Selector，指定了目标service和选择策略【散装地存etcd】
type Selector struct {
	Name     string            `yaml:"name" json:"name,omitempty"`         // #唯一名称
	Service  string            `yaml:"service" json:"service,omitempty"`   // #目标服务
	Port     uint32            `yaml:"port" json:"port,omitempty"`         // #目标端口
	Headers  map[string]string `yaml:"headers" json:"headers,omitempty"`   // #路由成功后为请求添加的headers
	Strategy Strategy          `yaml:"strategy" json:"strategy,omitempty"` // #路由策略【etcd中保存为Selector_$Name/Strategy -> $Strategy】
	Key      string            `yaml:"key" json:"key,omitempty"`           // #用于从header中获取路由用的value，仅在一致性哈希和指定目标策略下有效【etcd中保存为Selector_$Name/Key -> $HeaderKey】
	//DirectMap    map[string]Endpoint `yaml:"-" json:"direct_map,omitempty"`      // 指定目标路由，从redis现存现取，表名$SelectorName，键值对为header value -> Endpoint
}

// Rule 代表了一条路由规则，包含了匹配规则和目的地服务网格
type Rule struct {
	Priority int32      `yaml:"priority" json:"priority,omitempty"` // #优先级，数字越小优先级越高，相同优先级按照注册顺序排序
	Matchers []*Matcher `yaml:"matchers" json:"matchers,omitempty"` // #匹配规则，满足任意一个规则则匹配成功
	Selector string     `yaml:"selector" json:"selector,omitempty"` // #路由目的地选择器
}

// Router 代表了一个服务网格的所有路由规则【以json格式存etcd】
type Router struct {
	Name           string               `yaml:"name" json:"name,omitempty"`          // #服务网格名
	Rules          []*Rule              `yaml:"rules" json:"rules,omitempty"`        // #路由规则，按优先级从高到低排序
	SelectorByName map[string]*Selector `yaml:"-" json:"selector_by_name,omitempty"` // #所有路由选择器，用于快速查找
	ServiceByName  map[string]*Service  `yaml:"-" json:"service_by_name,omitempty"`  // #所有服务，用于快速查找
}
