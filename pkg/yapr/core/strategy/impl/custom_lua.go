package builtin

import (
	lua "github.com/yuin/gopher-lua"
	"google.golang.org/grpc/metadata"
	"noy/router/pkg/yapr/core/errcode"
	"noy/router/pkg/yapr/core/strategy"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
)

type CustomLuaStrategyBuilder struct{}

func (b *CustomLuaStrategyBuilder) Build(s *types.Selector) (strategy.Strategy, error) {
	return &CustomLuaStrategy{s.Script, nil}, nil
}

type CustomLuaStrategy struct {
	script    string
	endpoints []*types.Endpoint
}

func (r *CustomLuaStrategy) Select(match *types.MatchTarget) (*types.Endpoint, map[string]string, error) {
	luaState := lua.NewState()
	defer luaState.Close()
	err := luaState.DoFile(r.script)
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
	// 创建lua数组
	luaEndpoints := luaState.NewTable()
	for _, endpoint := range r.endpoints {
		luaEndpoints.Append(lua.LString(endpoint.String()))
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
	if idx < 0 || idx >= len(r.endpoints) {
		return nil, nil, errcode.ErrLuaIndexOutOfRange
	}
	//TODO: 从lua中获取header
	return r.endpoints[idx], nil, nil
}

func (r *CustomLuaStrategy) Update(endpoints map[types.Endpoint]*types.Attribute) {
	r.endpoints = make([]*types.Endpoint, 0, len(endpoints))
	for endpoint := range endpoints {
		r.endpoints = append(r.endpoints, &endpoint)
	}
}
