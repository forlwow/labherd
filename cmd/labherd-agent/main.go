// Command labherd-agent collects node metrics and reports them to the controller.
package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/forlwow/labherd/internal/agent/collector"
	"github.com/forlwow/labherd/internal/agent/transport"
	"github.com/forlwow/labherd/internal/common/version"
)

func main() {
	if err := run(); err != nil {
		slog.Error("agent exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("get hostname: %w", err)
	}

	// 骨架阶段用命令行参数；配置统一由 internal/common 处理的 Issue 中替换为 LABHERD_ 环境变量。
	controllerAddr := flag.String("controller", "localhost:9090", "controller gRPC address")
	nodeID := flag.String("node-id", hostname, "stable node identifier")
	interval := flag.Duration("interval", time.Second, "metrics report interval")
	logLevel := flag.String("log-level", "info", "log level: debug, info, warn, error")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("labherd-agent %s\n", version.Version)
		return nil
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(*logLevel)); err != nil {
		return fmt.Errorf("parse log level: %w", err)
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(log)

	hostInfo, err := collector.ReadHostInfo()
	if err != nil {
		return fmt.Errorf("read host info: %w", err)
	}
	host, err := collector.NewHost()
	if err != nil {
		return fmt.Errorf("create host collector: %w", err)
	}

	// Ctrl+C 或 systemd stop 时取消 ctx，Agent 正常结束会话后退出。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := transport.Config{
		ControllerAddr: *controllerAddr,
		NodeID:         *nodeID,
		InstanceID:     rand.Text(),
		Hostname:       hostname,
		AgentVersion:   version.Version,
		Host:           hostInfo,
		Sampler:        collector.NewSampler(log, host),
		Interval:       *interval,
	}
	log.Info("agent starting",
		"node_id", cfg.NodeID,
		"instance_id", cfg.InstanceID,
		"version", cfg.AgentVersion,
		"os", hostInfo.GetOs(),
		"cpu_model", hostInfo.GetCpuModel(),
		"cpu_logical_cores", hostInfo.GetCpuLogicalCores(),
		"memory_total_bytes", hostInfo.GetMemoryTotalBytes())

	if err := transport.Run(ctx, cfg, log); err != nil {
		return err
	}
	log.Info("agent stopped")
	return nil
}
