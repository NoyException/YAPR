package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net"
	echo_tcp "noy/router/cmd/example/echo-tcp"
	yaprsdk "noy/router/pkg/yapr/core/sdk/client"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
	"noy/router/pkg/yapr/metrics"
	"runtime"
	"sync"
	"time"
)

var sdk *yaprsdk.YaprSDK

var (
	// 统一参数
	configPath  = flag.String("configPath", "yapr.yaml", "config file path")
	id          = flag.Int("id", 1, "server id, must be unique")
	conns       = flag.Int("conns", 8, "concurrent connections")
	concurrency = flag.Int("concurrency", 200, "goroutines")
	qps         = flag.Int("qps", 100000, "qps")
	dataSize    = flag.String("dataSize", "100B", "data size")
	useYapr     = flag.Bool("useYapr", true, "use yapr")

	name string

	clientMap = new(sync.Map)
)

type ClientPool struct {
	clients []*Client
}

func NewClientPool(addr string, size int) (*ClientPool, error) {
	pool := &ClientPool{
		clients: make([]*Client, size),
	}
	for i := 0; i < size; i++ {
		client, err := NewClient(addr)
		if err != nil {
			return nil, err
		}
		pool.clients[i] = client
	}
	return pool, nil
}

func (p *ClientPool) GetAvailableClient() (*Client, error) {
	for _, client := range p.clients {
		if !client.busy {
			return client, nil
		}
	}
	return nil, errors.New("no available clients")
}

type Client struct {
	conn net.Conn
	mu   sync.Mutex
	busy bool
}

func NewClient(addr string) (*Client, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &Client{
		conn: conn,
	}, nil
}

func (c *Client) Send(message []byte) ([]byte, error) {
	c.busy = true
	c.mu.Lock()
	defer func() {
		c.mu.Unlock()
		c.busy = false
	}()

	// Send the length of the data
	var lengthBuf [4]byte
	binary.LittleEndian.PutUint32(lengthBuf[:], uint32(len(message)))
	_, err := c.conn.Write(lengthBuf[:])
	if err != nil {
		//return nil, fmt.Errorf("failed to write length: %v", err)
		panic(err)
	}

	// Send the data
	_, err = c.conn.Write(message)
	if err != nil {
		//return nil, fmt.Errorf("failed to write data: %v", err)
		panic(err)
	}

	reader := bufio.NewReader(c.conn)

	// Read the length of the response data
	_, err = io.ReadFull(reader, lengthBuf[:])
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("connection closed")
		}
		logger.Errorf("Failed to read length: %v", err)
		panic(err)
	}
	length := binary.LittleEndian.Uint32(lengthBuf[:])

	// Read the response data
	response := make([]byte, length)
	_, err = io.ReadFull(reader, response)
	if err != nil {
		//return nil, fmt.Errorf("failed to read data: %v", err)
		panic(err)
	}

	return response, nil
}

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

	time.Sleep(1000 * time.Millisecond)

	directTargets := make([]string, 0)
	if !*useYapr {
		endpoints := sdk.GetEndpoints("echo-rand")
		logger.Infof("size of endpoints: %d", len(endpoints))
		for endpoint := range endpoints {
			directTargets = append(directTargets, fmt.Sprintf("%s:%d", endpoint.IP, endpoint.Port))
		}
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
				if *useYapr {
					Send(uid, data)
				} else {
					target := directTargets[rand.Intn(len(directTargets))]
					SendDirect(target, uid, data)
				}
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

func Send(uid string, data []byte) {
	match := &types.MatchTarget{
		URI:  "/Echo",
		Port: 9090,
		Headers: map[string]string{
			"x-uid": uid,
		},
	}
	err := sdk.Invoke("echo", match, func(serviceName string, endpoint *types.Endpoint, port uint32, headers map[string]string) error {
		addr := fmt.Sprintf("%s:%d", endpoint.IP, port)
		var clientPool *ClientPool
		rawPool, ok := clientMap.Load(addr)
		if !ok {
			var err error
			clientPool, err = NewClientPool(addr, *conns)
			if err != nil {
				return err
			}
			clientMap.Store(addr, clientPool)
		} else {
			clientPool = rawPool.(*ClientPool)
		}
		d := &echo_tcp.Data{
			Headers: headers,
			Data:    data,
		}
		client, err := clientPool.GetAvailableClient()
		if err != nil {
			logger.Errorf("get client failed: %v", err)
			return err
		}
		_, err = client.Send(d.Marshal())
		if err != nil {
			return err
		}
		//unmarshalData := echo_tcp.UnmarshalData(d2)
		//logger.Infof("response: %s", string(unmarshalData.Data))
		return nil
	})
	if err != nil {
		logger.Errorf("request failed: %v", err)
	}
}

func SendDirect(target string, uid string, data []byte) {
	addr := target
	var clientPool *ClientPool
	rawPool, ok := clientMap.Load(addr)
	if !ok {
		var err error
		clientPool, err = NewClientPool(addr, *conns)
		if err != nil {
			logger.Errorf("new client pool failed: %v", err)
			return
		}
		clientMap.Store(addr, clientPool)
	} else {
		clientPool = rawPool.(*ClientPool)
	}
	d := &echo_tcp.Data{
		Headers: map[string]string{
			"x-uid": uid,
		},
		Data: data,
	}
	client, err := clientPool.GetAvailableClient()
	if err != nil {
		logger.Errorf("get client failed: %v", err)
		return
	}
	d2, err := client.Send(d.Marshal())
	if err != nil {
		logger.Errorf("send failed: %v", err)
	}
	unmarshalData := echo_tcp.UnmarshalData(d2)
	if unmarshalData.Err != nil {
		logger.Errorf("response error: %v", unmarshalData.Err)
	}
	//logger.Infof("response: %s", string(unmarshalData.Data))
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
