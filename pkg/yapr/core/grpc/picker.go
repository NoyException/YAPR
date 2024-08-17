package yaprgrpc

import (
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/metadata"
	"noy/router/pkg/yapr/core"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
	"strconv"
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
	headers := make(map[string]string)
	md, exists := metadata.FromOutgoingContext(info.Ctx)
	if exists {
		for k, v := range md {
			headers[k] = v[0]
		}
	}
	mt := &types.MatchTarget{
		Port:    y.port,
		URI:     info.FullMethodName,
		Headers: headers,
		Timeout: info.Ctx.Done(),
	}
	_, endpoint, port, meta, err := y.router.Route(mt)
	if err != nil {
		return balancer.PickResult{}, err
	}
	key := endpoint.IP + ":" + strconv.FormatUint(uint64(port), 10)
	subConn, ok := y.subConns[key]
	if !ok {
		logger.Warnf("subConn not found %v", key)
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}
	return balancer.PickResult{
		SubConn:  subConn,
		Metadata: metadata.New(meta),
	}, nil
}
