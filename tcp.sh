#!/bin/bash

set -e

cd deploy

# 读入参数：-s 路由策略
STRATEGY="random"
DATA_SIZE="100B"
CPUS=2
CONCURRENCY=200
USE_YAPR=true
RECORD_METRICS=false
TOTAL_REQ=0

while getopts c:d:n:r:s:t:u: flag
do
    case "${flag}" in
        c) CPUS=${OPTARG};;
        d) DATA_SIZE=${OPTARG};;
        n) CONCURRENCY=${OPTARG};;
        r) RECORD_METRICS=${OPTARG};;
        s) STRATEGY=${OPTARG};;
        t) TOTAL_REQ=${OPTARG};;
        u) USE_YAPR=${OPTARG};;
        *) echo "Unknown parameter passed: ${flag}";;
    esac
done
cp "./example/${STRATEGY}.yaml" "./yapr.yaml"

# 设置默认TOTAL_REQ
if [ "${TOTAL_REQ}" == "0" ]; then
  if [ "${RECORD_METRICS}" == "true" ]; then
    TOTAL_REQ=20000000
  else
    TOTAL_REQ=2000000
  fi
fi

# 编译
CGO_ENABLED=0 go build -o server ../cmd/example/echo-tcp/server/main.go
CGO_ENABLED=0 go build -o client ../cmd/example/echo-tcp/client/main.go

# 优雅退出
trap cleanup SIGINT
cleanup() {
    echo "Cleaning up..."
    set +e
    killall -9 client
    killall -9 server
    docker-compose down
}

# 启动docker-compose（etcd+redis+prometheus+grafana）
docker-compose up -d

# 启动server和client
./server --id=1 --weight=1 --endpointCnt=1000 &
if [ "${RECORD_METRICS}" == "true" ]; then
    ./server --id=2 --weight=2 --endpointCnt=1000 &
    ./server --id=3 --weight=3 --endpointCnt=1000 &
fi
echo "1000个endpoint注册完毕"
sleep 2s
echo "压测开始，当前路由策略为${STRATEGY}"
echo "当前CPU核数为$CPUS，并发量为$CONCURRENCY，总请求数为$TOTAL_REQ，数据大小为$DATA_SIZE"
./client --cpus="${CPUS}" --concurrency="${CONCURRENCY}" --totalReq="${TOTAL_REQ}" --dataSize="${DATA_SIZE}" --recordMetrics="${RECORD_METRICS}" --useYapr="${USE_YAPR}"

wait