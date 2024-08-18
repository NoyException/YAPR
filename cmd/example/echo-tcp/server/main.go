package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"noy/router/cmd/example/echo-tcp"
	"noy/router/pkg/yapr/core/sdk/server"
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

	go metrics.Init(8080+*id, false)

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

type Response struct {
	id   uint32
	data []byte
	sent func()
}

type Server struct {
	reader    *bufio.Reader
	responses chan *Response
}

func handleConnection(conn net.Conn) *Server {
	server := &Server{
		reader:    bufio.NewReader(conn),
		responses: make(chan *Response, 1000),
	}

	go func() {
		for {
			response := &Response{}

			idBuf := make([]byte, 4)
			_, err := io.ReadFull(server.reader, idBuf)
			if err != nil {
				if errors.Is(err, io.EOF) {
					close(server.responses)
					return
				}
				logger.Errorf("read id failed: %v", err)
				return
			}
			response.id = binary.LittleEndian.Uint32(idBuf)

			// Read the length of the incoming data
			var lengthBuf [4]byte
			_, err = io.ReadFull(server.reader, lengthBuf[:])
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
			_, err = io.ReadFull(server.reader, data)
			if err != nil {
				logger.Errorf("Failed to read data: %v", err)
				return
			}

			d := echo_tcp.UnmarshalData(data)
			response.sent, err = sdk.OnRequestReceived(d.Headers)
			d.Err = err
			// 业务逻辑：当自定义路由没设置时，使用随机路由路由到了这里，于是设置自定义路由永远路由到这里
			if _, ok := d.Headers["set-custom-route"]; ok {
				go func() {
					rawEndpoint := d.Headers["yapr-endpoint"]
					uid := d.Headers["x-uid"]
					endpoint := types.EndpointFromString(rawEndpoint)
					_, _, err := sdk.SetCustomRoute("echo-dir", uid, endpoint, 0, false)
					if err != nil {
						logger.Errorf("SetCustomRoute error: %v", err)
					} else {
						logger.Infof("SetCustomRoute: %s, %s", uid, endpoint)
					}
				}()
			}
			response.data = d.Marshal()
			server.responses <- response
		}
	}()

	go func() {
		for response := range server.responses {
			idBuf := make([]byte, 4)
			binary.LittleEndian.PutUint32(idBuf, response.id)
			_, err := conn.Write(idBuf)
			if err != nil {
				logger.Errorf("Failed to write id: %v", err)
				return
			}
			// Send the length of the response data
			var lengthBuf [4]byte
			binary.LittleEndian.PutUint32(lengthBuf[:], uint32(len(response.data)))
			_, err = conn.Write(lengthBuf[:])
			if err != nil {
				logger.Errorf("Failed to write length: %v", err)
				return
			}
			// Send the response data
			_, err = conn.Write(response.data)
			if err != nil {
				logger.Errorf("Failed to write data: %v", err)
				return
			}
			if response.sent != nil {
				response.sent()
			}
		}
	}()

	return server
}
