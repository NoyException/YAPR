#!/bin/bash

set -e

cd deploy

# 读入参数：-s 路由策略
STRATEGY="random"
CPUS=4

while getopts c:s: flag
do
    case "${flag}" in
        c) CPUS=${OPTARG};;
        s) STRATEGY=${OPTARG};;
        *) echo "Unknown parameter passed: ${flag}";;
    esac
done
cp "./example/${STRATEGY}.yaml" "./yapr.yaml"

# 编译
CGO_ENABLED=0 go build -o server ../cmd/example/stress/server/main.go
CGO_ENABLED=0 go build -o client ../cmd/example/stress/client/main.go

# 优雅退出
trap cleanup SIGINT
cleanup() {
    echo "Cleaning up..."
    docker-compose down
}

# 启动docker-compose（etcd+redis+prometheus+grafana）
docker-compose up -d

echo $CPUS
# 启动server和client
./server --endpointCnt=1000 &
echo "1000个endpoint注册完毕"
sleep 1
./client --cpus="${CPUS}" &
echo "client启动完毕，当前路由策略为${STRATEGY}"

wait