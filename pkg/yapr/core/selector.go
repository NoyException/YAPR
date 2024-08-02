package core

import (
	"fmt"
	"google.golang.org/grpc/metadata"
	"hash/fnv"
	"math/rand/v2"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
	"noy/router/pkg/yapr/metrics"
	"time"
)

// Selector 全名是Endpoint Selector，指定了目标service和选择策略【以json格式存etcd】
type Selector struct {
	Name       string            `yaml:"name" json:"name,omitempty"`               // #唯一名称
	Service    string            `yaml:"service" json:"service,omitempty"`         // #目标服务
	Port       uint32            `yaml:"port" json:"port,omitempty"`               // #目标端口
	Headers    map[string]string `yaml:"headers" json:"headers,omitempty"`         // #路由成功后为请求添加的headers
	Strategy   types.Strategy    `yaml:"strategy" json:"strategy,omitempty"`       // #路由策略，默认为random
	Key        string            `yaml:"key" json:"key,omitempty"`                 // #用于从header中获取路由用的value，仅在一致性哈希和指定目标策略下有效
	BufferType types.BufferType  `yaml:"buffer_type" json:"buffer_type,omitempty"` // #动态键值路由缓存类型，仅在指定目标策略下有效，默认为none
	BufferSize uint32            `yaml:"buffer_size" json:"buffer_size,omitempty"` // #动态键值路由缓存大小，仅在指定目标策略下有效，默认为4096
	//DirectMap    map[string]Endpoint `yaml:"-" json:"direct_map,omitempty"`      // 指定目标路由，从redis现存现取，表名$SelectorName，键值对为header value -> Endpoint

	Buffer types.DynamicRouteBuffer `yaml:"-" json:"-"` // 动态路由缓存

	lastIdx uint32 // 上次选择的endpoint索引
}

func (s *Selector) Select(service *Service, target *types.MatchTarget) (*types.Endpoint, error) {
	start := time.Now()
	defer func() {
		metrics.ObserveSelectorDuration(string(s.Strategy), time.Since(start).Seconds())
	}()

	switch s.Strategy {
	case types.StrategyRandom:
		return s.randomSelect(service)
	case types.StrategyRoundRobin:
		return s.roundRobinSelect(service)
	case types.StrategyWeightedRandom:
		return s.weightedRandomSelect(service)
	case types.StrategyWeightedRoundRobin:
		return s.weightedRoundRobinSelect(service)
	case types.StrategyLeastCost:
		return s.leastCostSelect(service)
	case types.StrategyConsistentHash:
		return s.consistentHashSelect(service, target)
	case types.StrategyDirect:
		return s.directSelect(service, target)
	default:
		return nil, fmt.Errorf("unknown strategy: %s", s.Strategy)
	}
}

func (s *Selector) randomSelect(service *Service) (*types.Endpoint, error) {
	endpoints := service.Endpoints()
	if len(endpoints) == 0 {
		return nil, ErrNoEndpointAvailable
	}
	return endpoints[rand.IntN(len(endpoints))], nil
}

func (s *Selector) roundRobinSelect(service *Service) (*types.Endpoint, error) {
	endpoints := service.Endpoints()
	if len(endpoints) == 0 {
		return nil, ErrNoEndpointAvailable
	}
	s.lastIdx = (s.lastIdx + 1) % uint32(len(endpoints))
	return endpoints[s.lastIdx], nil
}

func (s *Selector) weightedRandomSelect(service *Service) (*types.Endpoint, error) {
	endpoints := service.Endpoints()
	if len(endpoints) == 0 {
		return nil, ErrNoEndpointAvailable
	}
	attributes := service.Attributes(s.Name)

	totalWeight := uint32(0)
	for _, attr := range attributes {
		totalWeight += attr.Weight
	}
	r := rand.Uint32() % totalWeight
	totalWeight = 0
	for i, attr := range attributes {
		totalWeight += attr.Weight
		if r < totalWeight {
			return endpoints[i], nil
		}
	}
	return nil, ErrNoEndpointAvailable
}

func (s *Selector) weightedRoundRobinSelect(service *Service) (*types.Endpoint, error) {
	endpoints := service.Endpoints()
	if len(endpoints) == 0 {
		return nil, ErrNoEndpointAvailable
	}
	attributes := service.Attributes(s.Name)

	s.lastIdx++
	total := uint32(0)
	for i, attr := range attributes {
		total += attr.Weight
		if s.lastIdx < total {
			return endpoints[i], nil
		}
	}
	s.lastIdx = 0
	if total == 0 {
		return nil, ErrNoEndpointAvailable
	}
	return endpoints[0], nil
}

// 实现上是选取最大权重（weight = C - cost）
func (s *Selector) leastCostSelect(service *Service) (*types.Endpoint, error) {
	endpoints := service.Endpoints()
	if len(endpoints) == 0 {
		return nil, ErrNoEndpointAvailable
	}
	attributes := service.Attributes(s.Name)

	idx := -1
	maxWeight := uint32(0)
	for i, attr := range attributes {
		if idx == -1 || attr.Weight > maxWeight {
			maxWeight = attr.Weight
			idx = i
		}
	}
	if idx == -1 {
		return nil, ErrNoEndpointAvailable
	}
	return endpoints[idx], nil
}

func JumpConsistentHash(key uint64, numBuckets int32) int32 {
	if numBuckets <= 0 {
		panic("numBuckets must be greater than 0")
	}
	var b, j int64 = -1, 0
	for j < int64(numBuckets) {
		b = j
		key = key*2862933555777941757 + 1
		j = int64(float64(b+1) * (float64(1<<31) / float64((key>>33)+1)))
	}
	return int32(b)
}

func hashString(s string) uint64 {
	h := fnv.New64a()
	_, err := h.Write([]byte(s))
	if err != nil {
		logger.Errorf("hash string failed: %v", err)
		return 0
	}
	return h.Sum64()
}

func (s *Selector) headerValue(target *types.MatchTarget) (string, error) {
	if s.Key == "" {
		return "", ErrNoKeyAvailable
	}

	md, exist := metadata.FromOutgoingContext(target.Ctx)
	if !exist {
		return "", ErrNoValueAvailable
	}
	values := md.Get(s.Key)
	if len(values) == 0 {
		return "", ErrNoValueAvailable
	}
	return values[0], nil
}

// TODO：新增或删除endpoint时，该如何处理？
func (s *Selector) consistentHashSelect(service *Service, target *types.MatchTarget) (*types.Endpoint, error) {
	if s.Key == "" {
		return nil, ErrNoKeyAvailable
	}

	endpoints := service.Endpoints()
	if len(endpoints) == 0 {
		return nil, ErrNoEndpointAvailable
	}

	value, err := s.headerValue(target)
	if err != nil {
		return nil, err
	}
	hashed := hashString(value)
	idx := JumpConsistentHash(hashed, int32(len(endpoints)))
	return endpoints[idx], nil
}

// TODO：新增或删除endpoint时，该如何处理？
func (s *Selector) directSelect(service *Service, target *types.MatchTarget) (*types.Endpoint, error) {
	if s.Key == "" {
		return nil, ErrNoKeyAvailable
	}

	endpoints := service.EndpointsSet()
	if len(endpoints) == 0 {
		return nil, ErrNoEndpointAvailable
	}

	value, err := s.headerValue(target)
	if err != nil {
		return nil, err
	}

	if s.Buffer == nil {
		return nil, ErrBufferNotFound
	}

	endpoint, err := s.Buffer.Get(value)
	if err != nil {
		return nil, err
	}
	if endpoint == nil {
		return nil, ErrNoEndpointAvailable
	}
	// endpoint有可能已经被删除，需要检查
	if _, ok := endpoints[*endpoint]; !ok {
		return endpoint, ErrBadEndpoint
	}
	logger.Debugf("direct select endpoint %v by value %v", endpoint, value)
	return endpoint, nil
}
