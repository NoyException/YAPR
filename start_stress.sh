#!/bin/bash

cd ./deploy || exit

CGO_ENABLED=0 go build -o server ../cmd/example/echo/server/main.go
CGO_ENABLED=0 go build -o client ../cmd/example/echo/client/main.go

docker-compose up -d

./server --name=svr1 --ip=localhost
./client --name=cli1