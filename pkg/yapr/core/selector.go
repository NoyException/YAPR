package core

import (
	"fmt"
	lua "github.com/yuin/gopher-lua"
	"google.golang.org/grpc/metadata"
	"hash/fnv"
	"math/rand/v2"
	"noy/router/pkg/yapr/core/errcode"
	"noy/router/pkg/yapr/core/store"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
	"noy/router/pkg/yapr/metrics"
	"time"
)

// Selector 全名是Endpoint Selector，指定了目标service和选择策略【以json格式存etcd】
type Selector struct {
	*types.Selector

	lastIdx uint32 // 上次选择的endpoint索引
}

var selectors map[string]*Selector

func GetSelector(name string) (*Selector, error) {
	if selectors == nil {
		s, err := store.MustStore().GetSelectors()
		if err != nil {
			logger.Errorf("selectors not found")
			return nil, err
		}
		selectors = make(map[string]*Selector)
		for selectorName, raw := range s {
			selectors[selectorName] = &Selector{Selector: raw}
		}
	}
	if selector, ok := selectors[name]; ok {
		return selector, nil
	}
	return nil, errcode.ErrSelectorNotFound
}

func (s *Selector) RefreshCache(headerValue string) {
	if s.Cache == nil {
		return
	}
	s.Cache.Refresh(headerValue)
}

func (s *Selector) Endpoints() map[types.Endpoint]*types.Attribute {
	result := make(map[types.Endpoint]*types.Attribute)
	service, err := GetService(s.Service)
	if err != nil {
		logger.Errorf("get service %s failed: %v", s.Service, err)
		return result
	}
	endpoints := service.Endpoints()
	if len(endpoints) == 0 {
		return result
	}
	attributes := service.AttributesInSelector(s.Name)
	for i := 0; i < len(endpoints); i++ {
		result[*endpoints[i]] = attributes[i]
	}
	return result
}

func (s *Selector) Select(target *types.MatchTarget) (endpoint *types.Endpoint, headers map[string]string, err error) {
	start := time.Now()
	defer func() {
		metrics.ObserveSelectorDuration(s.Strategy, time.Since(start).Seconds())
	}()

	service, err := GetService(s.Service)
	if err != nil {
		logger.Errorf("get service %s failed: %v", s.Service, err)
		return nil, nil, err
	}

	switch s.Strategy {
	case types.StrategyRandom:
		endpoint, headers, err = s.randomSelect(service)
	case types.StrategyRoundRobin:
		endpoint, headers, err = s.roundRobinSelect(service)
	case types.StrategyWeightedRandom:
		endpoint, headers, err = s.weightedRandomSelect(service)
	case types.StrategyWeightedRoundRobin:
		endpoint, headers, err = s.weightedRoundRobinSelect(service)
	case types.StrategyLeastCost:
		endpoint, headers, err = s.leastCostSelect(service)
	case types.StrategyHashRing:
		endpoint, headers, err = s.hashRingSelect(service, target)
	case types.StrategyJumpConsistentHash:
		endpoint, headers, err = s.jumpConsistentHashSelect(service, target)
	case types.StrategyDirect:
		endpoint, headers, err = s.directSelect(service, target)
	case types.StrategyCustom:
		endpoint, headers, err = s.selectByLua(service, target)
	default:
		endpoint, headers, err = nil, s.baseHeaders(), fmt.Errorf("unknown strategy: %s", s.Strategy)
	}
	if endpoint != nil && err == nil && !service.AttrMap[*endpoint].Available {
		err = errcode.ErrEndpointUnavailable
	}
	return
}

func (s *Selector) baseHeaders() map[string]string {
	headers := map[string]string{
		"yapr-strategy": s.Strategy,
		"yapr-selector": s.Name,
		"yapr-service":  s.Service,
	}
	for k, v := range s.Headers {
		headers[k] = v
	}
	return headers
}

func (s *Selector) randomSelect(service *Service) (*types.Endpoint, map[string]string, error) {
	endpoints := service.Endpoints()
	if len(endpoints) == 0 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}
	return endpoints[rand.IntN(len(endpoints))], s.baseHeaders(), nil
}

func (s *Selector) roundRobinSelect(service *Service) (*types.Endpoint, map[string]string, error) {
	endpoints := service.Endpoints()
	if len(endpoints) == 0 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}
	s.lastIdx = (s.lastIdx + 1) % uint32(len(endpoints))
	return endpoints[s.lastIdx], s.baseHeaders(), nil
}

func (s *Selector) weightedRandomSelect(service *Service) (*types.Endpoint, map[string]string, error) {
	endpoints := service.Endpoints()
	if len(endpoints) == 0 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}
	attributes := service.AttributesInSelector(s.Name)

	totalWeight := uint32(0)
	for _, attr := range attributes {
		totalWeight += attr.Weight
	}
	r := rand.Uint32() % totalWeight
	totalWeight = 0
	for i, attr := range attributes {
		totalWeight += attr.Weight
		if r < totalWeight {
			return endpoints[i], s.baseHeaders(), nil
		}
	}
	return nil, nil, errcode.ErrNoEndpointAvailable
}

func (s *Selector) weightedRoundRobinSelect(service *Service) (*types.Endpoint, map[string]string, error) {
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
			return endpoints[i], s.baseHeaders(), nil
		}
	}
	s.lastIdx = 0
	if total == 0 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}
	return endpoints[0], s.baseHeaders(), nil
}

// 实现上是选取最大权重（weight = C - cost）
func (s *Selector) leastCostSelect(service *Service) (*types.Endpoint, map[string]string, error) {
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
	return endpoints[idx], s.baseHeaders(), nil
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
		return "", errcode.ErrNoKeyAvailable
	}

	md, exist := metadata.FromOutgoingContext(target.Ctx)
	if !exist {
		return "", errcode.ErrNoValueAvailable
	}
	values := md.Get(s.Key)
	if len(values) == 0 {
		return "", errcode.ErrNoValueAvailable
	}
	return values[0], nil
}

func (s *Selector) hashRingSelect(service *Service, target *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
	// TODO: 实现hash ring算法
	panic("not implemented")
}

// TODO: 增或删除endpoint时，该如何处理？
func (s *Selector) jumpConsistentHashSelect(service *Service, target *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
	if s.Key == "" {
		return nil, nil, errcode.ErrNoKeyAvailable
	}

	endpoints := service.Endpoints()
	if len(endpoints) == 0 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}

	value, err := s.headerValue(target)
	if err != nil {
		return nil, nil, err
	}
	hashed := hashString(value)
	idx := JumpConsistentHash(hashed, int32(len(endpoints)))

	headers := s.baseHeaders()
	headers["yapr-header-value"] = value
	return endpoints[idx], headers, nil
}

func (s *Selector) directSelect(service *Service, target *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
	if s.Key == "" {
		return nil, nil, errcode.ErrNoKeyAvailable
	}

	endpoints := service.EndpointsSet()
	if len(endpoints) == 0 {
		return nil, nil, errcode.ErrNoEndpointAvailable
	}

	value, err := s.headerValue(target)
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
	headers := s.baseHeaders()
	headers["yapr-header-value"] = value
	if _, ok := endpoints[*endpoint]; !ok {
		return endpoint, headers, errcode.ErrEndpointUnavailable
	}
	logger.Debugf("direct select endpoint %v by value %v", endpoint, value)
	return endpoint, headers, nil
}

func (s *Selector) selectByLua(service *Service, target *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
	luaState := lua.NewState()
	defer luaState.Close()
	err := luaState.DoFile(s.Script)
	if err != nil {
		return nil, nil, err
	}
	md, exist := metadata.FromOutgoingContext(target.Ctx)
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
	headers := s.baseHeaders()
	//TODO: 从lua中获取header
	return endpoints[idx], headers, nil
}
