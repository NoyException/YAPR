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
	"noy/router/cmd/example/echo-tcp"
	"noy/router/pkg/yapr/core/sdk/client"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
	"noy/router/pkg/yapr/metrics"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

var sdk *yaprsdk.YaprSDK

var (
	// 统一参数
	configPath    = flag.String("configPath", "yapr.yaml", "config file path")
	id            = flag.Int("id", 1, "server id, must be unique")
	concurrency   = flag.Int("concurrency", 200, "goroutines")
	totalReq      = flag.Int("totalReq", 2000000, "total request")
	dataSize      = flag.String("dataSize", "100B", "data size")
	useYapr       = flag.Bool("useYapr", true, "use yapr")
	cpus          = flag.Int("cpus", 4, "cpus")
	recordMetrics = flag.Bool("recordMetrics", false, "record metrics using prometheus")

	name string

	clientMap = new(sync.Map)
)

type Request struct {
	id       uint32
	Data     []byte
	Response chan []byte
}

type Client struct {
	conn      net.Conn
	reader    *bufio.Reader
	sendQueue chan *Request

	pendingRequests sync.Map
	id              uint32
}

func NewClient(addr string) (*Client, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	client := &Client{
		conn:      conn,
		reader:    bufio.NewReader(conn),
		sendQueue: make(chan *Request, 1000),
	}
	go func() {
		for request := range client.sendQueue {
			buf := make([]byte, 4)
			binary.LittleEndian.PutUint32(buf, request.id)
			client.conn.Write(buf)
			client.conn.Write(request.Data)
		}
	}()
	go func() {
		for {
			idBuf := make([]byte, 4)
			_, err := io.ReadFull(client.reader, idBuf)
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					close(client.sendQueue)
					return
				}
				logger.Errorf("read id failed: %v", err)
				return
			}
			rid := binary.LittleEndian.Uint32(idBuf)
			request, loaded := client.pendingRequests.LoadAndDelete(rid)
			if request == nil || !loaded {
				logger.Errorf("request not found: %d", rid)
				return
			}

			sizeBuf := make([]byte, 4)
			_, err = io.ReadFull(client.reader, sizeBuf)
			if err != nil {
				logger.Errorf("read size failed: %v", err)
				return
			}
			size := binary.LittleEndian.Uint32(sizeBuf)
			data := make([]byte, size)
			_, err = io.ReadFull(client.reader, data)
			if err != nil {
				logger.Errorf("read data failed: %v", err)
				return
			}
			req := request.(*Request)
			req.Response <- data
		}
	}()
	return client, nil
}

func (c *Client) Send(message []byte) ([]byte, error) {
	buf := bytes.Buffer{}
	err := binary.Write(&buf, binary.LittleEndian, uint32(len(message)))
	if err != nil {
		return nil, err
	}
	err = binary.Write(&buf, binary.LittleEndian, message)
	if err != nil {
		return nil, err
	}
	request := &Request{
		id:       atomic.AddUint32(&c.id, 1),
		Data:     buf.Bytes(),
		Response: make(chan []byte, 1),
	}
	c.pendingRequests.Store(request.id, request)
	c.sendQueue <- request
	return <-request.Response, nil
}

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

	sdk = yaprsdk.Init(*configPath, false)
	go metrics.Init(8080, !*recordMetrics)

	endpoints := sdk.GetEndpoints("echosvr")
	addrs := make([]string, 0)
	logger.Infof("size of endpoints: %d", len(endpoints))
	for _, endpoint := range endpoints {
		addr := fmt.Sprintf("%s:%d", endpoint.IP, endpoint.Port)
		addrs = append(addrs, addr)
		client, err := NewClient(addr)
		if err != nil {
			panic(err)
		}
		clientMap.Store(addr, client)
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
					target := addrs[rand.Intn(len(addrs))]
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
	clientMap.Range(func(key, value any) bool {
		client := value.(*Client)
		err := client.conn.Close()
		if err != nil {
			logger.Errorf("close conn failed: %v", err)
		}
		return true
	})
	logger.Infof("total cost: %v, qps: %v", time.Since(now), float64(*totalReq)/time.Since(now).Seconds())
	os.Exit(0)
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
		rawClient, ok := clientMap.Load(addr)
		if !ok {
			logger.Errorf("client not found: %s", addr)
			panic("client not found")
		}
		client := rawClient.(*Client)

		headers["x-uid"] = uid
		d := &echo_tcp.Data{
			Headers: headers,
			Data:    data,
		}
		_, err := client.Send(d.Marshal())
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
	rawClient, ok := clientMap.Load(addr)
	if !ok {
		logger.Errorf("client not found")
		return
	}
	client := rawClient.(*Client)

	d := &echo_tcp.Data{
		Headers: map[string]string{
			"x-uid": uid,
		},
		Data: data,
	}
	_, err := client.Send(d.Marshal())
	if err != nil {
		logger.Errorf("send failed: %v", err)
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
