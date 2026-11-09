// Command labherd-agent collects node metrics and reports them to the controller.
package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	agentv1 "github.com/forlwow/labherd/gen/labherd/agent/v1"
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
	nvidiaSMI := flag.String("nvidia-smi", "", "path to nvidia-smi (default: search PATH and known locations)")
	logLevel := flag.String("log-level", "info", "log level: debug, info, warn, error")
	showVersion := flag.Bool("version", false, "print version and exit")
	advertiseIP := flag.String("advertise-ip", "", "IP shown for this node (default: source address seen by the controller)")
	excludeMounts := flag.String("exclude-mounts", "", "comma-separated mount point prefixes to skip, in addition to the defaults")
	flag.Parse()

	if *showVersion {
		fmt.Printf("labherd-agent %s\n", version.Version)
		return nil
	}
	if *advertiseIP != "" && net.ParseIP(*advertiseIP) == nil {
		return fmt.Errorf("invalid -advertise-ip %q", *advertiseIP)
	}
	excluded := append(slices.Clone(collector.DefaultExcludedMounts), splitList(*excludeMounts)...)

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
	host, err := collector.NewHost(excluded)
	if err != nil {
		return fmt.Errorf("create host collector: %w", err)
	}

	// Ctrl+C 或 systemd stop 时取消 ctx，Agent 正常结束会话后退出。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	collectors := []collector.Collector{host}
	gpu, gpus, driverVersion, err := setupGPU(ctx, *nvidiaSMI, log)
	if err != nil {
		return err
	}
	if gpu != nil {
		collectors = append(collectors, gpu)
		hostInfo.GpuDriverVersion = driverVersion
	}

	cfg := transport.Config{
		ControllerAddr: *controllerAddr,
		NodeID:         *nodeID,
		InstanceID:     rand.Text(),
		Hostname:       hostname,
		AgentVersion:   version.Version,
		Host:           hostInfo,
		GPUs:           gpus,
		Sampler:        collector.NewSampler(log, collectors...),
		Interval:       *interval,
		IP:             *advertiseIP,
	}
	log.Info("agent starting",
		"node_id", cfg.NodeID,
		"instance_id", cfg.InstanceID,
		"version", cfg.AgentVersion,
		"os", hostInfo.GetOs(),
		"cpu_model", hostInfo.GetCpuModel(),
		"cpu_logical_cores", hostInfo.GetCpuLogicalCores(),
		"memory_total_bytes", hostInfo.GetMemoryTotalBytes(),
		"gpu_count", len(gpus),
		"gpu_driver_version", hostInfo.GetGpuDriverVersion(),
		"advertise_ip", cfg.IP,
		"excluded_mounts", excluded,
	)

	if err := transport.Run(ctx, cfg, log); err != nil {
		return err
	}
	log.Info("agent stopped")
	return nil
}

// setupGPU 查找 nvidia-smi 并读取 GPU 清单，返回 GPU 采集器、清单和驱动版本。
//
// 显式指定了 nvidia-smi 路径却不可用时返回错误，因为这是配置错误。
// 自动查找不到或读取清单失败时，视为本机没有可用的 GPU：记录日志并返回 nil 采集器，
// Agent 照常上报主机指标。
func setupGPU(ctx context.Context, explicit string, log *slog.Logger) (
	*collector.GPU, []*agentv1.GpuInfo, string, error,
) {
	path, err := collector.FindNvidiaSMI(explicit)
	if err != nil {
		if explicit != "" {
			return nil, nil, "", fmt.Errorf("find nvidia-smi: %w", err)
		}
		log.Info("nvidia-smi not found, gpu metrics disabled")
		return nil, nil, "", nil
	}

	gpu := collector.NewGPU(path)
	gpus, driverVersion, err := gpu.Inventory(ctx)
	if err != nil {
		log.Warn("read gpu inventory failed, gpu metrics disabled", "nvidia_smi", path, "err", err)
		return nil, nil, "", nil
	}
	log.Info("gpu collector enabled", "nvidia_smi", path)
	return gpu, gpus, driverVersion, nil
}

// splitList 把逗号分隔的参数拆成列表，去掉两端空格和空项。
func splitList(s string) []string {
	var out []string
	for item := range strings.SplitSeq(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
