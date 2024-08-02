package main

import (
	"context"
	"flag"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"log"
	"math/rand/v2"
	"noy/router/cmd/example/echo/echopb"
	"noy/router/pkg/yapr/core/sdk"
	"noy/router/pkg/yapr/logger"
	"noy/router/pkg/yapr/metrics"
	"time"
)

var (
	// 统一参数
	configPath = flag.String("configPath", "yapr.yaml", "config file path")
	name       = flag.String("name", "unnamed-node", "node name, must be unique")
	addr       = flag.String("addr", "localhost:23333", "node address, must be unique")
	//httpAddr   = flag.String("httpAddr", "localhost:23334", "node http address, must be unique")
)

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
	yaprsdk.Init(*configPath)
	time.Sleep(10 * time.Millisecond)

	conn, err := grpc.NewClient("yapr:///echo", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf(err.Error())
	}
	client := echopb.NewEchoServiceClient(conn)
	ticker := time.NewTicker(2 * time.Second)
	uids := []string{"10001", "10002", "10003", "10004", "10005"}
	for range ticker.C {
		uid := *name + "_" + uids[rand.IntN(len(uids))]
		ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-uid", uid))
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)

		// 记录用时
		start := time.Now()
		response, err := client.Echo(ctx, &echopb.EchoRequest{Message: "Hello world from user " + uid})
		metrics.ObserveGRPCDuration("echo", time.Since(start).Seconds())

		cancel()
		if err != nil {
			logger.Errorf(err.Error())
		} else {
			log.Println(response.Message)
		}
	}
}
