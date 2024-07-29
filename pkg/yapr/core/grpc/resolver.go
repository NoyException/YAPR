package yapr

import (
	"context"
	"errors"
	"fmt"
	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/resolver"
	"noy/router/pkg/yapr/core"
	"noy/router/pkg/yapr/logger"
	"strconv"
	"strings"
	"time"
)

func init() { // nolint:gochecknoinits
	resolver.Register(&yaprResolverBuilder{})
}

type yaprResolverBuilder struct {
}

var _ resolver.Builder = (*yaprResolverBuilder)(nil)

func (r *yaprResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	host, port, err := parseTarget(target.Endpoint())
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	yr := &yaprResolver{
		ctx:        ctx,
		cancel:     cancel,
		cc:         cc,
		routerName: host,
		port:       port,
		rn:         make(chan struct{}, 1),
	}

	go yr.watcher()

	yr.ResolveNow(resolver.ResolveNowOptions{})
	return yr, nil
}

func (r *yaprResolverBuilder) Scheme() string {
	return "yapr"
}

type yaprResolver struct {
	ctx        context.Context
	cancel     context.CancelFunc
	cc         resolver.ClientConn
	routerName string
	port       uint32
	rn         chan struct{}
}

var _ resolver.Resolver = (*yaprResolver)(nil)

func (y *yaprResolver) ResolveNow(options resolver.ResolveNowOptions) {
	select {
	case y.rn <- struct{}{}:
	default:
	}
}

func (y *yaprResolver) Close() {
	y.cancel()
}

func (y *yaprResolver) watcher() {
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-y.ctx.Done():
			return
		case <-y.rn:
		case <-ticker.C:
		}

		router := core.MustRouter(y.routerName)
		if router != nil {
			err := y.cc.UpdateState(resolver.State{
				Endpoints:  y.Endpoints(router),
				Attributes: attributes.New("router", router).WithValue("port", y.port),
			})
			if err != nil {
				logger.Warnf("update resolver state error: %v", err)
			}
		}
	}
}

func (y *yaprResolver) Endpoints(router *core.Router) []resolver.Endpoint {
	var endpoints []resolver.Endpoint
	for _, selector := range router.Selectors() {
		service, ok := router.ServiceByName[selector.Service]
		if !ok {
			logger.Warnf("service %s not found", selector.Service)
			continue
		}
		var addrs []resolver.Address
		for endpoint, _ := range service.AttrMap {
			addrs = append(addrs, resolver.Address{
				Addr: fmt.Sprintf("%s:%d", endpoint.IP, selector.Port),
			})
		}
		if len(addrs) == 0 {
			addrs = append(addrs, resolver.Address{
				Addr: fmt.Sprintf("%s:%d", service.Name, selector.Port),
			})
		}
		endpoints = append(endpoints, resolver.Endpoint{
			Addresses:  addrs,
			Attributes: attributes.New("service", service),
		})
	}
	return endpoints
}

func parseTarget(target string) (string, uint32, error) {
	splits := strings.Split(target, ":")
	if len(splits) > 2 {
		return "", 0, errors.New("error format routerName")
	}
	if len(splits) == 1 {
		return target, 6100, nil
	}
	port, err := strconv.ParseUint(splits[1], 10, 32)
	if err != nil {
		return "", 0, err
	}
	return splits[0], uint32(port), nil
}
