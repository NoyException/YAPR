# YAPR

### 这是什么？

Yet Another Poor Router

一个简单的、轻量级的、基于golang的客户端路由

### Quick Start

需要提前安装：

- golang >= 1.20
- docker
- docker-compose

##### 演示功能

```shell
sh start.sh -s 路由策略名
```

提供的路由策略名有：

- random 随机
- weighted_random 加权随机
- round_robin 轮询
- weighted_round_robin 加权轮询
- least_request 最小请求数
- hash_ring 哈希环
- direct 动态键值路由
- direct_no_cache 动态键值路由(不使用缓存)
- custom_lua 自定义lua脚本

##### 压测

使用tcp并自定义协议压测，参数可都不填

```shell
sh tcp.sh -s 路由策略名 -c CPU核数 -n 并发数 -t 总请求数 -d 数据包大小 -r 是否上报promethus -u 是否启用sdk(对照用)
```

单纯压测客户端SDK的性能，不真实发送请求，参数可都不填。需要注意的是，压测direct策略前，需要先用tcp压测direct一遍，以便在数据库中先填充好每个user路由到的端点信息

需要注意的是，单纯压测SDK时，不支持least_request策略（因为没有服务端上报请求数）

```shell
sh stress.sh -s 路由策略名 -c CPU核数 -n 并发数 -t 总请求数
```

### 如何配置

下面是一份比较全面的参考配置

```yaml
# etcd 集群配置
etcd:
  endpoints:
    - localhost:2379
  dialTimeout: 5s
  username: ""
  password: ""

# redis 集群配置
redis:
  url: "redis://:noy@localhost:6379"

# yapr 配置
yapr:
  # 版本号增加时会更新数据库（暂未实现热更）
  version: 2.0
  routers:
    - name: "echo" # 构建一个名为echo的路由
      rules: # 匹配规则
        # 匹配规则1
        - matchers: # 匹配器
          - uri: ".*/Echo" # （可选）匹配uri为xxx/Echo的请求
            port: 9090 # （可选）匹配端口为9090的请求
            headers:
              "x-uid": 123 # （可选）匹配header中x-uid为123的请求
          selector: "echo-dir" # 如果匹配成功，使用名为echo-dir的选择器
          catch: # （可选）异常处理
            no_endpoint: # 如果没有可用于选择的端点
              solution: "pass" # 进入下一条匹配规则
            unavailable: # 如果端点不可用
              solution: "block" # 阻塞请求
              timeout: 5.0 # 阻塞超时时间
            default: # 默认情况
              solution: "throw" # 向外抛出异常
              message: "unknown error"
              code: 500
        
        - matchers:
            - uri: ".*/Echo"
          selector: "echo-rr"

  selectors:
    - name: "echo-dir" # 构建一个名为echo-dir的选择器
      service: "echosvr" # 选择的端点来源是名为echosvr的服务
      port: 9090 # 如果端点没有设置port，则默认使用9090
      key: "x-uid" # 动态键值路由的键所在的header key（哈希环也要填写）
      strategy: "direct" # 使用动态键值路由策略
      cache_size: 100000 # 缓存大小（缓存仅限于动态键值路由使用）
      cache_type: "lru" # 缓存类型
      
    - name: "echo-rr"
      service: "echosvr" 
      port: 9090 
      strategy: "round_robin" # 使用轮询策略
      headers: # （可选）使用该选择器选择成功时，添加自定义header
        "custom-header": 233
```