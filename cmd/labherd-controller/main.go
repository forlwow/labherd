// Command labherd-controller aggregates node state and serves the web console.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	agentv1 "github.com/forlwow/labherd/gen/labherd/agent/v1"
	"github.com/forlwow/labherd/internal/common/version"
	"github.com/forlwow/labherd/internal/controller/api"
	"github.com/forlwow/labherd/internal/controller/registry"
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
	httpAddr := flag.String("http-listen", ":8080", "HTTP listen address for the web API")
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

	// 两个端口都先监听：任何一个被占用时直接退出，而不是启动一半。
	grpcLis, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		return fmt.Errorf("listen grpc: %w", err)
	}
	httpLis, err := net.Listen("tcp", *httpAddr)
	if err != nil {
		_ = grpcLis.Close()
		return fmt.Errorf("listen http: %w", err)
	}

	reg := registry.New(*lostTimeout)

	grpcSrv := grpc.NewServer()
	agentv1.RegisterAgentServiceServer(grpcSrv, session.NewServer(reg, log))
	defer grpcSrv.Stop()

	httpSrv := &http.Server{
		Handler:           api.NewHandler(reg, log),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// HTTP 服务异常退出时取消 ctx，使整个进程退出，由 systemd 重启。
	go func() {
		if err := httpSrv.Serve(httpLis); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server failed", "err", err)
			stop()
		}
	}()

	go func() {
		<-ctx.Done()
		log.Info("shutting down")
		// 用 Stop 而不是 GracefulStop：Agent 会话是长连接，GracefulStop 会一直等待。
		grpcSrv.Stop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
	}()

	log.Info("controller started",
		"listen", grpcLis.Addr().String(),
		"http_listen", httpLis.Addr().String(),
		"version", version.Version)

	if err := grpcSrv.Serve(grpcLis); err != nil {
		return fmt.Errorf("serve grpc: %w", err)
	}
	log.Info("controller stopped")
	return nil
}
