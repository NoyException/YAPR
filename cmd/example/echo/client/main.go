package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"math/rand"
	"noy/router/cmd/example/echo/echopb"
	"noy/router/pkg/yapr/core/sdk/client"
	"noy/router/pkg/yapr/logger"
	"noy/router/pkg/yapr/metrics"
	"runtime"
	"time"
)

var (
	// 统一参数
	configPath  = flag.String("configPath", "yapr.yaml", "config file path")
	id          = flag.Int("id", 1, "server id, must be unique")
	conns       = flag.Int("conns", 1, "concurrent connections")
	concurrency = flag.Int("concurrency", 1000, "goroutines")
	qps         = flag.Int("qps", 100000, "qps")
	dataSize    = flag.String("dataSize", "100B", "data size")

	name string
)

func main() {
	runtime.GOMAXPROCS(4)
	flag.Parse()
	name = fmt.Sprintf("cli-%d", *id)

	logger.ReplaceDefault(logger.NewWithLogFile(logger.InfoLevel, fmt.Sprintf("/.logs/cli-%d.log", *id)))
	defer func() {
		err := logger.Sync()
		if err != nil {
			fmt.Printf("sync logger error: %v", err)
		}
	}()

	time.Sleep(1000 * time.Millisecond)

	sdk := yaprsdk.Init(*configPath)

	var clients []echopb.EchoServiceClient

	for i := 0; i < *conns; i++ {
		conn, err := grpc.NewClient("yapr:///echo",
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithUnaryInterceptor(sdk.GRPCClientInterceptor))
		if err != nil {
			logger.Errorf("grpc new client error: %v", err)
			return
		}
		clients = append(clients, echopb.NewEchoServiceClient(conn))
	}

	uids := make([]string, 100000)
	for i := 0; i < 100000; i++ {
		uid := 1000000 + i
		uids[i] = fmt.Sprintf("%d", uid)
	}

	for i := 0; i < *concurrency; i++ {
		go func() {
			ticker := time.NewTicker(time.Duration(1000000**concurrency / *qps) * time.Microsecond)
			client := clients[i%len(clients)]
			data, err := createData(*dataSize)
			if err != nil {
				logger.Errorf(err.Error())
				return
			}
			for {
				uid := uids[rand.Intn(len(uids))]
				ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-uid", uid))
				ctx2, cancel := context.WithTimeout(ctx, 1*time.Second)
				// 记录用时
				start := time.Now()
				response, err := client.Echo(ctx2, data)
				metrics.ObserveGRPCDuration("echo", time.Since(start).Seconds())
				metrics.IncRequestTotal(name, "echo/Echo")
				if err != nil {
					logger.Errorf(err.Error())
				} else {
					logger.Debugf(response.Message)
				}
				<-ticker.C
				cancel()
			}
		}()
	}

	metrics.Init(8080)
}

func createData(size string) (*echopb.EchoRequest, error) {
	var v string

	switch size {
	case "100B":
		v = string(bytes.Repeat([]byte{'A'}, 100))
	case "1KB":
		v = string(bytes.Repeat([]byte{'A'}, 1024))
	case "10KB":
		v = string(bytes.Repeat([]byte{'A'}, 10*1024))
	case "64KB":
		v = string(bytes.Repeat([]byte{'A'}, 64*1024))
	case "128KB":
		v = string(bytes.Repeat([]byte{'A'}, 128*1024))
	default:
		return nil, errors.New("size should be 100B 1KB 10KB 64KB 128KB")
	}

	return &echopb.EchoRequest{Message: v}, nil
}
