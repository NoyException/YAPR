package node

import (
	"context"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	clientv3 "go.etcd.io/etcd/client/v3"
	"math/rand"
	"net/http"
	"noy/router/pkg/router/config"
	"noy/router/pkg/router/etcd"
	"noy/router/pkg/router/logger"
	"noy/router/pkg/router/store"
	"strconv"
	"sync"
	"time"
)

type ServiceInfo string

type Node struct {
	name     string
	addr     string
	httpAddr string
	stopCh   chan struct{}
	store    store.Store

	etcdClient  *clientv3.Client // etcd 客户端
	redisClient *redis.Client    // 缓存

	// 服务发现
	serviceInfo      map[string]ServiceInfo
	serviceInfoMutex sync.RWMutex
}

func NewNode(name, addr, httpAddr string, store store.Store, cfg *config.Config) *Node {

	// 初始化 etcd 客户端
	etcdClient, err := etcd.NewClient(cfg.Etcd)
	if err != nil {
		panic(err)
	}

	// 初始化 redis 客户端
	opt, err := redis.ParseURL(cfg.Redis.Url)
	if err != nil {
		panic(err)
	}
	redisClient := redis.NewClient(opt)

	return &Node{
		name:             name,
		addr:             addr,
		httpAddr:         httpAddr,
		stopCh:           make(chan struct{}),
		store:            store,
		etcdClient:       etcdClient,
		redisClient:      redisClient,
		serviceInfo:      make(map[string]ServiceInfo),
		serviceInfoMutex: sync.RWMutex{},
	}
}

func (n *Node) Run() {
	// 启动 http metrics 服务
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		if err := http.ListenAndServe(n.httpAddr, nil); err != nil {
			logger.Errorf("[Node] http server start failed: %s", err)
		}
	}()

	go n.serviceDiscovery()

	// 下面的代码用于测试
	//n.redisClient.HSet(context.Background(), "test", "test-key-1", "test-value-1")
	n.store.SetString("k", "v")

	for {
		logger.Debugf("Hello, I'm %s", n.name)
		//s := n.redisClient.HGet(context.Background(), "test", "test-key-1").String()
		//logger.Debugf("get value from redis: %s", s)

		value, _ := n.store.GetString("k")
		logger.Debugf("get value from store: %s", value)
		_, err := n.etcdClient.Put(context.Background(), "service-"+strconv.Itoa(rand.Intn(5)), "test-value-1")
		if err != nil {
			logger.Errorf("put value to etcd failed: %s", err)
		}
		time.Sleep(time.Second * 5)
	}
}

func (n *Node) Stop() {
	close(n.stopCh)
}

func (n *Node) serviceDiscovery() {
	watcher := n.etcdClient.Watch(context.Background(), "service", clientv3.WithPrefix())
	for {
		select {
		case <-n.stopCh:
			return
		case resp := <-watcher:
			for _, event := range resp.Events {
				switch event.Type {
				case clientv3.EventTypePut:
					n.serviceInfoMutex.Lock()
					n.serviceInfo[string(event.Kv.Key)] = ServiceInfo(event.Kv.Value)
					n.serviceInfoMutex.Unlock()
					logger.Infof("service discovered: %s", string(event.Kv.Key))
				case clientv3.EventTypeDelete:
					n.serviceInfoMutex.Lock()
					delete(n.serviceInfo, string(event.Kv.Key))
					n.serviceInfoMutex.Unlock()
					logger.Infof("service removed: %s", string(event.Kv.Key))
				}
			}
		}
	}
}
