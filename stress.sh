#!/bin/bash
source ~/.bashrc
set -e

cd deploy

# 读入参数：-s 路由策略
STRATEGY="random"
CPUS=2
CONCURRENCY=200
TOTAL_REQ=20000000

while getopts c:n:s:t: flag
do
    case "${flag}" in
        c) CPUS=${OPTARG};;
        n) CONCURRENCY=${OPTARG};;
        s) STRATEGY=${OPTARG};;
        t) TOTAL_REQ=${OPTARG};;
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
sleep 1
echo "压测开始，当前路由策略为${STRATEGY}"
echo "当前CPU核数为$CPUS，并发量为$CONCURRENCY，总请求数为$TOTAL_REQ"
./client --cpus="${CPUS}" --concurrency="${CONCURRENCY}" --totalReq="${TOTAL_REQ}" &

wait