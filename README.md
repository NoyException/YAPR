# YAPR

### Description

Yet Another Poor Compiler

### Quick Start

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