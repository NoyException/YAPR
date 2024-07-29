package main

import (
	"context"
	"example/echo/echopb"
	"google.golang.org/grpc"
	"log"
	"net"
)

type EchoServer struct {
	echopb.UnimplementedEchoServiceServer
}

func (e *EchoServer) Echo(ctx context.Context, request *echopb.EchoRequest) (*echopb.EchoResponse, error) {
	return &echopb.EchoResponse{Message: request.Message}, nil
}

func main() {
	l, err := net.Listen("tcp", ":50051")
	if err != nil {
		panic(err)
	}
	s := grpc.NewServer()
	defer s.Stop()
	echopb.RegisterEchoServiceServer(s, &EchoServer{})
	log.Printf("server listening at %v", l.Addr())
	if err := s.Serve(l); err != nil {
		panic(err)
	}
}
