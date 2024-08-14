package main

import (
	"context"
	"flag"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"log"
	"net"
	"net/http"
	common "noy/router/cmd/example/echo-http"
	"noy/router/cmd/example/echo/echopb"
	"noy/router/pkg/yapr/core/sdk/server"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
	"noy/router/pkg/yapr/metrics"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
)

var (
	// 统一参数
	configPath   = flag.String("configPath", "yapr.yaml", "config file path")
	id           = flag.Int("id", 1, "server id, must be unique")
	ip           = flag.String("ip", "localhost", "node ip address, must be unique")
	endpointCnt  = flag.Int("endpointCnt", 100, "endpoint count")
	weight       = flag.Int("weight", 1, "default weight for all endpoints")
	gracefulStop = flag.Bool("gracefulStop", true, "enable graceful stop")

	name string
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
	metrics.IncRequestTotal(name, "echo/Echo")
	return &echopb.EchoResponse{Message: name + ": " + request.Message}, nil
}

func main() {
	flag.Parse()
	runtime.GOMAXPROCS(4)

	name = fmt.Sprintf("svr-%d", *id)

	logger.ReplaceDefault(logger.NewWithLogFile(logger.InfoLevel, fmt.Sprintf("/.logs/%s.log", name)))
	defer func() {
		err := logger.Sync()
		if err != nil {
			fmt.Printf("sync logger error: %v", err)
		}
	}()

	go metrics.Init(8080 + *id)
	if *id == 1 {
		go normalServer()
	}

	sdk := yaprsdk.Init(*configPath, name)
	sdk.SetMigrationListener(func(selectorName, headerValue string, from, to *types.Endpoint) {
		logger.Infof("migration: %s, %s, %v, %v", selectorName, headerValue, from, to)
	})

	endpoints := make([]*types.Endpoint, 0)

	http.HandleFunc("/Echo", func(writer http.ResponseWriter, request *http.Request) {
		response := common.Response{}
		defer func() {
			if response.Err == nil {
				writer.WriteHeader(200)
			} else {
				writer.WriteHeader(500)
			}
			bytes, err := response.Marshal()
			if err != nil {
				logger.Errorf("marshal response error: %v", err)
			}
			_, err = writer.Write(bytes)
			if err != nil {
				logger.Errorf("write response error: %v", err)
			}
		}()

		headers := make(map[string]string)
		for k, v := range request.Header {
			headers[strings.ToLower(k)] = v[0]
		}
		err := sdk.OnRequestReceived(headers)

		if err != nil {
			response.Err = err
			return
		}

		response.Payload = make(map[string]string)
		response.Payload["message"] = headers["message"]
	})

	for port := uint32(23333 + (*id-1)*(*endpointCnt)); port < uint32(23333+*id*(*endpointCnt)); port++ {
		addr := fmt.Sprintf("%s:%d", *ip, port)
		go func() {
			err := http.ListenAndServe(addr, nil)
			if err != nil {
				panic(err)
			}
		}()

		endpoint := sdk.NewEndpointWithPort(*ip, port)
		endpoints = append(endpoints, endpoint)
	}

	//logger.Debugf("weight: %d", *weight)
	//
	//for _, endpoint := range endpoints {
	//	time.Sleep(1 * time.Millisecond)
	//	err := sdk.SetEndpointAttribute(endpoint, "echo-rr", &types.AttributeInSelector{
	//		Weight: uint32(*weight),
	//	})
	//	if err != nil {
	//		panic(err)
	//	}
	//}

	if *gracefulStop {
		// 捕获 SIGTERM 信号
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

		go func() {
			<-sigChan
			logger.Infof("Received shutdown signal, performing cleanup...")
			// 执行收尾工作
			err := sdk.UnregisterService("echosvr")
			if err != nil {
				logger.Errorf("unregister service error: %v", err)
			}
			os.Exit(0)
		}()
	}

	for {
		ch, err := sdk.RegisterService("echosvr", endpoints)
		if err != nil {
			panic(err)
		}
		<-ch
		err = sdk.UnregisterService("echosvr")
		if err != nil {
			logger.Errorf("unregister service error: %v", err)
		}
		logger.Warnf("failed to keep alive, retry")
	}

}

func normalServer() {
	port := 23332
	addr := fmt.Sprintf("%s:%d", *ip, port)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	s := grpc.NewServer()

	echopb.RegisterEchoServiceServer(s, &EchoServer{})

	go func() {
		log.Printf("server listening at %v", l.Addr())
		if err := s.Serve(l); err != nil {
			panic(err)
		}
	}()
}