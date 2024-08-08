package core

import (
	lua "github.com/yuin/gopher-lua"
	"google.golang.org/grpc/metadata"
	"math/rand/v2"
	"noy/router/pkg/yapr/core/errcode"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
)

type StrategyBuilder interface {
	Build(s *Selector) (Strategy, error)
}

type Strategy interface {
	// Select 选择一个endpoint，其中headers应当基于selector.BaseHeaders()构建
	Select(service *Service, match *types.MatchTarget) (endpoint *types.Endpoint, headers map[string]string, err error)
}

var strategyBuilders = make(map[string]StrategyBuilder)

func RegisterStrategyBuilder(name string, builder StrategyBuilder) {
	strategyBuilders[name] = builder
}

type RandomStrategyBuilder struct{}

func (b *RandomStrategyBuilder) Build(s *Selector) (Strategy, error) {
	return &RandomStrategy{s}, nil
}

type RandomStrategy struct {
	selector *Selector
}

func (r *RandomStrategy) Select(service *Service, _ *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
	endpoints := service.Endpoints()
	if len(endpoints) == 0 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}
	return endpoints[rand.IntN(len(endpoints))], r.selector.BaseHeaders(), nil
}

type RoundRobinStrategyBuilder struct{}

func (b *RoundRobinStrategyBuilder) Build(s *Selector) (Strategy, error) {
	return &RoundRobinStrategy{s}, nil
}

type RoundRobinStrategy struct {
	selector *Selector
}

func (r *RoundRobinStrategy) Select(service *Service, _ *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
	s := r.selector
	endpoints := service.Endpoints()
	if len(endpoints) == 0 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}
	s.lastIdx = (s.lastIdx + 1) % uint32(len(endpoints))
	return endpoints[s.lastIdx], s.BaseHeaders(), nil
}

type WeightedRandomStrategyBuilder struct{}

func (b *WeightedRandomStrategyBuilder) Build(s *Selector) (Strategy, error) {
	return &WeightedRandomStrategy{s}, nil
}

type WeightedRandomStrategy struct {
	selector *Selector
}

func (r *WeightedRandomStrategy) Select(service *Service, _ *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
	s := r.selector
	endpoints := service.Endpoints()
	if len(endpoints) == 0 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}
	attributes := service.AttributesInSelector(s.Name)

	totalWeight := uint32(0)
	for _, attr := range attributes {
		totalWeight += attr.Weight
	}
	rnd := rand.Uint32() % totalWeight
	totalWeight = 0
	for i, attr := range attributes {
		totalWeight += attr.Weight
		if rnd < totalWeight {
			return endpoints[i], s.BaseHeaders(), nil
		}
	}
	return nil, nil, errcode.ErrNoEndpointAvailable
}

type WeightedRoundRobinStrategyBuilder struct{}

func (b *WeightedRoundRobinStrategyBuilder) Build(s *Selector) (Strategy, error) {
	return &WeightedRoundRobinStrategy{s}, nil
}

type WeightedRoundRobinStrategy struct {
	selector *Selector
}

// Select 实现上是选取最大权重（weight = C - cost）
func (r *WeightedRoundRobinStrategy) Select(service *Service, _ *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
	s := r.selector
	endpoints := service.Endpoints()
	if len(endpoints) == 0 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}
	attributes := service.AttributesInSelector(s.Name)

	s.lastIdx++
	total := uint32(0)
	for i, attr := range attributes {
		total += attr.Weight
		if s.lastIdx < total {
			return endpoints[i], s.BaseHeaders(), nil
		}
	}
	s.lastIdx = 0
	if total == 0 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}
	return endpoints[0], s.BaseHeaders(), nil
}

type LeastCostStrategyBuilder struct{}

func (b *LeastCostStrategyBuilder) Build(s *Selector) (Strategy, error) {
	return &LeastCostStrategy{s}, nil
}

type LeastCostStrategy struct {
	selector *Selector
}

func (r *LeastCostStrategy) Select(service *Service, _ *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
	s := r.selector
	endpoints := service.Endpoints()
	if len(endpoints) == 0 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}
	attributes := service.AttributesInSelector(s.Name)

	idx := -1
	maxWeight := uint32(0)
	for i, attr := range attributes {
		if idx == -1 || attr.Weight > maxWeight {
			maxWeight = attr.Weight
			idx = i
		}
	}
	if idx == -1 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}
	return endpoints[idx], s.BaseHeaders(), nil
}

//func JumpConsistentHash(key uint64, numBuckets int32) int32 {
//	if numBuckets <= 0 {
//		panic("numBuckets must be greater than 0")
//	}
//	var b, j int64 = -1, 0
//	for j < int64(numBuckets) {
//		b = j
//		key = key*2862933555777941757 + 1
//		j = int64(float64(b+1) * (float64(1<<31) / float64((key>>33)+1)))
//	}
//	return int32(b)
//}
//
//func hashString(s string) uint64 {
//	h := fnv.New64a()
//	_, err := h.Write([]byte(s))
//	if err != nil {
//		logger.Errorf("hash string failed: %v", err)
//		return 0
//	}
//	return h.Sum64()
//}
//// TODO: 增或删除endpoint时，该如何处理？
//func (s *Selector) jumpConsistentHashSelect(service *Service, target *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
//	if s.Key == "" {
//		return nil, nil, errcode.ErrNoKeyAvailable
//	}
//
//	endpoints := service.Endpoints()
//	if len(endpoints) == 0 {
//		return nil, nil, errcode.ErrNoEndpointAvailable
//	}
//
//	value, err := s.HeaderValue(target)
//	if err != nil {
//		return nil, nil, err
//	}
//	hashed := hashString(value)
//	idx := JumpConsistentHash(hashed, int32(len(endpoints)))
//
//	headers := s.BaseHeaders()
//	headers["yapr-header-value"] = value
//	return endpoints[idx], headers, nil
//}

type DirectStrategyBuilder struct{}

func (b *DirectStrategyBuilder) Build(s *Selector) (Strategy, error) {
	return &DirectStrategy{s}, nil
}

type DirectStrategy struct {
	selector *Selector
}

func (r *DirectStrategy) Select(service *Service, match *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
	s := r.selector
	if s.Key == "" {
		return nil, nil, errcode.ErrNoKeyAvailable
	}

	endpoints := service.EndpointsSet()
	if len(endpoints) == 0 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}

	value, err := s.HeaderValue(match)
	if err != nil {
		return nil, nil, err
	}

	if s.Cache == nil {
		return nil, nil, errcode.ErrBufferNotFound
	}

	endpoint, err := s.Cache.Get(value)
	if err != nil {
		return nil, nil, err
	}
	if endpoint == nil {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}
	// endpoint有可能已经被删除，需要检查
	headers := s.BaseHeaders()
	headers["yapr-header-value"] = value
	if _, ok := endpoints[*endpoint]; !ok {
		return endpoint, headers, errcode.ErrEndpointUnavailable
	}
	logger.Debugf("direct select endpoint %v by value %v", endpoint, value)
	return endpoint, headers, nil
}

type CustomLuaStrategyBuilder struct{}

func (b *CustomLuaStrategyBuilder) Build(s *Selector) (Strategy, error) {
	return &CustomLuaStrategy{s}, nil
}

type CustomLuaStrategy struct {
	selector *Selector
}

func (r *CustomLuaStrategy) Select(service *Service, match *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
	s := r.selector
	luaState := lua.NewState()
	defer luaState.Close()
	err := luaState.DoFile(s.Script)
	if err != nil {
		return nil, nil, err
	}
	md, exist := metadata.FromOutgoingContext(match.Ctx)
	if !exist {
		return nil, nil, errcode.ErrNoValueAvailable
	}
	luaHeaders := luaState.NewTable()
	for k, vs := range md {
		luaHeaders.RawSetString(k, lua.LString(vs[0]))
	}
	endpoints := service.Endpoints()
	// 创建lua数组
	luaEndpoints := luaState.NewTable()
	for i, endpoint := range endpoints {
		luaEndpoints.Insert(i+1, lua.LString(endpoint.String()))
	}
	logger.Debugf("header x-uid: %v", luaHeaders.RawGetString("x-uid"))
	err = luaState.CallByParam(lua.P{
		Fn:   luaState.GetGlobal("select"),
		NRet: 1,
	}, luaHeaders, luaEndpoints)
	if err != nil {
		return nil, nil, err
	}
	idx := luaState.ToInt(-1)
	if idx == -1 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}
	if idx < 0 || idx >= len(endpoints) {
		return nil, nil, errcode.ErrLuaIndexOutOfRange
	}
	headers := s.BaseHeaders()
	//TODO: 从lua中获取header
	return endpoints[idx], headers, nil
}

func init() {
	RegisterStrategyBuilder(types.StrategyRandom, &RandomStrategyBuilder{})
	RegisterStrategyBuilder(types.StrategyRoundRobin, &RoundRobinStrategyBuilder{})
	RegisterStrategyBuilder(types.StrategyWeightedRandom, &WeightedRandomStrategyBuilder{})
	RegisterStrategyBuilder(types.StrategyWeightedRoundRobin, &WeightedRoundRobinStrategyBuilder{})
	RegisterStrategyBuilder(types.StrategyLeastCost, &LeastCostStrategyBuilder{})
	RegisterStrategyBuilder(types.StrategyDirect, &DirectStrategyBuilder{})
	RegisterStrategyBuilder(types.StrategyCustomLua, &CustomLuaStrategyBuilder{})
}

func GetStrategy(selector *Selector) (Strategy, error) {
	builder, ok := strategyBuilders[selector.Strategy]
	if !ok {
		return nil, errcode.ErrUnknownStrategy
	}
	return builder.Build(selector)
}
