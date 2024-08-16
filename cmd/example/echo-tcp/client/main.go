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
	totalReq    = flag.Int("totalReq", 10000000, "total request")
	dataSize    = flag.String("dataSize", "100B", "data size")
	useYapr     = flag.Bool("useYapr", true, "use yapr")
	cpus        = flag.Int("cpus", 4, "cpus")

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
	flag.Parse()
	runtime.GOMAXPROCS(*cpus)
	name = fmt.Sprintf("cli-%d", *id)

	logger.ReplaceDefault(logger.NewWithLogFile(logger.InfoLevel, fmt.Sprintf("/.logs/cli-%d.log", *id)))
	defer func() {
		err := logger.Sync()
		if err != nil {
			fmt.Printf("sync logger error: %v", err)
		}
	}()

	sdk = yaprsdk.Init(*configPath)
	go metrics.Init(8080, false)

	directTargets := make([]string, 0)
	if !*useYapr {
		endpoints := sdk.GetEndpoints("echo-rand")
		logger.Infof("size of endpoints: %d", len(endpoints))
		for endpoint := range endpoints {
			directTargets = append(directTargets, fmt.Sprintf("%s:%d", endpoint.IP, endpoint.Port))
		}
	}

	playerCnt := 1000
	uids := make([]string, playerCnt)
	for i := 0; i < playerCnt; i++ {
		uid := 1000000 + i
		uids[i] = fmt.Sprintf("%d", uid)
	}

	wg := new(sync.WaitGroup)
	now := time.Now()
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, err := createData(*dataSize)
			if err != nil {
				logger.Errorf(err.Error())
				return
			}
			maxJ := *totalReq / *concurrency
			for j := 0; j < maxJ; j++ {
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
			}
		}()
	}

	wg.Wait()
	logger.Infof("total cost: %v, qps: %v", time.Since(now), float64(*totalReq)/time.Since(now).Seconds())
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
		headers["x-uid"] = uid
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
	_, err = client.Send(d.Marshal())
	if err != nil {
		logger.Errorf("send failed: %v", err)
	}
	//unmarshalData := echo_tcp.UnmarshalData(d2)
	//if unmarshalData.Err != nil {
	//	logger.Errorf("response error: %v", unmarshalData.Err)
	//}
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
