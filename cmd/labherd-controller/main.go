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
	"time"

	"google.golang.org/grpc"

	"github.com/forlwow/labherd/internal/controller/registry"

	agentv1 "github.com/forlwow/labherd/gen/labherd/agent/v1"
	"github.com/forlwow/labherd/internal/common/version"
	"github.com/forlwow/labherd/internal/controller/session"
)

func main() {
	if err := run(); err != nil {
		slog.Error("controller exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	listenAddr := flag.String("listen", ":9090", "gRPC listen address for agents")
	lostTimeout := flag.Duration("lost-timeout", 10*time.Second, "mark a node lost after this long without messages")
	logLevel := flag.String("log-level", "info", "log level: debug, info, warn, error")
	showVersion := flag.Bool("version", false, "show version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("labherd-controller: %s\n", version.Version)
		return nil
	}
	var level slog.Level
	if err := level.UnmarshalText([]byte(*logLevel)); err != nil {
		return fmt.Errorf("parse log level: %w", err)
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(log)
	reg := registry.New(*lostTimeout)
	lis, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	srv := grpc.NewServer()
	agentv1.RegisterAgentServiceServer(srv, session.NewServer(reg, log))
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
