#!/bin/bash

set -e

cd deploy

# 读入参数：-s 路由策略
STRATEGY="random"
HANDLE_TIME=0

while getopts s: flag
do
    case "${flag}" in
        s) STRATEGY=${OPTARG};;
        *) echo "Unknown parameter passed: ${flag}";;
    esac
done
cp "./example/${STRATEGY}.yaml" "./yapr.yaml"

# 为了体现最少请求数策略，将最少请求数策略下的server处理时间设置为1.2s
if [ "${STRATEGY}" == "least_request" ]; then
  echo "路由策略为最少请求数策略，将server处理时间设置为1.2s（防止rps一直是0）"
  HANDLE_TIME=1200
fi

# 编译
CGO_ENABLED=0 go build -o server ../cmd/example/echo/server/main.go
CGO_ENABLED=0 go build -o client ../cmd/example/echo/client/main.go

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
./server --id=1 --weight=1 --handleTime=${HANDLE_TIME} &
./server --id=2 --weight=2 --handleTime=${HANDLE_TIME} &
./server --id=3 --weight=3 --handleTime=${HANDLE_TIME} &
echo "三个server启动完毕"
sleep 1
./client &
echo "client启动完毕，当前路由策略为${STRATEGY}"

wait