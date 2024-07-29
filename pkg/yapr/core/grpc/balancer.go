package yapr

import (
	"errors"
	"fmt"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/resolver"
	"noy/router/pkg/yapr/core"
	"noy/router/pkg/yapr/logger"
	"sync"
)

func init() {
	balancer.Register(&yaprBalancerBuilder{})
}

type yaprBalancerBuilder struct {
}

var _ balancer.Builder = (*yaprBalancerBuilder)(nil)

func (y *yaprBalancerBuilder) Build(cc balancer.ClientConn, opts balancer.BuildOptions) balancer.Balancer {
	bal := &yaprBalancer{
		cc: cc,

		subConns: make(map[string]balancer.SubConn),
		scStates: make(map[balancer.SubConn]connectivity.State),
		csEvltr:  &balancer.ConnectivityStateEvaluator{},
		state:    connectivity.Connecting,
	}
	// Initialize picker to a picker that always returns
	// ErrNoSubConnAvailable, because when state of a SubConn changes, we
	// may call UpdateState with this picker.
	bal.picker = base.NewErrPicker(balancer.ErrNoSubConnAvailable)
	return bal
}

func (y *yaprBalancerBuilder) Name() string {
	return "yapr"
}

type yaprBalancer struct {
	cc   balancer.ClientConn
	rwMu sync.RWMutex

	csEvltr *balancer.ConnectivityStateEvaluator
	state   connectivity.State

	subConns map[string]balancer.SubConn // Address -> SubConn
	scStates map[balancer.SubConn]connectivity.State
	picker   balancer.Picker

	router *core.Router
	port   uint32 // 访问的Router的port

	resolverErr error // the last error reported by the resolver; cleared on successful resolution
	connErr     error // the last connection error; cleared upon leaving TransientFailure
}

var _ balancer.Balancer = (*yaprBalancer)(nil)

func (y *yaprBalancer) UpdateClientConnState(state balancer.ClientConnState) error {
	if state.ResolverState.Attributes == nil {
		y.ResolverError(errors.New("attributes is nil"))
		return balancer.ErrBadResolverState
	}
	y.router = state.ResolverState.Attributes.Value("router").(*core.Router)
	y.port = state.ResolverState.Attributes.Value("port").(uint32)

	if len(state.ResolverState.Endpoints) == 0 {
		y.ResolverError(errors.New("no endpoints"))
		return balancer.ErrBadResolverState
	}

	y.ResolverError(nil)
	newSubConns := make(map[string]balancer.SubConn)
	for _, endpoint := range state.ResolverState.Endpoints {
		service := endpoint.Attributes.Value("service").(*core.Service)
		for _, addr := range endpoint.Addresses {
			key := service.Name + ":" + addr.Addr
			// 跳过已经存在的连接
			if subConn, ok := y.subConns[addr.Addr]; ok {
				newSubConns[key] = subConn
				continue
			}

			var subConn balancer.SubConn

			subConn, err := y.cc.NewSubConn([]resolver.Address{addr}, balancer.NewSubConnOptions{
				HealthCheckEnabled: true,
				StateListener: func(state balancer.SubConnState) {
					y.updateSubConnState(subConn, state)
				},
			})
			if err != nil {
				y.ResolverError(err)
				continue
			}
			newSubConns[key] = subConn
			y.scStates[subConn] = connectivity.Idle
			y.csEvltr.RecordTransition(connectivity.Shutdown, connectivity.Idle)
			subConn.Connect()
		}
	}
	// 关闭已经移除的连接
	for key, subConn := range y.subConns {
		if _, ok := newSubConns[key]; !ok {
			subConn.Shutdown()
		}
	}
	y.subConns = newSubConns
	y.picker = NewPicker(y.subConns, y.router, y.port)
	return nil
}

func (y *yaprBalancer) errorPicker() balancer.Picker {
	return base.NewErrPicker(fmt.Errorf("connection error: %v, resolver error: %v", y.connErr, y.resolverErr))
}

func (y *yaprBalancer) ResolverError(err error) {
	y.resolverErr = err
	if len(y.subConns) == 0 {
		y.state = connectivity.TransientFailure
	}

	if y.state != connectivity.TransientFailure {
		// The picker will not change since the balancer does not currently report an error.
		return
	}
	y.cc.UpdateState(balancer.State{
		ConnectivityState: y.state,
		Picker:            y.errorPicker(),
	})
}

// UpdateSubConnState 弃用的方法（gRPC遗留问题）
func (y *yaprBalancer) UpdateSubConnState(subConn balancer.SubConn, state balancer.SubConnState) {}
func (y *yaprBalancer) updateSubConnState(subConn balancer.SubConn, state balancer.SubConnState) {
	s := state.ConnectivityState

	oldS, ok := y.scStates[subConn]
	if !ok {
		logger.Debugf("Balancer got state changes for an unknown SubConn: %p, %v", subConn, s)
		return
	}

	if oldS == connectivity.TransientFailure &&
		(s == connectivity.Connecting || s == connectivity.Idle) {
		// Once a subconn enters TRANSIENT_FAILURE, ignore subsequent IDLE or
		// CONNECTING transitions to prevent the aggregated state from being
		// always CONNECTING when many backends exist but are all down.
		if s == connectivity.Idle {
			subConn.Connect()
		}
		return
	}
	y.scStates[subConn] = s
	switch s {
	case connectivity.Idle:
		subConn.Connect()
	case connectivity.Shutdown:
		// When an address was removed by resolver, b called Shutdown but kept
		// the subConn's state in scStates. Remove state for this subConn here.
		delete(y.scStates, subConn)
	case connectivity.TransientFailure:
		// Save error to be reported via picker.
		y.connErr = state.ConnectionError
	default:
	}

	y.state = y.csEvltr.RecordTransition(oldS, s)
	picker := y.picker

	if (s == connectivity.Ready) != (oldS == connectivity.Ready) ||
		y.state == connectivity.TransientFailure {
		picker = y.errorPicker()
	}
	y.cc.UpdateState(balancer.State{ConnectivityState: y.state, Picker: picker})
}

// Close is a nop because the balancer doesn't need to call Shutdown for the SubConns.
func (y *yaprBalancer) Close() {
}
