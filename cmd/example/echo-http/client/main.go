package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"noy/router/pkg/yapr/core/sdk/client"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
	"noy/router/pkg/yapr/metrics"
	"time"
)

var (
	// 统一参数
	configPath  = flag.String("configPath", "yapr.yaml", "config file path")
	id          = flag.Int("id", 1, "server id, must be unique")
	concurrency = flag.Int("concurrency", 1000, "goroutines")
	qps         = flag.Int("qps", 100000, "qps")
	dataSize    = flag.String("dataSize", "100B", "data size")
	useYapr     = flag.Bool("useYapr", true, "use yapr")

	name string
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

	requestSender := func(serviceName string, endpoint *types.Endpoint, port uint32, headers map[string]string, data []byte) error {
		url := fmt.Sprintf("http://%s:%d%s", endpoint.IP, port, "/Echo")
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			logger.Errorf("create request failed: %v", err)
			return err
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}
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

	uids := make([]string, 100000)
	for i := 0; i < 100000; i++ {
		uid := 1000000 + i
		uids[i] = fmt.Sprintf("%d", uid)
	}

	for i := 0; i < *concurrency; i++ {
		go func() {
			ticker := time.NewTicker(time.Duration(1000000**concurrency / *qps) * time.Microsecond)
			data, err := createData(*dataSize)
			if err != nil {
				logger.Errorf(err.Error())
				return
			}
			for {
				uid := uids[rand.Intn(len(uids))]
				// 记录用时
				start := time.Now()
				Send(uid, data, requestSender)
				metrics.ObserveRequestDuration("echo", time.Since(start).Seconds())
				metrics.IncRequestTotal(name, "echo/Echo")
				if err != nil {
					logger.Errorf(err.Error())
				}
				<-ticker.C
			}
		}()
	}
	metrics.Init(8080)
}

func Send(uid string, data []byte, requestSender func(serviceName string, endpoint *types.Endpoint, port uint32, headers map[string]string, data []byte) error) {
	match := &types.MatchTarget{
		URI:  "/Echo",
		Port: 9090,
		Headers: map[string]string{
			"x-uid": uid,
		},
	}
	err := sdk.Invoke("echo", match, func(serviceName string, endpoint *types.Endpoint, port uint32, headers map[string]string) error {
		return requestSender(serviceName, endpoint, port, headers, data)
	})
	if err != nil {
		logger.Errorf("request failed: %v", err)
	}
}

func createData(size string) ([]byte, error) {
	var v []byte

	switch size {
	case "100B":
		v = bytes.Repeat([]byte{'A'}, 100)
	case "1KB":
		v = bytes.Repeat([]byte{'A'}, 1024)
	case "10KB":
		v = bytes.Repeat([]byte{'A'}, 10*1024)
	case "64KB":
		v = bytes.Repeat([]byte{'A'}, 64*1024)
	case "128KB":
		v = bytes.Repeat([]byte{'A'}, 128*1024)
	default:
		return nil, errors.New("size should be 100B 1KB 10KB 64KB 128KB")
	}

	return v, nil
}
