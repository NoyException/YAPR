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
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

var (
	// 统一参数
	configPath  = flag.String("configPath", "yapr.yaml", "config file path")
	id          = flag.Int("id", 1, "server id, must be unique")
	conns       = flag.Int("conns", 8, "concurrent connections")
	concurrency = flag.Int("concurrency", 1, "goroutines")
	totalReq    = flag.Int("totalReq", 10000000, "total request")
	dataSize    = flag.String("dataSize", "100B", "data size")
	useYapr     = flag.Bool("useYapr", true, "use yapr")
	interval    = flag.Int("interval", 1000, "interval")
	cpus        = flag.Int("cpus", 4, "cpus")

	name string
)

func main() {
	flag.Parse()
	runtime.GOMAXPROCS(*cpus)
	name = fmt.Sprintf("cli-%d", *id)

	logger.ReplaceDefault(logger.NewWithLogFile(logger.DebugLevel, fmt.Sprintf("./.logs/cli-%d.log", *id)))
	defer func() {
		err := logger.Sync()
		if err != nil {
			fmt.Printf("sync logger error: %v", err)
		}
	}()

	// 捕获 SIGTERM 信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigChan
		logger.Infof("Received shutdown signal, performing cleanup...")
		os.Exit(0)
	}()

	sdk := yaprsdk.Init(*configPath, true)

	var clients []echopb.EchoServiceClient
	target := "yapr:///echo"
	if !*useYapr {
		target = "9.134.60.182:23332"
	}
	for i := 0; i < *conns; i++ {
		conn, err := grpc.NewClient(target,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithUnaryInterceptor(sdk.GRPCClientInterceptor))
		if err != nil {
			logger.Errorf("grpc new client error: %v", err)
			return
		}
		clients = append(clients, echopb.NewEchoServiceClient(conn))
	}

	time.Sleep(1 * time.Second)

	uids := make([]string, 5)
	for i := 0; i < len(uids); i++ {
		uid := 1000000 + i
		uids[i] = fmt.Sprintf("%d", uid)
	}

	for i := 0; i < *concurrency; i++ {
		go func() {
			client := clients[i%len(clients)]
			data, err := createData(*dataSize)
			if err != nil {
				logger.Errorf(err.Error())
				return
			}
			maxJ := *totalReq / *concurrency
			for j := 0; j < maxJ; j++ {
				uid := uids[rand.Intn(len(uids))]
				ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-uid", uid))
				ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
				// 记录用时
				start := time.Now()
				// 触发RPC调用
				response, err := client.Echo(ctx2, data)

				metrics.ObserveRequestDuration("echo", time.Since(start).Seconds())
				metrics.IncRequestTotal(name, "echo/Echo")
				if err != nil {
					logger.Errorf(err.Error())
				} else {
					logger.Debugf("UID-%s %s", uid, response.Message)
				}
				time.Sleep(time.Duration(*interval) * time.Millisecond)
				cancel()
			}
		}()
	}

	metrics.Init(8080, false)
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
