package types

import "context"

type MatchTarget struct {
	URI  string          // 方法名
	Port uint32          // Router 端口
	Ctx  context.Context // headers
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

// DynamicRouteBuffer 动态键值路由缓存
type DynamicRouteBuffer interface {
	Get(headerValue string) (*Endpoint, error)
	Set(headerValue string, endpoint *Endpoint, timeout int64) error
	Remove(headerValue string) error
	Refresh(headerValue string)
	Clear()
}

type BufferType string

const (
	BufferTypeNone = BufferType("none")
	BufferTypeLRU  = BufferType("lru")
)
