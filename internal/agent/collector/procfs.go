package collector

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// cpuTimes 是 /proc/stat 中 cpu 汇总行的累计时间，单位为 USER_HZ。
type cpuTimes struct {
	idle  uint64
	total uint64
}

// parseCPUTimes 读取 /proc/stat 第一行（所有 CPU 的汇总）。
//
// 字段依次为 user nice system idle iowait irq softirq steal guest guest_nice。
// guest 与 guest_nice 已经计入 user 与 nice，所以只累加前 8 项。
// iowait 期间 CPU 实际空闲，计入 idle。
func parseCPUTimes(r io.Reader) (cpuTimes, error) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 5 || fields[0] != "cpu" {
			continue
		}
		var t cpuTimes
		for i, f := range fields[1:min(len(fields), 9)] {
			v, err := strconv.ParseUint(f, 10, 64)
			if err != nil {
				return cpuTimes{}, fmt.Errorf("parse cpu field %d: %w", i, err)
			}
			t.total += v
			if i == 3 || i == 4 { // idle、iowait
				t.idle += v
			}
		}
		return t, nil
	}
	if err := sc.Err(); err != nil {
		return cpuTimes{}, err
	}
	return cpuTimes{}, errors.New("cpu line not found")
}

// cpuUtilization 计算两次读数之间的 CPU 利用率，范围 0～1。
func cpuUtilization(prev, cur cpuTimes) float64 {
	if cur.total <= prev.total {
		return 0
	}
	total := cur.total - prev.total
	// iowait 计数在内核里可能回退，此时把空闲增量当作 0。
	var idle uint64
	if cur.idle > prev.idle {
		idle = cur.idle - prev.idle
	}
	ratio := 1 - float64(idle)/float64(total)
	return max(0, min(1, ratio))
}

// parseLoad1 读取 /proc/loadavg 的第一个字段。
func parseLoad1(r io.Reader) (float64, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, errors.New("empty loadavg")
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, fmt.Errorf("parse load1: %w", err)
	}
	return v, nil
}

// meminfo 是 /proc/meminfo 中用到的字段，单位为字节。
type meminfo struct {
	totalBytes     uint64
	availableBytes uint64
}

// parseMeminfo 读取 MemTotal 与 MemAvailable。文件中的单位是 kB（实际为 KiB）。
func parseMeminfo(r io.Reader) (meminfo, error) {
	var m meminfo
	var found int
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		key, rest, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		var dst *uint64
		switch key {
		case "MemTotal":
			dst = &m.totalBytes
		case "MemAvailable":
			dst = &m.availableBytes
		default:
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			return meminfo{}, fmt.Errorf("empty value for %s", key)
		}
		v, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return meminfo{}, fmt.Errorf("parse %s: %w", key, err)
		}
		*dst = v * 1024
		found++
	}
	if err := sc.Err(); err != nil {
		return meminfo{}, err
	}
	if found < 2 {
		return meminfo{}, errors.New("MemTotal or MemAvailable not found")
	}
	return m, nil
}

// mount 是 /proc/mounts 中的一行。
type mount struct {
	device   string
	point    string
	fsType   string
	readOnly bool
}

// parseMounts 解析 /proc/mounts。挂载点中的空格等字符以 \040 这样的八进制转义出现。
func parseMounts(r io.Reader) ([]mount, error) {
	var mounts []mount
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 4 {
			continue
		}
		mounts = append(mounts, mount{
			device:   unescapeOctal(fields[0]),
			point:    unescapeOctal(fields[1]),
			fsType:   fields[2],
			readOnly: hasOption(fields[3], "ro"),
		})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return mounts, nil
}

// diskFSTypes 是视为本地磁盘的文件系统类型。
// 排除 tmpfs、overlay、squashfs、nfs、9p（WSL 中的 /mnt/c）等。
var diskFSTypes = map[string]bool{
	"ext2":  true,
	"ext3":  true,
	"ext4":  true,
	"xfs":   true,
	"btrfs": true,
	"f2fs":  true,
	"zfs":   true,
}

// DefaultExcludedMounts 是默认不上报的挂载点前缀：启动分区、snap，以及容器运行时的数据目录。
// 这些挂载可能是 ext4/xfs，按文件系统类型过滤不掉。
var DefaultExcludedMounts = []string{
	"/boot",
	"/snap",
	"/run",
	"/var/lib/docker",
	"/var/lib/containerd",
	"/var/lib/kubelet",
}

// localDisks 选出需要上报的挂载点：本地磁盘文件系统、可写、不在 excluded 中的路径下。
// 同一设备挂载多次时（bind mount 等）只保留路径最短的挂载点；一样长时保留先出现的。
func localDisks(mounts []mount, excluded []string) []mount {
	index := make(map[string]int) // 设备 → 在 disks 中的位置
	var disks []mount
	for _, m := range mounts {
		if !diskFSTypes[m.fsType] || m.readOnly || underAny(m.point, excluded) {
			continue
		}
		if i, ok := index[m.device]; ok {
			if len(m.point) < len(disks[i].point) {
				disks[i] = m
			}
			continue
		}
		index[m.device] = len(disks)
		disks = append(disks, m)
	}
	return disks
}

// underAny 判断 path 是否等于某个前缀或位于其下。
// "/boot" 匹配 "/boot" 与 "/boot/efi"，不匹配 "/bootstrap"。前缀 "/" 会排除一切，因此忽略。
func underAny(path string, prefixes []string) bool {
	for _, p := range prefixes {
		p = strings.TrimSuffix(p, "/")
		if p == "" {
			continue
		}
		if path == p || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}

func hasOption(options, want string) bool {
	for opt := range strings.SplitSeq(options, ",") {
		if opt == want {
			return true
		}
	}
	return false
}

// unescapeOctal 还原 /proc/mounts 中形如 \040 的三位八进制转义。
func unescapeOctal(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+4 <= len(s) {
			if v, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(v))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
