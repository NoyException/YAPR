package main

import (
	"fmt"
	"noy/router/pkg/router/core"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	sc := core.NewSidecar(nil, 25001)
	defer func() {
		err := sc.Stop()
		if err != nil {
			panic(err)
		}
	}()
	// 创建一个信号通道，用于接收系统信号
	sigChan := make(chan os.Signal, 1)
	// 监听SIGINT和SIGTERM信号
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	// 使用goroutine来处理信号
	go func() {
		sig := <-sigChan
		fmt.Printf("接收到信号: %v\n", sig)
		// 当接收到中断信号时，通过panic来触发defer的执行
		if sig == syscall.SIGINT || sig == syscall.SIGTERM {
			panic("程序被中断")
		}
	}()
	err := sc.Start()
	if err != nil {
		panic(err)
	}
}
