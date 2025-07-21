package main

import (
	"context"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"net"
	"net/http"
	"test/grpc/hello"
)

type HelloServer struct {
	hello.UnimplementedHelloServiceServer
}

func (s *HelloServer) SayHello(ctx context.Context, req *hello.HelloRequest) (*hello.HelloResponse, error) {
	return &hello.HelloResponse{
		Message: "Hello, " + req.Name,
	}, nil
}

func main() {
	l, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalln(err)
	}

	s := grpc.NewServer()

	hello.RegisterHelloServiceServer(s, &HelloServer{})

	go func() {
		if err := s.Serve(l); err != nil {
			log.Fatalln(err)
		}
	}()

	conn, err := grpc.NewClient("127.0.0.1:8080", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalln(err)
	}

	gwmux := runtime.NewServeMux()

	if err := hello.RegisterHelloServiceHandler(context.Background(), gwmux, conn); err != nil {
		log.Fatalln(err)
	}

	server := http.Server{
		Addr:    ":8081",
		Handler: gwmux,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalln(err)
	}
}
