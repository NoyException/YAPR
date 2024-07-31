package core

import "context"

// 带有*的字段表示是可以被更新的字段，需要在etcd中监听变化
// 带有#的字段表示是会从配置文件中加载的字段

type MatchTarget struct {
	URI  string          // 方法名
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
	StrategyLeastCost          = Strategy("least_cost")           // 最少开销，开销越高权重越低
	StrategyConsistentHash     = Strategy("consistent_hash")      // 一致性哈希
	StrategyDirect             = Strategy("direct")               // 指定目标路由
)

type Endpoint struct {
	//Pod  string `yaml:"pod" json:"pod,omitempty"`   // pod
	IP string `yaml:"ip" json:"ip,omitempty"` // ip
}

type Attribute struct {
	Weight   uint32 `yaml:"weight" json:"weight,omitempty"`     // 权重，默认为1
	Deadline int64  `yaml:"deadline" json:"deadline,omitempty"` // 截止时间，单位毫秒，0表示永久，-1表示已过期仅在
}

type Service struct {
	Name           string                             `yaml:"name" json:"name,omitempty"`          // #唯一名称
	AttrMap        map[Endpoint]map[string]*Attribute `yaml:"-" json:"attr_map,omitempty"`         // *每个endpoint和他在某个 Selector 中的属性【etcd中保存为slt/$Name/AttrMap_$Endpoint -> $Attr】
	EndpointsByPod map[string][]*Endpoint             `yaml:"-" json:"endpoints_by_pod,omitempty"` // *每个pod对应的endpoints，【etcd不保存】

	dirty      bool                    // 是否需要更新endpoints
	endpoints  []*Endpoint             // 所有的endpoints
	attributes []map[string]*Attribute // 所有的属性，idx和endpoints对应，selectorName -> attr

	//EndpointsAddNtf chan *Endpoint // *Endpoints更新通知
	//EndpointsDelNtf chan *Endpoint // *Endpoints删除通知
}

// Selector 全名是Endpoint Selector，指定了目标service和选择策略【以json格式存etcd】
type Selector struct {
	Name     string            `yaml:"name" json:"name,omitempty"`         // #唯一名称
	Service  string            `yaml:"service" json:"service,omitempty"`   // #目标服务
	Port     uint32            `yaml:"port" json:"port,omitempty"`         // #目标端口
	Headers  map[string]string `yaml:"headers" json:"headers,omitempty"`   // #路由成功后为请求添加的headers
	Strategy Strategy          `yaml:"strategy" json:"strategy,omitempty"` // #路由策略
	Key      string            `yaml:"key" json:"key,omitempty"`           // #用于从header中获取路由用的value，仅在一致性哈希和指定目标策略下有效
	//DirectMap    map[string]Endpoint `yaml:"-" json:"direct_map,omitempty"`      // 指定目标路由，从redis现存现取，表名$SelectorName，键值对为header value -> Endpoint

	lastIdx uint32 // 上次选择的endpoint索引
}

// Rule 代表了一条路由规则，包含了匹配规则和目的地服务网格
type Rule struct {
	Priority int32      `yaml:"priority" json:"priority,omitempty"` // #优先级，数字越小优先级越高，相同优先级按照注册顺序排序
	Matchers []*Matcher `yaml:"matchers" json:"matchers,omitempty"` // #匹配规则，满足任意一个规则则匹配成功
	Selector string     `yaml:"selector" json:"selector,omitempty"` // #路由目的地选择器
}

// Router 代表了一个服务网格的所有路由规则【以json格式存etcd】
type Router struct {
	Name           string               `yaml:"name" json:"name,omitempty"`   // #服务网格名
	Rules          []*Rule              `yaml:"rules" json:"rules,omitempty"` // #路由规则，按优先级从高到低排序
	SelectorByName map[string]*Selector `yaml:"-" json:"-"`                   // #所有路由选择器，用于快速查找
	ServiceByName  map[string]*Service  `yaml:"-" json:"-"`                   // #所有服务，用于快速查找
}

// Config 代表了所有服务网格的配置【以yaml文件保存】
type Config struct {
	Routers   []*Router   `yaml:"routers" json:"routers,omitempty"`     // #所有服务网格
	Selectors []*Selector `yaml:"selectors" json:"selectors,omitempty"` // #所有路由选择器
}
