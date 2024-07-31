package store

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"noy/router/pkg/yapr/config"
	"noy/router/pkg/yapr/core"
	"noy/router/pkg/yapr/logger"
	"noy/router/pkg/yapr/store/etcd"
	"strings"
	"time"
)

type Impl struct {
	etcdClient  *clientv3.Client // etcd 客户端
	redisClient *redis.Client    // redis 客户端

	uuid string

	setCustomRouteSha1 string
}

func (s *Impl) LoadConfig(config *core.Config) error {
	session, err := concurrency.NewSession(s.etcdClient, concurrency.WithTTL(5))
	if err != nil {
		logger.Fatalf("could not create session: %v", err)
	}
	defer func(session *concurrency.Session) {
		err := session.Close()
		if err != nil {
			logger.Fatalf("could not close session: %v", err)
		}
	}(session)
	mu := concurrency.NewMutex(session, "config")
	// 尝试获取锁
	ctx := context.TODO()
	if err := mu.Lock(ctx); err != nil {
		logger.Fatalf("could not lock: %v", err)
	}
	defer func() {
		err := mu.Unlock(ctx)
		if err != nil {
			logger.Fatalf("could not unlock: %v", err)
		}
	}()

	// 检查数据是否已加载
	response, err := s.etcdClient.Get(ctx, "ready")
	if err != nil {
		return err
	}
	if len(response.Kvs) > 0 && string(response.Kvs[0].Value) == "true" {
		logger.Info("config is already loaded")
		return nil
	}

	for _, selector := range config.Selectors {
		// 将 Selector 存入 etcd
		bytes, err := json.Marshal(selector)
		if err != nil {
			logger.Warnf("marshal selector error: %v", err)
			continue
		}
		_, err = s.etcdClient.Put(ctx, "slt/"+selector.Name, string(bytes))
		if err != nil {
			logger.Errorf("put selector error: %v", err)
			return err
		}
	}
	logger.Infof("%d selectors loaded", len(config.Selectors))
	for _, router := range config.Routers {
		// 将 Router 存入 etcd
		bytes, err := json.Marshal(router)
		if err != nil {
			logger.Warnf("marshal router error: %v", err)
			continue
		}
		_, err = s.etcdClient.Put(ctx, "rtr/"+router.Name, string(bytes))
		if err != nil {
			logger.Errorf("put router error: %v", err)
			return err
		}
	}
	logger.Infof("%d routers loaded", len(config.Routers))
	_, err = s.etcdClient.Put(ctx, "ready", "true")
	logger.Infof("config loaded")
	return err
}

func (s *Impl) GetRouter(name string) (*core.Router, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	response, err := s.etcdClient.Get(ctx, "rtr/"+name)
	if err != nil {
		return nil, err
	}
	if len(response.Kvs) == 0 {
		return nil, core.ErrRouterNotFound
	}
	bytes := response.Kvs[0].Value
	var router core.Router
	err = json.Unmarshal(bytes, &router)
	if err != nil {
		return nil, err
	}
	selectors, err := s.GetSelectors()
	if err != nil {
		return nil, err
	}
	router.SelectorByName = selectors
	services, err := s.GetServices()
	if err != nil {
		return nil, err
	}
	router.ServiceByName = services
	return &router, nil
}

func (s *Impl) GetSelectors() (map[string]*core.Selector, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	response, err := s.etcdClient.Get(ctx, "slt/", clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	selectors := make(map[string]*core.Selector)
	for _, kv := range response.Kvs {
		name := strings.TrimPrefix(string(kv.Key), "slt/")
		bytes := kv.Value
		var selector core.Selector
		err = json.Unmarshal(bytes, &selector)
		if err != nil {
			return nil, err
		}
		selectors[name] = &selector
	}
	return selectors, nil
}

func (s *Impl) GetServices() (map[string]*core.Service, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	services := make(map[string]*core.Service)
	// 监听服务的节点变化
	s.RegisterServiceChangeListener(func(service string, isPut bool, pod string, endpoints []*core.Endpoint) {
		svc := services[service]
		svc.SetDirty()
		if isPut {
			for _, endpoint := range endpoints {
				svc.AttrMap[*endpoint] = make(map[string]*core.Attr)
			}
			svc.EndpointsByPod[pod] = endpoints
		} else {
			logger.Infof("delete pod: %v in service %v", pod, service)
			endpoints = svc.EndpointsByPod[pod]
			for _, endpoint := range endpoints {
				logger.Debugf("delete endpoint: %v in service %v", endpoint, service)
				delete(svc.AttrMap, *endpoint)
				for ep, _ := range svc.AttrMap {
					if ep.IP == endpoint.IP {
						logger.Errorf("failed to delete endpoint: %v in service %v", endpoint, service)
					}
				}
			}
			delete(svc.EndpointsByPod, pod)
		}
	})

	// 获取所有服务
	response, err := s.etcdClient.Get(ctx, "svc/", clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	for _, kv := range response.Kvs {
		splits := strings.Split(string(kv.Key), "/")
		if len(splits) != 3 {
			logger.Warnf("invalid key: %s", kv.Key)
			continue
		}
		name := splits[1]
		id := splits[2]
		service, ok := services[name]
		// 如果服务不存在，则创建一个新的服务
		if !ok {
			service = core.NewService(name)
			services[name] = service
			// 监听服务的属性变化
			s.RegisterAttributeChangeListener(func(endpoint *core.Endpoint, selector string, attribute *core.Attr) {
				service.SetAttribute(endpoint, selector, attribute)
			})
			// 获取服务的所有属性
			response, err = s.etcdClient.Get(ctx, "attr/", clientv3.WithPrefix())
			if err != nil {
				return nil, err
			}
			for _, kv := range response.Kvs {
				var attr core.Attr
				err = json.Unmarshal(kv.Value, &attr)
				if err != nil {
					logger.Warnf("unmarshal attribute error: %v", err)
					return nil, err
				}
				splits := strings.Split(string(kv.Key), "/")
				if len(splits) != 3 {
					logger.Warnf("invalid key: %s", kv.Key)
					continue
				}
				selector := splits[1]
				ip := splits[2]
				endpoint := &core.Endpoint{
					IP: ip,
				}
				service.SetAttribute(endpoint, selector, &attr)
			}
		}
		// 获取服务在某个pod下的所有节点
		var endpoints []*core.Endpoint
		err = json.Unmarshal(kv.Value, &endpoints)
		if err != nil {
			return nil, err
		}
		service.EndpointsByPod[id] = endpoints
		service.SetDirty()
	}
	return services, nil
}

func (s *Impl) RegisterService(service string, endpoints []*core.Endpoint) error {
	bytes, err := json.Marshal(endpoints)
	if err != nil {
		return err
	}

	// 创建一个5秒的租约
	leaseResp, err := s.etcdClient.Grant(context.Background(), 5)
	if err != nil {
		return err
	}
	leaseID := leaseResp.ID

	// 启动一个 goroutine 来保持租约的活跃状态
	ch, err := s.etcdClient.KeepAlive(context.Background(), leaseID)
	if err != nil {
		return err
	}
	go func() {
		for {
			resp := <-ch
			if resp == nil {
				return
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = s.etcdClient.Put(ctx, "svc/"+service+"/"+s.uuid, string(bytes), clientv3.WithLease(leaseID))
	return err
}

func (s *Impl) RegisterServiceChangeListener(listener func(service string, isPut bool, pod string, endpoints []*core.Endpoint)) {
	go func() {
		watchChan := s.etcdClient.Watch(context.Background(), "svc/", clientv3.WithPrefix())
		for watchResp := range watchChan {
			for _, event := range watchResp.Events {
				key := string(event.Kv.Key)
				value := event.Kv.Value
				splits := strings.Split(key, "/")
				if len(splits) != 3 {
					logger.Warnf("invalid key: %s", key)
					continue
				}
				service := splits[1]
				id := splits[2]

				if event.Type == clientv3.EventTypeDelete {
					listener(service, false, id, nil)
					continue
				}

				endpoints := make([]*core.Endpoint, 0)
				err := json.Unmarshal(value, &endpoints)
				if err != nil {
					logger.Warnf("unmarshal endpoints error: %v", err)
					continue
				}
				listener(service, true, id, endpoints)
			}
		}
	}()
}

func (s *Impl) SetEndpointAttribute(endpoint *core.Endpoint, selector string, attribute *core.Attr) error {
	key := "attr/" + selector + "/" + endpoint.IP
	bytes, err := json.Marshal(attribute)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = s.etcdClient.Put(ctx, key, string(bytes))
	return err
}

func (s *Impl) RegisterAttributeChangeListener(listener func(endpoint *core.Endpoint, selector string, attribute *core.Attr)) {
	go func() {
		watchChan := s.etcdClient.Watch(context.Background(), "attr/", clientv3.WithPrefix())
		for watchResp := range watchChan {
			for _, event := range watchResp.Events {
				attribute := &core.Attr{}
				err := json.Unmarshal(event.Kv.Value, attribute)
				if err != nil {
					logger.Warnf("unmarshal attribute error: %v", err)
					continue
				}
				splits := strings.Split(string(event.Kv.Key), "/")
				if len(splits) != 3 {
					logger.Warnf("invalid key: %s", event.Kv.Key)
					continue
				}
				selector := splits[1]
				endpoint := &core.Endpoint{
					IP: splits[2],
				}
				listener(endpoint, selector, attribute)
			}
		}
	}()
}

func (s *Impl) SetCustomRoute(selectorName, headerValue string, endpoint *core.Endpoint, timeout int64) error {
	if s.setCustomRouteSha1 == "" {
		luaScript := `
local selectorName = KEYS[1]
local headerValue = KEYS[2]
local endpoint = ARGV[1]
local timeout = tonumber(ARGV[2])
local currentTime = redis.call("TIME")
local currentTimeMillis = tonumber(currentTime[1]) * 1000 + tonumber(currentTime[2]) / 1000
local deadline = currentTimeMillis + timeout

local endpointTable = "_end_point_" .. selectorName
local ddlTable = "_ddl_" .. selectorName

-- 检查键值对是否存在且未过期
local existing = redis.call("HGET", endpointTable, headerValue)
local existingDDL = redis.call("HGET", ddlTable, headerValue)

if existing and existingDDL then
	local ddl = tonumber(existingDDL)
	if currentTimeMillis < ddl or ddl == 0 then
    	return 1 -- 键值对已存在且未过期，插入失败
	end
end

-- 插入新的键值对和超时时间
redis.call("HSET", endpointTable, headerValue, endpoint)
redis.call("HSET", ddlTable, headerValue, deadline)
return 0 -- 键值对不存在或已过期，插入成功
`
		// 计算 Lua 脚本的 SHA1 哈希值
		sha1, err := s.redisClient.ScriptLoad(context.Background(), luaScript).Result()
		if err != nil {
			panic(err)
		}
		s.setCustomRouteSha1 = sha1
	}

	// 执行 Lua 脚本
	res, err := s.redisClient.EvalSha(context.Background(), s.setCustomRouteSha1, []string{selectorName, headerValue}, endpoint, timeout).Result()
	if err != nil {
		return err
	}
	if res.(int64) == 1 {
		return core.ErrRouteAlreadyExists
	}
	return nil
}

func (s *Impl) Close() {
	err := s.etcdClient.Close()
	if err != nil {
		logger.Warnf("etcd close error: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.redisClient.Shutdown(ctx)
}

var _ Store = (*Impl)(nil)

func NewImpl(cfg *config.Config) (*Impl, error) { // 初始化 etcd 客户端
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

	return &Impl{
		etcdClient:  etcdClient,
		redisClient: redisClient,

		uuid: uuid.New().String(),
	}, nil
}
