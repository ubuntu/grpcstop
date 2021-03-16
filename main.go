package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"google.golang.org/grpc"
)

const (
	socket   = "/tmp/grpcstop.socket"
	waitTime = time.Second * 30
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
	// Server listening
	if os.Args[1] == "server" {
		fmt.Println("Starting")
		// Get socket fd from systemd
		fd, err := strconv.Atoi(os.Getenv("LISTEN_FDS"))
		if err != nil || fd == 0 {
			log.Fatalf("No socket passed by systemd: %v", err)
		}
		syscall.CloseOnExec(fd)
		f := os.NewFile(uintptr(2+fd), filepath.Base(socket))
		f.Close()
		lis, err := net.FileListener(f)
		if err != nil {
			log.Fatalf("failed to listen: %v", err)
		}

		s := grpc.NewServer()
		RegisterGrpcStopServer(s, &service{server: s})
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
		return
	}

	// Client
	conn, err := grpc.Dial(socket, grpc.WithInsecure(),
		grpc.WithContextDialer(unixDialer()),
	)
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := NewGrpcStopClient(conn)
	switch os.Args[1] {
	case "wait":
		if _, err := c.Wait(context.Background(), &Empty{}); err != nil {
			log.Fatal(err)
		}
	case "stop":
		if _, err := c.Stop(context.Background(), &Empty{}); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("unknown command: %s", os.Args[1])
	}
}

// Stop requests server for graceful stop.
func (s *service) Stop(context.Context, *Empty) (*Empty, error) {
	go s.server.GracefulStop()
	return &Empty{}, nil
}

// Wait waits block the request for a while before ending up.
func (s *service) Wait(context.Context, *Empty) (*Empty, error) {
	time.Sleep(waitTime)
	return &Empty{}, nil
}
