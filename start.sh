#!/bin/bash

docker build --rm --force-rm --no-cache -t "noy/echoserver:latest" -f ./cmd/example/echo/server/Dockerfile .
docker build --rm --force-rm --no-cache -t "noy/echoclient:latest" -f ./cmd/example/echo/client/Dockerfile .

cd deploy || exit
docker-compose up -d
