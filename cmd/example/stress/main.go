package main

import (
	"flag"
	"fmt"
	"math/rand"
	"noy/router/pkg/yapr/core/sdk/client"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
	"noy/router/pkg/yapr/metrics"
	"runtime"
	"time"
)

var sdk *yaprsdk.YaprSDK

var (
	// 统一参数
	configPath  = flag.String("configPath", "yapr.yaml", "config file path")
	id          = flag.Int("id", 1, "server id, must be unique")
	concurrency = flag.Int("concurrency", 200, "goroutines")
	qps         = flag.Int("qps", 100000, "qps")

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

	sdk = yaprsdk.Init(*configPath)

	playerCnt := 1000
	uids := make([]string, playerCnt)
	for i := 0; i < playerCnt; i++ {
		uid := 1000000 + i
		uids[i] = fmt.Sprintf("%d", uid)
	}

	for i := 0; i < *concurrency; i++ {
		go func() {
			//ticker := time.NewTicker(time.Duration(1000000**concurrency / *qps) * time.Microsecond)
			for {
				uid := uids[rand.Intn(len(uids))]
				// 记录用时
				start := time.Now()
				match := &types.MatchTarget{
					URI:  "/Echo",
					Port: 9090,
					Headers: map[string]string{
						"x-uid": uid,
					},
				}
				err := sdk.Invoke("echo", match, func(serviceName string, endpoint *types.Endpoint, port uint32, headers map[string]string) error {
					return nil
				})
				metrics.ObserveRequestDuration("echo", time.Since(start).Seconds())
				metrics.IncRequestTotal(name, "echo/Echo")
				if err != nil {
					logger.Errorf(err.Error())
				}
				//<-ticker.C
			}
		}()
	}

	metrics.Init(8080)
}
