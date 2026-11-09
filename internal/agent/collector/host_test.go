package collector

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"testing/fstest"
	"time"

	agentv1 "github.com/forlwow/labherd/gen/labherd/agent/v1"
)

// fakeRoot 返回一个包含主机指标所需文件的假文件系统。
func fakeRoot() fstest.MapFS {
	return fstest.MapFS{
		"proc/stat":                 {Data: []byte("cpu  100 0 100 200 0 0 0 0 0 0\n")},
		"proc/loadavg":              {Data: []byte("0.50 0.40 0.30 1/100 42\n")},
		"proc/meminfo":              {Data: []byte("MemTotal: 2048 kB\nMemAvailable: 1024 kB\n")},
		"proc/mounts":               {Data: []byte("/dev/sda1 / ext4 rw 0 0\n/dev/sdb1 /data xfs rw 0 0\n")},
		"proc/cpuinfo":              {Data: []byte("processor\t: 0\nmodel name\t: Test CPU\nprocessor\t: 1\nmodel name\t: Test CPU\n")},
		"proc/sys/kernel/osrelease": {Data: []byte("6.6.0-test\n")},
		"etc/os-release":            {Data: []byte("NAME=\"Ubuntu\"\nPRETTY_NAME=\"Ubuntu 24.04 LTS\"\n")},
	}
}

// fakeStatfs 对 /data 返回错误，其余挂载点返回固定容量。
func fakeStatfs(path string) (diskSpace, error) {
	if path == "/data" {
		return diskSpace{}, errors.New("permission denied")
	}
	return diskSpace{totalBytes: 1000, availableBytes: 400}, nil
}

func TestHost_Collect(t *testing.T) {
	t.Parallel()
	root := fakeRoot()
	h, err := newHost(root, fakeStatfs, nil)
	if err != nil {
		t.Fatalf("newHost: %v", err)
	}

	// 两次读数之间：total 增加 100，idle 增加 25，利用率 0.75。
	root["proc/stat"] = &fstest.MapFile{Data: []byte("cpu  150 0 125 225 0 0 0 0 0 0\n")}

	sample := &agentv1.MetricsSample{}
	err = h.Collect(t.Context(), sample)
	// /data 的 statfs 失败：返回错误，但其余字段照常填入。
	if err == nil {
		t.Error("want error from failing statfs, got nil")
	}

	m := sample.GetHost()
	if got := m.GetCpuUtilizationRatio(); got != 0.75 {
		t.Errorf("cpu_utilization_ratio = %v, want 0.75", got)
	}
	if got := m.GetLoad1(); got != 0.5 {
		t.Errorf("load1 = %v, want 0.5", got)
	}
	if got := m.GetMemoryAvailableBytes(); got != 1024*1024 {
		t.Errorf("memory_available_bytes = %v, want %v", got, 1024*1024)
	}
	disks := m.GetDisks()
	if len(disks) != 1 || disks[0].GetMountPoint() != "/" || disks[0].GetAvailableBytes() != 400 {
		t.Errorf("disks = %v, want only / with 400 available", disks)
	}
}

func TestNewHost_FailsWithoutProcStat(t *testing.T) {
	t.Parallel()
	if _, err := newHost(fstest.MapFS{}, fakeStatfs, nil); err == nil {
		t.Error("want error, got nil")
	}
}

func TestReadHostInfo(t *testing.T) {
	t.Parallel()
	info, err := readHostInfo(fakeRoot())
	if err != nil {
		t.Fatalf("readHostInfo: %v", err)
	}
	if info.GetOs() != "Ubuntu 24.04 LTS" ||
		info.GetKernelVersion() != "6.6.0-test" ||
		info.GetCpuModel() != "Test CPU" ||
		info.GetCpuLogicalCores() != 2 ||
		info.GetMemoryTotalBytes() != 2048*1024 {
		t.Errorf("unexpected host info: %v", info)
	}
}

// stubCollector 是测试用的 Collector：填入固定负载，并可返回错误。
type stubCollector struct {
	load1 float64
	err   error
}

func (s stubCollector) Collect(_ context.Context, sample *agentv1.MetricsSample) error {
	sample.Host = &agentv1.HostMetrics{Load1: s.load1}
	return s.err
}

func TestSampler_ContinuesAfterCollectorError(t *testing.T) {
	t.Parallel()
	s := NewSampler(slog.New(slog.DiscardHandler),
		stubCollector{load1: 1, err: errors.New("boom")},
		stubCollector{load1: 2},
	)
	now := time.Unix(1700000000, 0)

	sample := s.Sample(t.Context(), now)
	if !sample.GetCollectedAt().AsTime().Equal(now) {
		t.Errorf("collected_at = %v, want %v", sample.GetCollectedAt().AsTime(), now)
	}
	if got := sample.GetHost().GetLoad1(); got != 2 {
		t.Errorf("load1 = %v, want 2 (second collector must still run)", got)
	}
}
