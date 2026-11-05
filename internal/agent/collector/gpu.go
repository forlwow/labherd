package collector

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	agentv1 "github.com/forlwow/labherd/gen/labherd/agent/v1"
)

// ErrNvidiaSMINotFound 表示找不到 nvidia-smi，通常说明本机没有 NVIDIA GPU 或没有装驱动。
var ErrNvidiaSMINotFound = errors.New("nvidia-smi not found")

// nvidiaSMIPaths 是 PATH 中找不到时依次尝试的位置。
// systemd 服务与远程 shell 的 PATH 往往很精简；WSL 中 nvidia-smi 位于 /usr/lib/wsl/lib，
// 只有 WSL 直接启动的 shell 才会把它加入 PATH。
var nvidiaSMIPaths = []string{
	"/usr/bin/nvidia-smi",
	"/usr/lib/wsl/lib/nvidia-smi",
}

// defaultGPUTimeout 是单次调用 nvidia-smi 的超时。驱动异常时 nvidia-smi 可能长时间卡住。
const defaultGPUTimeout = 2 * time.Second

// runFunc 执行命令并返回标准输出。测试中替换为假实现。
type runFunc func(ctx context.Context, path string, args ...string) ([]byte, error)

// GPU 通过调用 nvidia-smi 采集 GPU 指标。
type GPU struct {
	path    string
	timeout time.Duration
	run     runFunc
}

// FindNvidiaSMI 返回 nvidia-smi 的路径。
// explicit 不为空时只检查该路径；否则先查 PATH，再查 nvidiaSMIPaths。
func FindNvidiaSMI(explicit string) (string, error) {
	return findExecutable(explicit, "nvidia-smi", nvidiaSMIPaths)
}

func findExecutable(explicit, name string, candidates []string) (string, error) {
	if explicit != "" {
		if !isExecutable(explicit) {
			return "", fmt.Errorf("%s is not an executable file", explicit)
		}
		return explicit, nil
	}
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	for _, c := range candidates {
		if isExecutable(c) {
			return c, nil
		}
	}
	return "", ErrNvidiaSMINotFound
}

func isExecutable(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular() && fi.Mode().Perm()&0o111 != 0
}

// NewGPU 返回调用 path 处 nvidia-smi 的 GPU 采集器。
func NewGPU(path string) *GPU {
	return &GPU{path: path, timeout: defaultGPUTimeout, run: runCommand}
}

// Inventory 返回 GPU 清单和驱动版本，用于 Hello。
func (g *GPU) Inventory(ctx context.Context) ([]*agentv1.GpuInfo, string, error) {
	out, err := g.query(ctx, inventoryFields)
	if err != nil {
		return nil, "", err
	}
	inv, err := parseInventory(bytes.NewReader(out))
	if err != nil {
		return nil, "", fmt.Errorf("parse gpu inventory: %w", err)
	}
	return inv.gpus, inv.driverVersion, nil
}

// Collect 把每张 GPU 的指标追加到 sample.Gpus。
func (g *GPU) Collect(ctx context.Context, sample *agentv1.MetricsSample) error {
	out, err := g.query(ctx, metricsFields)
	if err != nil {
		return err
	}
	gpus, err := parseMetrics(bytes.NewReader(out))
	if err != nil {
		return fmt.Errorf("parse gpu metrics: %w", err)
	}
	sample.Gpus = append(sample.Gpus, gpus...)
	return nil
}

// query 以 CSV 格式查询给定字段，超时后终止 nvidia-smi。
func (g *GPU) query(ctx context.Context, fields string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()
	out, err := g.run(ctx, g.path, "--query-gpu="+fields, "--format=csv,noheader,nounits")
	if err != nil {
		return nil, fmt.Errorf("run nvidia-smi: %w", err)
	}
	return out, nil
}

// runCommand 是 runFunc 的真实实现。
// 命令失败时把输出附在错误信息中：nvidia-smi 的错误提示（如驱动版本不匹配）多数写在标准输出。
func runCommand(ctx context.Context, path string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, path, args...) //nolint:gosec // path 来自配置或固定位置，参数是常量
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String() + " " + string(out))
		if msg != "" {
			return nil, fmt.Errorf("%w: %s", err, msg)
		}
		return nil, err
	}
	return out, nil
}
