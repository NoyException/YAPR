package yaprgrpc

import (
	"context"
	"errors"
	"fmt"
	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/resolver"
	"noy/router/pkg/yapr/core"
	"noy/router/pkg/yapr/logger"
	"noy/router/pkg/yapr/metrics"
	"strconv"
	"strings"
	"time"
)

func init() { // nolint:gochecknoinits
	logger.Infof("init yaprgrpc resolver")
	resolver.Register(&yaprResolverBuilder{})
}

type yaprResolverBuilder struct {
}

var _ resolver.Builder = (*yaprResolverBuilder)(nil)

func (r *yaprResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	//logger.Debugf("target: %+v", target)
	host, port, err := parseTarget(target.Endpoint())
	//logger.Debugf("host: %s, port: %d", host, port)
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

		router, err := core.GetRouter(y.routerName)
		if err == nil && router != nil {
			serviceConfigJSON := fmt.Sprintf(`{"loadBalancingConfig":[{"%s":{}}]}`, "yapr")
			config := y.cc.ParseServiceConfig(serviceConfigJSON)
			err := y.cc.UpdateState(resolver.State{
				Endpoints:     y.Endpoints(router),
				Attributes:    attributes.New("router", router).WithValue("port", y.port),
				ServiceConfig: config,
			})
			if err != nil {
				logger.Warnf("update resolver state error: %v", err)
			}
		}
	}
}

func (y *yaprResolver) Endpoints(router *core.Router) []resolver.Endpoint {
	start := time.Now()
	defer func() {
		metrics.ObserveResolverDuration(time.Since(start).Seconds())
	}()

	visited := make(map[string]struct{})
	var endpoints []resolver.Endpoint
	for _, selector := range router.Selectors() {
		service, err := core.GetService(selector.Service)
		if err != nil {
			logger.Warnf("service %s not found", selector.Service)
			continue
		}
		var addrs []resolver.Address
		for _, endpoint := range service.Endpoints() {
			port := selector.Port
			if endpoint.Port != nil {
				port = *endpoint.Port
			}
			addr := fmt.Sprintf("%s:%d", endpoint.IP, port)
			// 除去重复的地址
			if _, ok := visited[addr]; ok {
				continue
			}
			visited[addr] = struct{}{}
			//logger.Debugf("add addr: %s", addr)

			addrs = append(addrs, resolver.Address{
				Addr: addr,
			})
		}
		//if len(addrs) == 0 {
		//	addrs = append(addrs, resolver.Address{
		//		Addr: fmt.Sprintf("%s:%d", service.Name, selector.Port),
		//	})
		//}
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
		return target, 9090, nil
	}
	port, err := strconv.ParseUint(splits[1], 10, 32)
	if err != nil {
		return "", 0, err
	}
	return splits[0], uint32(port), nil
}
