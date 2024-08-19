# YAPR

### Description

Yet Another Poor Compiler

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

