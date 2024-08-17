# YAPR

### Description

Yet Another Poor Compiler

### Quick Start

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
- custom_lua 自定义lua脚本

##### 压测

单纯压测客户端SDK的性能，不真实发送请求

```shell
sh stress.sh -s 路由策略名 -c CPU核数 -n 并发数
```