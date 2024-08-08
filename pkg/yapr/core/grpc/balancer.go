package yaprgrpc

import (
	"errors"
	"fmt"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/resolver"
	"noy/router/pkg/yapr/core"
	"noy/router/pkg/yapr/core/errcode"
	"noy/router/pkg/yapr/logger"
)

func init() { // nolint:gochecknoinits
	logger.Infof("init yaprgrpc balancer")
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
	cc balancer.ClientConn

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

func (y *yaprBalancer) ResolverError(err error) {
	y.resolverErr = err
	if len(y.subConns) == 0 {
		y.state = connectivity.TransientFailure
	}

	if y.state != connectivity.TransientFailure {
		// The picker will not change since the balancer does not currently report an error.
		return
	}

	y.regeneratePicker()
	y.cc.UpdateState(balancer.State{ConnectivityState: y.state, Picker: y.picker})
}

func (y *yaprBalancer) UpdateClientConnState(state balancer.ClientConnState) error {
	y.resolverErr = nil

	if state.ResolverState.Attributes == nil {
		logger.Error("attributes is nil")
		y.ResolverError(errors.New("attributes is nil"))
		return balancer.ErrBadResolverState
	}
	y.router = state.ResolverState.Attributes.Value("router").(*core.Router)
	y.port = state.ResolverState.Attributes.Value("port").(uint32)
	if len(state.ResolverState.Endpoints) == 0 {
		logger.Error("no endpoints")
		y.ResolverError(errors.New("no endpoints"))
		return balancer.ErrBadResolverState
	}

	newSubConns := make(map[string]balancer.SubConn)
	for _, endpoint := range state.ResolverState.Endpoints {
		//service := endpoint.Attributes.Value("service").(*core.Service)
		for _, addr := range endpoint.Addresses {
			key := addr.Addr
			// 跳过已经存在的连接
			if subConn, ok := y.subConns[key]; ok {
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
				logger.Warnf("failed to create new SubConn: %v", err)
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
	if len(y.subConns) == 0 {
		y.ResolverError(errcode.ErrNoEndpointAvailable)
		return balancer.ErrBadResolverState
	}

	y.regeneratePicker()
	y.cc.UpdateState(balancer.State{ConnectivityState: y.state, Picker: y.picker})
	return nil
}

// mergeErrors builds an error from the last connection error and the last
// resolver error.  Must only be called if b.state is TransientFailure.
func (y *yaprBalancer) mergeErrors() error {
	// connErr must always be non-nil unless there are no SubConns, in which
	// case resolverErr must be non-nil.
	if y.connErr == nil {
		return fmt.Errorf("last resolver error: %v", y.resolverErr)
	}
	if y.resolverErr == nil {
		return fmt.Errorf("last connection error: %v", y.connErr)
	}
	return fmt.Errorf("last connection error: %v; last resolver error: %v", y.connErr, y.resolverErr)
}

func (y *yaprBalancer) regeneratePicker() {
	if y.state == connectivity.TransientFailure {
		y.picker = base.NewErrPicker(y.mergeErrors())
		return
	}
	//TODO: 过滤掉不可用的连接
	y.picker = NewPicker(y.subConns, y.router, y.port)
}

// UpdateSubConnState 弃用的方法（gRPC遗留问题）
func (y *yaprBalancer) UpdateSubConnState(subConn balancer.SubConn, state balancer.SubConnState) {
	logger.Errorf("UpdateSubConnState(%v, %+v) called unexpectedly", subConn, state)
}

func (y *yaprBalancer) updateSubConnState(subConn balancer.SubConn, state balancer.SubConnState) {
	s := state.ConnectivityState
	oldS, ok := y.scStates[subConn]
	if !ok {
		logger.Debugf("Balancer got state changes for an unknown SubConn: %p, %v", subConn, s)
		return
	}
	logger.Infof("handle SubConn state change: %p, from %v to %v", subConn, oldS, s)

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

	if (s == connectivity.Ready) != (oldS == connectivity.Ready) ||
		y.state == connectivity.TransientFailure {
		y.regeneratePicker()
	}
	y.cc.UpdateState(balancer.State{ConnectivityState: y.state, Picker: y.picker})
}

// Close is a nop because the balancer doesn't need to call Shutdown for the SubConns.
func (y *yaprBalancer) Close() {
}

func (y *yaprBalancer) ExitIdle() {
}
