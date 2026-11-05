package collector

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentv1 "github.com/forlwow/labherd/gen/labherd/agent/v1"
)

// 以下输出取自 WSL2 中的 RTX 5080，第二张卡是虚构的，用来覆盖读不到的字段。
const (
	inventoryOutput = "GPU-eb28ea92-8291-017b-6820-02af53cfb6d7, 0, NVIDIA GeForce RTX 5080, 16303, 610.88\n" +
		"GPU-00000000-0000-0000-0000-000000000001, 1, NVIDIA A100-SXM4-80GB, 81920, 610.88\n"
	metricsOutput = "GPU-eb28ea92-8291-017b-6820-02af53cfb6d7, 1, 2738, 51, 69.00\n" +
		"GPU-00000000-0000-0000-0000-000000000001, [N/A], 0, [Not Supported], [Unknown Error]\n"
)

func TestParseInventory(t *testing.T) {
	t.Parallel()
	inv, err := parseInventory(strings.NewReader(inventoryOutput))
	if err != nil {
		t.Fatalf("parseInventory: %v", err)
	}
	if inv.driverVersion != "610.88" {
		t.Errorf("driver version = %q, want 610.88", inv.driverVersion)
	}
	if len(inv.gpus) != 2 {
		t.Fatalf("got %d gpus, want 2", len(inv.gpus))
	}
	g := inv.gpus[0]
	if g.GetUuid() != "GPU-eb28ea92-8291-017b-6820-02af53cfb6d7" ||
		g.GetIndex() != 0 ||
		g.GetModel() != "NVIDIA GeForce RTX 5080" ||
		g.GetMemoryTotalBytes() != 16303*mib {
		t.Errorf("unexpected gpu info: %v", g)
	}
}

func TestParseInventory_WrongFieldCount(t *testing.T) {
	t.Parallel()
	if _, err := parseInventory(strings.NewReader("GPU-x, 0, model\n")); err == nil {
		t.Error("want error, got nil")
	}
}

func TestParseMetrics(t *testing.T) {
	t.Parallel()
	gpus, err := parseMetrics(strings.NewReader(metricsOutput))
	if err != nil {
		t.Fatalf("parseMetrics: %v", err)
	}
	if len(gpus) != 2 {
		t.Fatalf("got %d gpus, want 2", len(gpus))
	}

	g := gpus[0]
	if g.GetUtilizationRatio() != 0.01 ||
		g.GetMemoryUsedBytes() != 2738*mib ||
		g.GetTemperatureCelsius() != 51 ||
		g.GetPowerWatts() != 69 {
		t.Errorf("unexpected metrics: %v", g)
	}

	// 读不到的字段不设置；显存为 0 时要设置为 0，而不是当作读不到。
	g = gpus[1]
	if g.UtilizationRatio != nil || g.TemperatureCelsius != nil || g.PowerWatts != nil {
		t.Errorf("unsupported fields should be unset: %v", g)
	}
	if g.MemoryUsedBytes == nil || *g.MemoryUsedBytes != 0 {
		t.Errorf("memory_used_bytes = %v, want set to 0", g.MemoryUsedBytes)
	}
}

func TestParseMetrics_InvalidNumber(t *testing.T) {
	t.Parallel()
	_, err := parseMetrics(strings.NewReader("GPU-x, abc, 0, 0, 0\n"))
	if err == nil {
		t.Error("want error, got nil")
	}
}

// fakeRun 返回固定输出，并记录收到的参数。
func fakeRun(out string, err error, gotArgs *[]string) runFunc {
	return func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if gotArgs != nil {
			*gotArgs = args
		}
		return []byte(out), err
	}
}

func TestGPU_Inventory(t *testing.T) {
	t.Parallel()
	var args []string
	g := &GPU{path: "nvidia-smi", timeout: time.Second, run: fakeRun(inventoryOutput, nil, &args)}

	gpus, driver, err := g.Inventory(t.Context())
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	if len(gpus) != 2 || driver != "610.88" {
		t.Errorf("got %d gpus, driver %q", len(gpus), driver)
	}
	if len(args) == 0 || args[0] != "--query-gpu="+inventoryFields {
		t.Errorf("args = %q", args)
	}
}

func TestGPU_CollectAppendsMetrics(t *testing.T) {
	t.Parallel()
	g := &GPU{path: "nvidia-smi", timeout: time.Second, run: fakeRun(metricsOutput, nil, nil)}

	sample := &agentv1.MetricsSample{}
	if err := g.Collect(t.Context(), sample); err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(sample.GetGpus()) != 2 {
		t.Errorf("got %d gpus, want 2", len(sample.GetGpus()))
	}
}

func TestGPU_CollectReturnsCommandError(t *testing.T) {
	t.Parallel()
	g := &GPU{path: "nvidia-smi", timeout: time.Second, run: fakeRun("", errors.New("exit status 9"), nil)}

	sample := &agentv1.MetricsSample{}
	if err := g.Collect(t.Context(), sample); err == nil {
		t.Error("want error, got nil")
	}
	if len(sample.GetGpus()) != 0 {
		t.Errorf("gpus should be empty on error, got %v", sample.GetGpus())
	}
}

func TestRunCommand_Timeout(t *testing.T) {
	t.Parallel()
	sleep, err := findExecutable("", "sleep", nil)
	if err != nil {
		t.Skip("sleep not available")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	if _, err := runCommand(ctx, sleep, "10"); err == nil {
		t.Error("want error after timeout, got nil")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("runCommand took %v, want it killed by the timeout", elapsed)
	}
}

func TestFindExecutable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	exe := filepath.Join(dir, "fake-smi")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o700); err != nil { //nolint:gosec // 测试用的可执行文件
		t.Fatal(err)
	}
	plain := filepath.Join(dir, "plain")
	if err := os.WriteFile(plain, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	const missing = "labherd-no-such-binary"

	tests := []struct {
		name       string
		explicit   string
		candidates []string
		want       string
		wantErr    bool
	}{
		{"explicit path", exe, nil, exe, false},
		{"explicit path not executable", plain, nil, "", true},
		{"fallback to candidates", "", []string{filepath.Join(dir, "absent"), plain, exe}, exe, false},
		{"nothing found", "", []string{plain}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := findExecutable(tt.explicit, missing, tt.candidates)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Errorf("got (%q, %v), want (%q, wantErr=%v)", got, err, tt.want, tt.wantErr)
			}
		})
	}
}
