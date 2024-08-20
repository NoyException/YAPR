package main

import (
	"flag"
	"fmt"
	"math/rand"
	"noy/router/pkg/yapr/core/sdk/client"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
	"noy/router/pkg/yapr/metrics"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"
)

var sdk *yaprsdk.YaprSDK

var (
	// 统一参数
	configPath  = flag.String("configPath", "yapr.yaml", "config file path")
	id          = flag.Int("id", 1, "server id, must be unique")
	concurrency = flag.Int("concurrency", 200, "goroutines")
	totalReq    = flag.Int("totalReq", 20000000, "total request")
	cpus        = flag.Int("cpus", 4, "cpus")

	name string
)

func main() {
	flag.Parse()
	runtime.GOMAXPROCS(*cpus)
	name = fmt.Sprintf("cli-%d", *id)

	logger.ReplaceDefault(logger.NewWithLogFile(logger.InfoLevel, fmt.Sprintf("./.logs/cli-%d.log", *id)))
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

	sdk = yaprsdk.Init(*configPath, false)

	playerCnt := 1000
	uids := make([]string, playerCnt)
	for i := 0; i < playerCnt; i++ {
		uid := 1000000 + i
		uids[i] = fmt.Sprintf("%d", uid)
	}

	go metrics.Init(8080, true)
	wg := &sync.WaitGroup{}

	now := time.Now()
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			//ticker := time.NewTicker(time.Duration(1000000**concurrency / *qps) * time.Microsecond)
			maxJ := *totalReq / *concurrency
			for j := 0; j < maxJ; j++ {
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

	wg.Wait()
	fmt.Printf("total cost: %v, qps: %v\n", time.Since(now), float64(*totalReq)/time.Since(now).Seconds())
}
