package main

import (
	"context"
	"flag"
	"net"
	"os"

	log "github.com/golang/glog"
	"google.golang.org/grpc"
)

const (
	socket = "/tmp/grpcstop.socket"
)

type service struct {
	UnimplementedGrpcStopServer
	server *grpc.Server
}

// unixDialer returns a dialer for a local connection on an unix socket.
func unixDialer() func(ctx context.Context, addr string) (net.Conn, error) {
	return func(ctx context.Context, addr string) (conn net.Conn, err error) {
		unixAddr, err := net.ResolveUnixAddr("unix", addr)
		if err != nil {
			return nil, err
		}
		conn, err = net.DialUnix("unix", nil, unixAddr)
		return conn, err
	}
}

//go:generate protoc --proto_path=. --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative grpcstop.proto

func main() {
	flag.Parse()

	// Server listening
	os.Remove(socket)
	lis, err := net.Listen("unix", socket)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	RegisterGrpcStopServer(s, &service{server: s})
	serverdone := make(chan struct{})
	go func() {
		defer close(serverdone)
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	log.Info("Server ready")

	// Client
	conn, err := grpc.Dial(socket, grpc.WithInsecure(),
		grpc.WithContextDialer(unixDialer()),
	)
	if err != nil {
		log.Fatalf("client did not connect: %v", err)
	}
	c := NewGrpcStopClient(conn)
	if _, err := c.Stop(context.Background(), &Empty{}); err != nil {
		conn.Close()
		log.Fatal(err)
	}
	conn.Close()
	log.Info("Client Stopped")

	<-serverdone
	log.Info("Server Stopped")
}

// Stop requests server for graceful stop.
func (s *service) Stop(context.Context, *Empty) (*Empty, error) {
	go func() {
		log.Info("GracefulStop requested")
		s.server.GracefulStop()
		log.Info("GracefulStop done")
	}()
	return &Empty{}, nil
}
