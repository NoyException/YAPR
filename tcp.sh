#!/bin/bash

set -e

cd deploy

# 读入参数：-s 路由策略
STRATEGY="random"
CPUS=4
CONCURRENCY=200
USE_YAPR=true

while getopts c:n:s:u: flag
do
    case "${flag}" in
        c) CPUS=${OPTARG};;
        n) CONCURRENCY=${OPTARG};;
        s) STRATEGY=${OPTARG};;
        u) USE_YAPR=${OPTARG};;
        *) echo "Unknown parameter passed: ${flag}";;
    esac
done
cp "./example/${STRATEGY}.yaml" "./yapr.yaml"

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
./server --endpointCnt=1000 &
echo "1000个endpoint注册完毕"
sleep 2s
echo "压测开始，当前路由策略为${STRATEGY}"
./client --cpus="${CPUS}" --concurrency="${CONCURRENCY}" --useYapr="${USE_YAPR}"

wait