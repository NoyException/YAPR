package main

import (
	"context"
	"example/echo/echopb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"time"
)

func main() {
	conn, err := grpc.NewClient("localhost:50050", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf(err.Error())
	}
	client := echopb.NewEchoServiceClient(conn)
	ticker := time.NewTicker(2 * time.Second)

	for range ticker.C {
		response, err := client.Echo(context.Background(), &echopb.EchoRequest{Message: "Hello world"})
		if err != nil {
			log.Fatalf(err.Error())
		}
		log.Println(response.Message)
	}
}
