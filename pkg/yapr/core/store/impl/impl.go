package store_impl

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/redis/go-redis/v9"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"noy/router/pkg/yapr/core/config"
	"noy/router/pkg/yapr/core/errcode"
	"noy/router/pkg/yapr/core/store"
	"noy/router/pkg/yapr/core/store/etcd"
	"noy/router/pkg/yapr/core/types"
	"noy/router/pkg/yapr/logger"
	"noy/router/pkg/yapr/metrics"
	"strings"
	"sync"
	"time"
)

type Impl struct {
	etcdClient  *clientv3.Client // etcd 客户端
	redisClient *redis.Client    // redis 客户端

	pod string

	mu                sync.Mutex
	setCustomRouteSha string
	setAttributeSha   string

	serviceToLease map[string]clientv3.LeaseID
}

var _ store.Store = (*Impl)(nil)

func NewImpl(cfg *config.Config, pod string) (*Impl, error) { // 初始化 etcd 客户端
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

	newImpl := &Impl{
		etcdClient:  etcdClient,
		redisClient: redisClient,

		pod: pod,

		mu: sync.Mutex{},

		serviceToLease: make(map[string]clientv3.LeaseID),
	}

	err = newImpl.LoadConfig(cfg.Yapr)
	if err != nil {
		panic(err)
	}
	return newImpl, nil
}

func (s *Impl) LoadConfig(config *config.YaprConfig) error {
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
	response, err := s.etcdClient.Get(ctx, "version")
	if err != nil {
		return err
	}
	if len(response.Kvs) > 0 {
		etcdVersion := string(response.Kvs[0].Value)
		if etcdVersion >= config.Version {
			logger.Info("config is already loaded")
			return nil
		}
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
	_, err = s.etcdClient.Put(ctx, "version", config.Version)
	logger.Infof("config loaded")
	return err
}

func (s *Impl) GetRouter(name string) (*types.Router, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	response, err := s.etcdClient.Get(ctx, "rtr/"+name)
	if err != nil {
		return nil, err
	}
	if len(response.Kvs) == 0 {
		return nil, errcode.ErrRouterNotFound
	}
	bytes := response.Kvs[0].Value
	var router types.Router
	err = json.Unmarshal(bytes, &router)
	if err != nil {
		return nil, err
	}
	return &router, nil
}

func (s *Impl) GetSelectors() (map[string]*types.Selector, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	response, err := s.etcdClient.Get(ctx, "slt/", clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	selectors := make(map[string]*types.Selector)
	for _, kv := range response.Kvs {
		name := strings.TrimPrefix(string(kv.Key), "slt/")
		bytes := kv.Value
		var selector types.Selector
		err = json.Unmarshal(bytes, &selector)
		if err != nil {
			return nil, err
		}

		selectors[name] = &selector
	}
	return selectors, nil
}

type ServiceChangeType int

const (
	ServiceChangeTypePut ServiceChangeType = iota
	ServiceChangeTypeRemove
	ServiceChangeTypeTimeout
)

func (s *Impl) GetServices() (map[string]*types.Service, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	services := make(map[string]*types.Service)

	// 获取所有服务
	response, err := s.etcdClient.Get(ctx, "svc/data/", clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	for _, kv := range response.Kvs {
		splits := strings.Split(string(kv.Key), "/")
		if len(splits) != 4 {
			logger.Warnf("invalid key: %s", kv.Key)
			continue
		}
		name := splits[2]
		id := splits[3]
		service, ok := services[name]
		// 如果服务不存在，则创建一个新的服务
		if !ok {
			service = types.NewService(name)
			services[name] = service
			// 监听服务的属性变化
			s.RegisterAttributeChangeListener(func(endpoint *types.Endpoint, selector string, attribute *types.AttributeInSelector) {
				service.SetAttributeInSelector(endpoint, selector, attribute)
			})
		}
		// 获取服务在某个pod下的所有节点
		var endpoints []*types.Endpoint
		err = json.Unmarshal(kv.Value, &endpoints)
		if err != nil {
			logger.Warnf("unmarshal endpoints error: %v", err)
			continue
		}
		service.RegisterPod(id, endpoints)
		// 获取所有节点的属性
		for _, endpoint := range endpoints {
			res, err := s.redisClient.HGetAll(ctx, "_attr_"+endpoint.String()).Result()
			if err != nil {
				logger.Warnf("get attribute error: %v", err)
				continue
			}
			for selector, rawAttr := range res {
				var attr types.AttributeInSelector
				err = json.Unmarshal([]byte(rawAttr), &attr)
				service.SetAttributeInSelector(endpoint, selector, &attr)
			}
		}
	}

	// 监听服务的节点变化
	s.RegisterServiceChangeListener(func(service string, changeType ServiceChangeType, pod string, endpoints []*types.Endpoint) {
		svc, ok := services[service]
		if !ok {
			logger.Errorf("service %v not found", service)
			return
		}
		switch changeType {
		case ServiceChangeTypePut:
			logger.Infof("put pod: %v in service %v", pod, service)
			metrics.IncAddPodTotal(service, pod)
			svc.RegisterPod(pod, endpoints)
		case ServiceChangeTypeRemove:
			logger.Infof("remove pod: %v in service %v", pod, service)
			metrics.IncRemovePodTotal(service, pod)
			svc.RemovePod(pod)
		case ServiceChangeTypeTimeout:
			logger.Warnf("service %v provided by pod %v timeout", service, pod)
			metrics.IncHangPodTotal(service, pod)
			svc.HangPod(pod)
		}
	})
	return services, nil
}

func (s *Impl) RegisterService(service string, endpoints []*types.Endpoint) (chan struct{}, error) {
	_, ok := s.serviceToLease[service]
	if ok {
		return nil, errcode.ErrServiceAlreadyRegistered
	}

	bytes, err := json.Marshal(endpoints)
	if err != nil {
		return nil, err
	}

	// 创建一个5秒的租约
	leaseResp, err := s.etcdClient.Grant(context.Background(), 5)
	if err != nil {
		return nil, err
	}
	leaseID := leaseResp.ID

	// 启动一个 goroutine 来保持租约的活跃状态
	ch, err := s.etcdClient.KeepAlive(context.Background(), leaseID)
	if err != nil {
		return nil, err
	}
	keepAliveCh := make(chan struct{})
	go func() {
		for {
			resp := <-ch
			if resp == nil {
				close(keepAliveCh)
				return
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = s.etcdClient.Put(ctx, "svc/lease/"+service+"/"+s.pod, "1", clientv3.WithLease(leaseID))
	if err != nil {
		// 关闭租约
		_, _ = s.etcdClient.Revoke(context.Background(), leaseID)
		delete(s.serviceToLease, service)
		return nil, err
	}
	s.serviceToLease[service] = leaseID
	_, err = s.etcdClient.Put(ctx, "svc/data/"+service+"/"+s.pod, string(bytes))
	logger.Infof("service %v registered with %d endpoints", service, len(endpoints))
	return keepAliveCh, err
}

func (s *Impl) UnregisterService(service string) error {
	leaseID, ok := s.serviceToLease[service]
	if !ok {
		return nil
	}
	_, err := s.etcdClient.Revoke(context.Background(), leaseID)
	if err != nil {
		logger.Errorf("revoke lease error: %v", err)
	}
	delete(s.serviceToLease, service)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = s.etcdClient.Delete(ctx, "svc/lease/"+service+"/"+s.pod)
	if err != nil {
		logger.Errorf("delete lease error: %v", err)
	}
	_, err = s.etcdClient.Delete(ctx, "svc/data/"+service+"/"+s.pod)
	if err != nil {
		logger.Errorf("delete data error: %v", err)
	}
	return nil
}

func (s *Impl) RegisterServiceChangeListener(listener func(service string, changeType ServiceChangeType, pod string, endpoints []*types.Endpoint)) {
	go func() {
		watchChan := s.etcdClient.Watch(context.Background(), "svc/", clientv3.WithPrefix())
		for watchResp := range watchChan {
			for _, event := range watchResp.Events {
				key := string(event.Kv.Key)
				value := event.Kv.Value
				splits := strings.Split(key, "/")
				if len(splits) != 4 {
					logger.Warnf("invalid key: %s", key)
					continue
				}
				isLease := splits[1] == "lease"
				service := splits[2]
				id := splits[3]

				if event.Type == clientv3.EventTypeDelete {
					if isLease {
						listener(service, ServiceChangeTypeTimeout, id, nil)
					} else {
						listener(service, ServiceChangeTypeRemove, id, nil)
					}
					continue
				}
				if isLease {
					continue
				}

				endpoints := make([]*types.Endpoint, 0)
				err := json.Unmarshal(value, &endpoints)
				if err != nil {
					logger.Warnf("unmarshal endpoints error: %v", err)
					continue
				}
				listener(service, ServiceChangeTypePut, id, endpoints)
			}
		}
	}()
}

type AttributeChangeNTF struct {
	Selector  string `json:"selector"`
	Endpoint  string `json:"endpoint"`
	Attribute string `json:"attribute"`
}

func (s *Impl) SetEndpointAttribute(endpoint *types.Endpoint, selector string, attribute *types.AttributeInSelector) error {
	if s.setAttributeSha == "" {
		s.mu.Lock()
		if s.setAttributeSha == "" {
			// 仅当新属性存在某字段才覆盖该字段原有属性
			luaScript := `
local endpoint = KEYS[1]
local key = "_attr_" .. endpoint
local selector = ARGV[1]
local newAttr = cjson.decode(ARGV[2])

-- Get existing attribute
local existingAttrJson = redis.call("HGET", key, selector)
local existingAttr = {}
if existingAttrJson then
    existingAttr = cjson.decode(existingAttrJson)
end

-- Merge attributes
for k, v in pairs(newAttr) do
    if v ~= nil then
        existingAttr[k] = v
    end
end

-- Save merged attribute
local mergedAttrJson = cjson.encode(existingAttr)
redis.call("HSET", key, selector, mergedAttrJson)

-- Notify attribute change
redis.call("PUBLISH", "_attr_change", cjson.encode({
    selector = selector,
    endpoint = endpoint,
    attribute = mergedAttrJson
}))

return mergedAttrJson
`
			// 计算 Lua 脚本的 SHA1 哈希值
			sha, err := s.redisClient.ScriptLoad(context.Background(), luaScript).Result()
			if err != nil {
				panic(err)
			}
			s.setAttributeSha = sha
		}
		s.mu.Unlock()
	}

	// Convert attribute to JSON
	attrJson, err := json.Marshal(attribute)
	if err != nil {
		return err
	}

	// Execute Lua script
	_, err = s.redisClient.EvalSha(context.Background(), s.setAttributeSha, []string{endpoint.String()}, selector, string(attrJson)).Result()
	return err
}

func (s *Impl) RegisterAttributeChangeListener(listener func(endpoint *types.Endpoint, selector string, attribute *types.AttributeInSelector)) {
	go func() {
		pubsub := s.redisClient.Subscribe(context.Background(), "_attr_change")
		defer func() {
			_ = pubsub.Close()
		}()
		for {
			msg := <-pubsub.Channel()
			go func() {
				var ntf AttributeChangeNTF
				err := json.Unmarshal([]byte(msg.Payload), &ntf)
				if err != nil {
					logger.Warnf("unmarshal error: %v", err)
					return
				}
				endpoint := types.EndpointFromString(ntf.Endpoint)
				var attr types.AttributeInSelector
				err = json.Unmarshal([]byte(ntf.Attribute), &attr)
				if err != nil {
					logger.Warnf("unmarshal error: %v", err)
					return
				}
				listener(endpoint, ntf.Selector, &attr)
			}()
		}
	}()
}

func (s *Impl) SetCustomRoute(selectorName, headerValue string, endpoint, old *types.Endpoint, timeout int64) (*types.Endpoint, error) {
	if s.setCustomRouteSha == "" {
		s.mu.Lock()
		if s.setCustomRouteSha == "" {
			luaScript := `
local selectorName = KEYS[1]
local headerValue = KEYS[2]
local endpoint = ARGV[1]
local old = ARGV[2]
local timeout = tonumber(ARGV[3])
local currentTime = redis.call("TIME")
local currentTimeMillis = tonumber(currentTime[1]) * 1000 + tonumber(currentTime[2]) / 1000
local deadline
if timeout == 0 then
    deadline = -1
else
    deadline = currentTimeMillis + timeout
end

local endpointTable = "_end_point_" .. selectorName
local ddlTable = "_ddl_" .. selectorName

-- 检查键值对是否存在且未过期
local existing = redis.call("HGET", endpointTable, headerValue)
local existingDDL = tonumber(redis.call("HGET", ddlTable, headerValue))
if existingDDL == nil then
	existingDDL = 1
end
if existing ~= nil and existingDDL > 0 and currentTimeMillis >= existingDDL then
	existing = nil
end

if existing ~= nil and existing ~= old then
	return existing -- 键值对已存在且不等于旧值，插入失败
end

-- 插入新的键值对和超时时间
redis.call("HSET", endpointTable, headerValue, endpoint)
redis.call("HSET", ddlTable, headerValue, deadline)
return -- 键值对不存在/已过期/与旧值匹配，插入成功
`
			// 计算 Lua 脚本的 SHA1 哈希值
			sha, err := s.redisClient.ScriptLoad(context.Background(), luaScript).Result()
			if err != nil {
				panic(err)
			}
			s.setCustomRouteSha = sha
		}
		s.mu.Unlock()
	}

	// 执行 Lua 脚本
	oldStr := ""
	if old != nil {
		oldStr = old.String()
	}
	res, err := s.redisClient.EvalSha(context.Background(), s.setCustomRouteSha,
		[]string{selectorName, headerValue}, endpoint.String(), oldStr, timeout).Result()
	// 设置成功
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return types.EndpointFromString(res.(string)), nil
}

func (s *Impl) GetCustomRoute(selectorName, headerValue string) (*types.Endpoint, error) {
	endpoint, err := s.redisClient.HGet(context.Background(), "_end_point_"+selectorName, headerValue).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, errcode.ErrNoCustomRoute
		}
		return nil, err
	}
	if endpoint == "" {
		return nil, errcode.ErrNoCustomRoute
	}
	return types.EndpointFromString(endpoint), nil
}

func (s *Impl) RemoveCustomRoute(selectorName, headerValue string) error {
	_, err := s.redisClient.HDel(context.Background(), "_end_point_"+selectorName, headerValue).Result()
	return err
}

func (s *Impl) RegisterMigrationListener(listener func(selectorName, headerValue string, from, to *types.Endpoint)) store.CancelFunc {
	cancel := make(chan struct{})
	cancelFunc := func() {
		close(cancel)
	}
	go func() {
		pubsub := s.redisClient.Subscribe(context.Background(), "_migration")
		defer func() {
			_ = pubsub.Close()
		}()
		for {
			select {
			case <-cancel:
				return
			case msg := <-pubsub.Channel():
				splits := strings.Split(msg.Payload, " ")
				if len(splits) != 4 {
					logger.Warnf("invalid message: %s", msg.Payload)
					continue
				}
				selectorName := splits[0]
				headerValue := splits[1]
				from := types.EndpointFromString(splits[2])
				to := types.EndpointFromString(splits[3])
				listener(selectorName, headerValue, from, to)
			}
		}
	}()
	return cancelFunc
}

func (s *Impl) NotifyMigration(selectorName, headerValue string, from, to *types.Endpoint) error {
	_, err := s.redisClient.Publish(context.Background(), "_migration", selectorName+" "+headerValue+" "+from.String()+" "+to.String()).Result()
	return err
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
