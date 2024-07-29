package yapr

import (
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/metadata"
	"noy/router/pkg/yapr/core"
)

type yaprPicker struct {
	subConns map[string]balancer.SubConn
	router   *core.Router
	port     uint32
}

func NewPicker(subConns map[string]balancer.SubConn, router *core.Router, port uint32) balancer.Picker {
	return &yaprPicker{
		subConns: subConns,
		router:   router,
		port:     port,
	}
}

func (y *yaprPicker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	mt := &core.MatchTarget{
		Port: y.port,
		Uri:  info.FullMethodName,
		Ctx:  info.Ctx,
	}
	selector, err := y.router.Route(mt)
	if err != nil {
		return balancer.PickResult{}, err
	}
	endpoint, err := selector.Select(mt)
	if err != nil {
		return balancer.PickResult{}, err
	}
	return balancer.PickResult{
		SubConn:  y.subConns[endpoint.IP],
		Metadata: metadata.New(selector.Headers),
	}, nil
}
