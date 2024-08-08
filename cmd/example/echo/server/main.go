package main

import (
	"context"
	"flag"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"log"
	"net"
	"noy/router/cmd/example/echo/echopb"
	"noy/router/pkg/yapr/core/sdk/server"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
	"noy/router/pkg/yapr/metrics"
	"strconv"
	"strings"
	"time"
)

var (
	// 统一参数
	configPath = flag.String("configPath", "yapr.yaml", "config file path")
	name       = flag.String("name", "unnamed", "node name, must be unique")
	addr       = flag.String("addr", "localhost:23333", "node address, must be unique")
	weight     = flag.String("weight", "1", "default weight for all endpoints")
	//httpAddr   = flag.String("httpAddr", "localhost:23334", "node http address, must be unique")
)

type EchoServer struct {
	echopb.UnimplementedEchoServiceServer

	Endpoint *types.Endpoint
}

func (e *EchoServer) Echo(ctx context.Context, request *echopb.EchoRequest) (*echopb.EchoResponse, error) {
	md, exist := metadata.FromIncomingContext(ctx)
	if exist {
		values := md.Get("x-uid")
		if len(values) > 0 {
			uid := values[0]
			logger.Debugf("uid: %s", uid)
			//success, old, err := yaprsdk.MustInstance().SetCustomRoute("echo-dir", uid, e.Endpoint, 0, false)
			//if err != nil {
			//	logger.Errorf("set custom route error: %v", err)
			//}
			//logger.Debugf("set custom route success: %v, old: %v", success, old)
		}
	}
	return &echopb.EchoResponse{Message: *name + ": " + request.Message}, nil
}

func main() {
	flag.Parse()

	logger.ReplaceDefault(logger.NewWithLogFile(logger.DebugLevel, fmt.Sprintf(".logs/%s.log", *name)))
	defer func() {
		err := logger.Sync()
		if err != nil {
			fmt.Printf("sync logger error: %v", err)
		}
	}()

	go metrics.Init()
	l, err := net.Listen("tcp", *addr)
	if err != nil {
		panic(err)
	}

	sdk := yaprsdk.Init(*configPath, *name)
	sdk.SetMigrationListener(func(selectorName, headerValue string, from, to *types.Endpoint) {
		logger.Infof("migration: %s, %s, %v, %v", selectorName, headerValue, from, to)
	})

	s := grpc.NewServer(grpc.UnaryInterceptor(sdk.GRPCServerInterceptor))
	defer s.Stop()
	endpoint := sdk.NewEndpoint(strings.Split(*addr, ":")[0])
	echopb.RegisterEchoServiceServer(s, &EchoServer{
		Endpoint: endpoint,
	})

	go func() {
		time.Sleep(1 * time.Millisecond)
		w, err := strconv.ParseUint(*weight, 10, 32)
		if err != nil {
			logger.Warnf("convert weight error: %v", err)
			w = 1
		}
		err = sdk.SetEndpointAttribute(endpoint, "s1", &types.Attribute{
			Weight: uint32(w),
		})
		if err != nil {
			panic(err)
		}
		for {
			ch, err := sdk.RegisterService("echosvr", []*types.Endpoint{endpoint})
			if err != nil {
				panic(err)
			}
			<-ch
		}
	}()

	//// 捕获 SIGTERM 信号
	//sigChan := make(chan os.Signal, 1)
	//signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	//
	//go func() {
	//	<-sigChan
	//	logger.Infof("Received shutdown signal, performing cleanup...")
	//	// 执行收尾工作
	//	err := sdk.UnregisterService("echosvr")
	//	if err != nil {
	//		logger.Errorf("unregister service error: %v", err)
	//	}
	//	s.Stop()
	//	os.Exit(0)
	//}()

	log.Printf("server listening at %v", l.Addr())
	if err := s.Serve(l); err != nil {
		panic(err)
	}
}
