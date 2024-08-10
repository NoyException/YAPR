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
	"noy/router/pkg/yapr/core/sdk/client"
	"noy/router/pkg/yapr/logger"
	"noy/router/pkg/yapr/metrics"
	"time"
)

var (
	// 统一参数
	configPath = flag.String("configPath", "yapr.yaml", "config file path")
	name       = flag.String("name", "unnamed-node", "node name, must be unique")
	//httpAddr   = flag.String("httpAddr", "localhost:23334", "node http address, must be unique")
)

func main() {
	flag.Parse()

	logger.ReplaceDefault(logger.NewWithLogFile(logger.InfoLevel, fmt.Sprintf("/.logs/%s.log", *name)))
	defer func() {
		err := logger.Sync()
		if err != nil {
			fmt.Printf("sync logger error: %v", err)
		}
	}()

	time.Sleep(1000 * time.Millisecond)

	sdk := yaprsdk.Init(*configPath)

	go metrics.Init()

	conn, err := grpc.NewClient("yapr:///echo",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(sdk.GRPCClientInterceptor))
	if err != nil {
		log.Fatalf(err.Error())
	}
	client := echopb.NewEchoServiceClient(conn)
	uids := make([]string, 100000)
	for i := 0; i < 100000; i++ {
		uid := 1000000 + i
		uids[i] = fmt.Sprintf("%d", uid)
	}
	for i := 0; i < 100; i++ {
		go func() {
			ticker := time.NewTicker(10 * time.Millisecond)
			for {
				time.Sleep(time.Duration(rand.IntN(10)) * time.Millisecond)
				uid := *name + "_" + uids[rand.IntN(len(uids))]
				ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-uid", uid))
				ctx2, cancel := context.WithTimeout(ctx, 1*time.Second)

				// 记录用时
				start := time.Now()
				response, err := client.Echo(ctx2, &echopb.EchoRequest{Message: "Hello world from user " + uid})
				metrics.ObserveGRPCDuration("echo", time.Since(start).Seconds())
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

	select {}
	//for range ticker.C {
	//
	//	// 测试自定义路由
	//	endpoints := sdk.GetEndpoints("echo-dir")
	//	idx := rand.IntN(len(endpoints))
	//	var ep *types.Endpoint
	//	if len(endpoints) > 0 {
	//		for endpoint, _ := range endpoints {
	//			ep = &endpoint
	//			idx--
	//			if idx < 0 {
	//				break
	//			}
	//		}
	//	}
	//	if ep != nil {
	//		success, old, err := sdk.SetCustomRoute("echo-dir", uid, ep, 0, true)
	//		if err != nil {
	//			logger.Errorf("set custom route error: %v", err)
	//		}
	//		logger.Debugf("set custom route to %v success: %v, old: %v", ep, success, old)
	//	}
	//}
}
