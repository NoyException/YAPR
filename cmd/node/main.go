package main

import (
	"flag"
	"fmt"
	"noy/router/pkg/router/config"
	"noy/router/pkg/router/core/node"
	"noy/router/pkg/router/logger"
	"noy/router/pkg/router/store"
)

var (
	// 统一参数
	configPath = flag.String("configPath", "config.yaml", "config file path")
	name       = flag.String("name", "unnamed-node", "node name, must be unique")
	addr       = flag.String("addr", "localhost:23333", "node address, must be unique")
	httpAddr   = flag.String("httpAddr", "localhost:23334", "node http address, must be unique")
)

func main() {
	flag.Parse()

	// 加载配置
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		panic(err)
	}

	logger.ReplaceDefault(logger.NewWithLogFile(logger.DebugLevel, fmt.Sprintf(".logs/%s.log", *name)))
	defer func() {
		err := logger.Sync()
		if err != nil {
			fmt.Printf("sync logger error: %v", err)
		}
	}()

	var s store.Store

	// 初始化存储
	if s, err = store.Init(cfg.Store); err != nil {
		panic(err)
	}

	n := node.NewNode(*name, *addr, *httpAddr, s, cfg)
	n.Run()
}
