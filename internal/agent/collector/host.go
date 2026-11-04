package collector

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"syscall"

	agentv1 "github.com/forlwow/labherd/gen/labherd/agent/v1"
)

// diskSpace 是一个挂载点的容量，单位为字节。
type diskSpace struct {
	totalBytes     uint64
	availableBytes uint64
}

// statfsFunc 查询挂载点的容量。测试中替换为假实现。
type statfsFunc func(path string) (diskSpace, error)

// Host 采集主机级指标：CPU 利用率、负载、可用内存、磁盘空间。
//
// CPU 利用率是两次 Collect 之间的平均值，所以 Host 会保存上一次的读数，
// 不能被多个协程同时使用。
type Host struct {
	root   fs.FS // 文件系统根目录，文件路径形如 "proc/stat"
	statfs statfsFunc
	prev   cpuTimes
}

// NewHost 返回读取本机 /proc 的 Host。
// 创建时先读一次 CPU 时间，第一次 Collect 得到的是创建以来的平均利用率。
func NewHost() (*Host, error) {
	return newHost(os.DirFS("/"), statfs)
}

func newHost(root fs.FS, statfs statfsFunc) (*Host, error) {
	h := &Host{root: root, statfs: statfs}
	t, err := h.readCPUTimes()
	if err != nil {
		return nil, err
	}
	h.prev = t
	return h, nil
}

// Collect 填入 sample.Host。某一项读取失败时，其余项照常填入，最后返回合并的错误。
func (h *Host) Collect(_ context.Context, sample *agentv1.MetricsSample) error {
	m := &agentv1.HostMetrics{}
	var errs []error

	if cur, err := h.readCPUTimes(); err != nil {
		errs = append(errs, err)
	} else {
		m.CpuUtilizationRatio = cpuUtilization(h.prev, cur)
		h.prev = cur
	}

	if load1, err := readWith(h.root, "proc/loadavg", parseLoad1); err != nil {
		errs = append(errs, err)
	} else {
		m.Load1 = load1
	}

	if mem, err := readWith(h.root, "proc/meminfo", parseMeminfo); err != nil {
		errs = append(errs, err)
	} else {
		m.MemoryAvailableBytes = mem.availableBytes
	}

	disks, err := h.collectDisks()
	if err != nil {
		errs = append(errs, err)
	}
	m.Disks = disks

	sample.Host = m
	return errors.Join(errs...)
}

func (h *Host) readCPUTimes() (cpuTimes, error) {
	return readWith(h.root, "proc/stat", parseCPUTimes)
}

// collectDisks 查询每个本地磁盘挂载点的容量。单个挂载点失败时跳过它，继续其余的。
func (h *Host) collectDisks() ([]*agentv1.DiskUsage, error) {
	mounts, err := readWith(h.root, "proc/mounts", parseMounts)
	if err != nil {
		return nil, err
	}
	var disks []*agentv1.DiskUsage
	var errs []error
	for _, m := range localDisks(mounts) {
		space, err := h.statfs(m.point)
		if err != nil {
			errs = append(errs, fmt.Errorf("statfs %s: %w", m.point, err))
			continue
		}
		disks = append(disks, &agentv1.DiskUsage{
			MountPoint:     m.point,
			TotalBytes:     space.totalBytes,
			AvailableBytes: space.availableBytes,
		})
	}
	return disks, errors.Join(errs...)
}

// statfs 是 statfsFunc 的真实实现。
// 块数以 f_frsize 为单位；旧内核没有该字段时为 0，退回 f_bsize。
func statfs(path string) (diskSpace, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return diskSpace{}, err
	}
	size := st.Frsize
	if size <= 0 {
		size = st.Bsize
	}
	if size <= 0 {
		return diskSpace{}, fmt.Errorf("invalid block size %d", size)
	}
	bs := uint64(size)
	return diskSpace{
		totalBytes:     st.Blocks * bs,
		availableBytes: st.Bavail * bs,
	}, nil
}

// readWith 打开 root 下的 name，并用 parse 解析其内容。
func readWith[T any](root fs.FS, name string, parse func(io.Reader) (T, error)) (T, error) {
	f, err := root.Open(name)
	if err != nil {
		var zero T
		return zero, err
	}
	defer func() { _ = f.Close() }()

	v, err := parse(f)
	if err != nil {
		return v, fmt.Errorf("parse %s: %w", name, err)
	}
	return v, nil
}
