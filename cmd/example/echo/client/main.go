package main

import (
	"context"
	"flag"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"noy/router/cmd/example/echo/echopb"
	"noy/router/pkg/yapr/core/sdk"
	"noy/router/pkg/yapr/logger"
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

	yaprsdk.Init(*configPath)
	time.Sleep(10 * time.Millisecond)

	conn, err := grpc.NewClient("yapr:///echo", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf(err.Error())
	}
	client := echopb.NewEchoServiceClient(conn)
	ticker := time.NewTicker(2 * time.Second)

	for range ticker.C {
		response, err := client.Echo(context.Background(), &echopb.EchoRequest{Message: "Hello world"})
		if err != nil {
			log.Fatalf(err.Error())
		}
		log.Println(response.Message)
	}
}
