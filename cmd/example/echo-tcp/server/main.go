package main

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"net"
	echo_tcp "noy/router/cmd/example/echo-tcp"
	yaprsdk "noy/router/pkg/yapr/core/sdk/server"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
	"noy/router/pkg/yapr/metrics"
	"os"
	"os/signal"
	"syscall"
)

var (
	// 统一参数
	configPath   = flag.String("configPath", "yapr.yaml", "config file path")
	id           = flag.Int("id", 1, "server id, must be unique")
	ip           = flag.String("ip", "localhost", "node ip address, must be unique")
	endpointCnt  = flag.Int("endpointCnt", 1000, "endpoint count")
	weight       = flag.Int("weight", 1, "default weight for all endpoints")
	gracefulStop = flag.Bool("gracefulStop", true, "enable graceful stop")

	name string

	sdk *yaprsdk.YaprSDK
)

func main() {
	flag.Parse()

	name = fmt.Sprintf("svr-%d", *id)

	logger.ReplaceDefault(logger.NewWithLogFile(logger.InfoLevel, fmt.Sprintf("/.logs/%s.log", name)))
	defer func() {
		err := logger.Sync()
		if err != nil {
			fmt.Printf("sync logger error: %v", err)
		}
	}()

	sdk = yaprsdk.Init(*configPath, name)
	sdk.SetMigrationListener(func(selectorName, headerValue string, from, to *types.Endpoint) {
		logger.Infof("migration: %s, %s, %v, %v", selectorName, headerValue, from, to)
	})

	go metrics.Init(8080 + *id)

	endpoints := make([]*types.Endpoint, 0)

	for port := uint32(23333 + (*id-1)*(*endpointCnt)); port < uint32(23333+*id*(*endpointCnt)); port++ {
		addr := fmt.Sprintf("%s:%d", *ip, port)

		// Listen on TCP port 8080
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			logger.Fatalf("Failed to listen on port 8080: %v", err)
		}
		go func() {
			for {
				// Accept a new connection
				conn, err := listener.Accept()
				if err != nil {
					logger.Errorf("Failed to accept connection: %v", err)
					continue
				}

				// Handle the connection in a new goroutine
				go handleConnection(conn)
			}
		}()
		defer listener.Close()

		endpoint := sdk.NewEndpointWithPort(*ip, port)
		endpoints = append(endpoints, endpoint)
	}

	if *gracefulStop {
		// 捕获 SIGTERM 信号
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

		go func() {
			<-sigChan
			logger.Infof("Received shutdown signal, performing cleanup...")
			// 执行收尾工作
			err := sdk.UnregisterService("echosvr")
			if err != nil {
				logger.Errorf("unregister service error: %v", err)
			}
			os.Exit(0)
		}()
	}

	for {
		ch, err := sdk.RegisterService("echosvr", endpoints)
		if err != nil {
			panic(err)
		}
		<-ch
		err = sdk.UnregisterService("echosvr")
		if err != nil {
			logger.Errorf("unregister service error: %v", err)
		}
		logger.Warnf("failed to keep alive, retry")
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	for {
		// Read the length of the incoming data
		var lengthBuf [4]byte
		_, err := io.ReadFull(reader, lengthBuf[:])
		if err != nil {
			if err == io.EOF {
				//logger.Warnf("Connection closed")
				return
			}
			logger.Errorf("Failed to read length: %v", err)
			return
		}
		length := int32(binary.LittleEndian.Uint32(lengthBuf[:]))

		if length > 65536 {
			logger.Errorf("length too large: %d", length)
			return
		}
		// Read the data based on the length
		data := make([]byte, length)
		_, err = io.ReadFull(reader, data)
		if err != nil {
			logger.Errorf("Failed to read data: %v", err)
			return
		}

		d := echo_tcp.UnmarshalData(data)
		err = sdk.OnRequestReceived(d.Headers)
		d.Err = err
		response := d.Marshal()

		// Send the length of the response data
		binary.LittleEndian.PutUint32(lengthBuf[:], uint32(len(response)))
		_, err = conn.Write(lengthBuf[:])
		if err != nil {
			logger.Errorf("Failed to write length: %v", err)
			return
		}

		// Send the response data
		_, err = conn.Write(response)
		if err != nil {
			logger.Errorf("Failed to write data: %v", err)
			return
		}
	}
}
