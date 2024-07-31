#!/bin/bash

SERVER_NAME="noy/echoserver:latest"
CLIENT_NAME="noy/echoclient:latest"
# 删除旧的同名镜像
docker rmi $(docker images -q ${SERVER_NAME}) || true
docker rmi $(docker images -q ${CLIENT_NAME}) || true

docker build --rm --force-rm --no-cache -t ${SERVER_NAME} -f ./cmd/example/echo/server/Dockerfile .
docker build --rm --force-rm --no-cache -t ${CLIENT_NAME} -f ./cmd/example/echo/client/Dockerfile .
cd deploy || exit
docker-compose up -d
