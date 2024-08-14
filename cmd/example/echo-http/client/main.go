package main

import (
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"noy/router/pkg/yapr/core/sdk/client"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
	"noy/router/pkg/yapr/metrics"
	"strconv"
)

var (
	// 统一参数
	configPath = flag.String("configPath", "yapr.yaml", "config file path")
	id         = flag.Int("id", 1, "server id, must be unique")
)

var sdk *yaprsdk.YaprSDK

func main() {
	flag.Parse()

	logger.ReplaceDefault(logger.NewWithLogFile(logger.InfoLevel, fmt.Sprintf("/.logs/cli-%d.log", *id)))
	defer func() {
		err := logger.Sync()
		if err != nil {
			fmt.Printf("sync logger error: %v", err)
		}
	}()

	sdk = yaprsdk.Init(*configPath)

	client := &http.Client{}

	requestSender := func(serviceName string, endpoint *types.Endpoint, port uint32, headers map[string]string) error {
		url := fmt.Sprintf("http://%s:%d%s", endpoint.IP, port, "/Echo")
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			logger.Errorf("create request failed: %v", err)
			return err
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		req.Header.Set("message", "hello world")
		resp, err := client.Do(req)
		if err != nil {
			logger.Errorf("request failed: %v", err)
			return err
		}
		defer resp.Body.Close()
		_, err = io.ReadAll(resp.Body)
		if err != nil {
			logger.Errorf("read response body failed: %v", err)
			return err
		}
		if resp.StatusCode != http.StatusOK {
			logger.Errorf("response status code: %d", resp.StatusCode)
		}
		return nil
	}

	http.HandleFunc("/once", func(w http.ResponseWriter, r *http.Request) {
		Send(requestSender)
	})

	metrics.Init(8080)
}

func Send(requestSender func(serviceName string, endpoint *types.Endpoint, port uint32, headers map[string]string) error) {
	match := &types.MatchTarget{
		URI:  "/Echo",
		Port: 9090,
		Headers: map[string]string{
			"x-uid": strconv.Itoa(rand.Intn(1000000)),
		},
	}
	err := sdk.Invoke("echo", match, requestSender)
	if err != nil {
		logger.Errorf("request failed: %v", err)
	}
}
