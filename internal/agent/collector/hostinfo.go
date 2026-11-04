package collector

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"runtime"
	"strconv"
	"strings"

	agentv1 "github.com/forlwow/labherd/gen/labherd/agent/v1"
)

// ReadHostInfo 读取本机的静态信息，用于 Hello。
// GPU 驱动版本由 GPU 采集器负责，这里留空。
func ReadHostInfo() (*agentv1.HostInfo, error) {
	return readHostInfo(os.DirFS("/"))
}

// readHostInfo 只在内存总量读不到时返回错误；其余字段读不到时留空，
// 因为它们只用于显示，不应阻止 Agent 启动。
func readHostInfo(root fs.FS) (*agentv1.HostInfo, error) {
	mem, err := readWith(root, "proc/meminfo", parseMeminfo)
	if err != nil {
		return nil, err
	}

	info := &agentv1.HostInfo{MemoryTotalBytes: mem.totalBytes}

	if name, err := readWith(root, "etc/os-release", parseOSRelease); err == nil {
		info.Os = name
	}
	if data, err := fs.ReadFile(root, "proc/sys/kernel/osrelease"); err == nil {
		info.KernelVersion = strings.TrimSpace(string(data))
	}
	if cpu, err := readWith(root, "proc/cpuinfo", parseCPUInfo); err == nil {
		info.CpuModel = cpu.model
		info.CpuLogicalCores = cpu.logicalCores
	}
	if info.CpuLogicalCores == 0 {
		info.CpuLogicalCores = uint32(runtime.NumCPU()) //nolint:gosec // CPU 数不会超过 uint32
	}
	return info, nil
}

// parseOSRelease 返回 /etc/os-release 中的 PRETTY_NAME，如 "Ubuntu 22.04.4 LTS"。
func parseOSRelease(r io.Reader) (string, error) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		value, ok := strings.CutPrefix(sc.Text(), "PRETTY_NAME=")
		if !ok {
			continue
		}
		if unquoted, err := strconv.Unquote(value); err == nil {
			return unquoted, nil
		}
		return strings.Trim(value, `'"`), nil
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", errors.New("PRETTY_NAME not found")
}

// cpuInfo 是 /proc/cpuinfo 中用到的信息。
type cpuInfo struct {
	model        string
	logicalCores uint32
}

// parseCPUInfo 取第一个 "model name"，并统计 "processor" 行数作为逻辑 CPU 数。
// 部分 ARM 机器没有 "model name"，此时型号为空。
func parseCPUInfo(r io.Reader) (cpuInfo, error) {
	var info cpuInfo
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		key, value, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "processor":
			info.logicalCores++
		case "model name":
			if info.model == "" {
				info.model = strings.TrimSpace(value)
			}
		}
	}
	if err := sc.Err(); err != nil {
		return cpuInfo{}, fmt.Errorf("scan cpuinfo: %w", err)
	}
	return info, nil
}
