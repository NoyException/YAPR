package main

import (
	"context"
	"flag"
	"fmt"
	"google.golang.org/grpc"
	"log"
	"net"
	"noy/router/cmd/example/echo/echopb"
	"noy/router/pkg/yapr/core"
	yaprsdk "noy/router/pkg/yapr/core/sdk"
	"noy/router/pkg/yapr/logger"
	"strings"
	"time"
)

var (
	// 统一参数
	configPath = flag.String("configPath", "yapr.yaml", "config file path")
	name       = flag.String("name", "unnamed-node", "node name, must be unique")
	addr       = flag.String("addr", "localhost:23333", "node address, must be unique")
	//httpAddr   = flag.String("httpAddr", "localhost:23334", "node http address, must be unique")
)

type EchoServer struct {
	echopb.UnimplementedEchoServiceServer
}

func (e *EchoServer) Echo(ctx context.Context, request *echopb.EchoRequest) (*echopb.EchoResponse, error) {
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

	l, err := net.Listen("tcp", *addr)
	if err != nil {
		panic(err)
	}
	s := grpc.NewServer()
	defer s.Stop()
	echopb.RegisterEchoServiceServer(s, &EchoServer{})

	go func() {
		time.Sleep(10 * time.Millisecond)
		yaprsdk.Init(*configPath)
		err = yaprsdk.RegisterService("echosvr", []*core.Endpoint{{strings.Split(*addr, ":")[0]}})
		if err != nil {
			panic(err)
		}
	}()

	log.Printf("server listening at %v", l.Addr())
	if err := s.Serve(l); err != nil {
		panic(err)
	}

}
