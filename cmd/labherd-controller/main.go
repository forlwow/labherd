// Command labherd-controller aggregates node state and serves the web console.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	agentv1 "github.com/forlwow/labherd/gen/labherd/agent/v1"
	"github.com/forlwow/labherd/internal/common/version"
	"github.com/forlwow/labherd/internal/controller/session"
	"google.golang.org/grpc"
)

func main() {
}

func run() error {
	listenAddr := flag.String("listen", "9090", "gRPC listen address for agents")
	showVersion := flag.Bool("version", false, "show version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("labherd-controller: %s\n", version.Version)
		return nil
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(log)
	lis, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	srv := grpc.NewServer()
	agentv1.RegisterAgentServiceServer(srv, session.NewServer(log))
	defer srv.Stop()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		log.Info("shutting down")
		srv.Stop()
	}()

	log.Info("controller started", "listen", lis.Addr().String(), "version", version.Version)

	if err := srv.Serve(lis); err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	log.Info("controller stopped")
	return nil
}
