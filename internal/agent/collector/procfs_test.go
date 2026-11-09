package collector

import (
	"slices"
	"strings"
	"testing"
)

func TestParseCPUTimes_SumsFirstEightFields(t *testing.T) {
	t.Parallel()
	// user nice system idle iowait irq softirq steal guest guest_nice
	input := "cpu  100 10 50 800 20 5 5 10 99 99\ncpu0 1 1 1 1 1 1 1 1 0 0\n"
	got, err := parseCPUTimes(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parseCPUTimes: %v", err)
	}
	want := cpuTimes{idle: 820, total: 1000}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParseCPUTimes_MissingCPULine(t *testing.T) {
	t.Parallel()
	if _, err := parseCPUTimes(strings.NewReader("intr 1 2 3\n")); err == nil {
		t.Error("want error, got nil")
	}
}

func TestCPUUtilization(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		prev, cur cpuTimes
		want      float64
	}{
		{"quarter busy", cpuTimes{idle: 100, total: 200}, cpuTimes{idle: 175, total: 300}, 0.25},
		{"fully idle", cpuTimes{idle: 100, total: 200}, cpuTimes{idle: 200, total: 300}, 0},
		{"fully busy", cpuTimes{idle: 100, total: 200}, cpuTimes{idle: 100, total: 300}, 1},
		{"no time passed", cpuTimes{idle: 100, total: 200}, cpuTimes{idle: 100, total: 200}, 0},
		{"idle went backwards", cpuTimes{idle: 100, total: 200}, cpuTimes{idle: 90, total: 300}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := cpuUtilization(tt.prev, tt.cur); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseLoad1(t *testing.T) {
	t.Parallel()
	got, err := parseLoad1(strings.NewReader("1.25 0.80 0.50 2/345 6789\n"))
	if err != nil {
		t.Fatalf("parseLoad1: %v", err)
	}
	if got != 1.25 {
		t.Errorf("got %v, want 1.25", got)
	}
}

func TestParseMeminfo_ConvertsKiBToBytes(t *testing.T) {
	t.Parallel()
	input := "MemTotal:       16000 kB\nMemFree:         1000 kB\nMemAvailable:    8000 kB\n"
	got, err := parseMeminfo(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parseMeminfo: %v", err)
	}
	want := meminfo{totalBytes: 16000 * 1024, availableBytes: 8000 * 1024}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParseMeminfo_MissingAvailable(t *testing.T) {
	t.Parallel()
	if _, err := parseMeminfo(strings.NewReader("MemTotal: 16000 kB\n")); err == nil {
		t.Error("want error, got nil")
	}
}

// wslMounts 是 WSL2 中典型的 /proc/mounts 片段。
const wslMounts = `none /mnt/wsl tmpfs rw,relatime 0 0
/dev/sdc / ext4 rw,relatime,discard,errors=remount-ro,data=ordered 0 0
/dev/sdc /mnt/wslg/distro ext4 ro,relatime,discard,errors=remount-ro,data=ordered 0 0
C:\134 /mnt/c 9p rw,noatime,dirsync,aname=drvfs 0 0
/dev/sdc /var/lib/docker ext4 rw,relatime 0 0
overlay /var/lib/docker/overlay2/abc/merged overlay rw,relatime 0 0
/dev/loop3 /snap/core/1 squashfs ro,nodev,relatime 0 0
/dev/sdd /data\040disk xfs rw,relatime 0 0
/dev/sde /backup ext4 ro,relatime 0 0
`

func TestLocalDisks_FiltersAndDeduplicates(t *testing.T) {
	t.Parallel()
	mounts, err := parseMounts(strings.NewReader(wslMounts))
	if err != nil {
		t.Fatalf("parseMounts: %v", err)
	}
	var got []string
	for _, m := range localDisks(mounts, DefaultExcludedMounts) {
		got = append(got, m.point)
	}
	want := []string{"/", "/data disk"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestUnescapeOctal(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		`/plain`:         "/plain",
		`/a\040b`:        "/a b",
		`C:\134`:         `C:\`,
		`/bad\04`:        `/bad\04`,
		`/not\999octal`:  `/not\999octal`,
		`/two\040\040sp`: "/two  sp",
	}
	for in, want := range tests {
		if got := unescapeOctal(in); got != want {
			t.Errorf("unescapeOctal(%q) = %q, want %q", in, got, want)
		}
	}
}

func points(mounts []mount) []string {
	var out []string
	for _, m := range mounts {
		out = append(out, m.point)
	}
	return out
}

func TestLocalDisks_ExcludesPrefixes(t *testing.T) {
	t.Parallel()
	mounts := []mount{
		{device: "/dev/sda1", point: "/", fsType: "ext4"},
		{device: "/dev/sda2", point: "/boot", fsType: "ext4"},
		{device: "/dev/sdb", point: "/bootstrap", fsType: "ext4"},
		{device: "/dev/mapper/docker-1", point: "/var/lib/docker/devicemapper/mnt/abc", fsType: "xfs"},
		{device: "/dev/sdc", point: "/data/scratch", fsType: "xfs"},
	}
	excluded := append(slices.Clone(DefaultExcludedMounts), "/data/scratch/")
	got := points(localDisks(mounts, excluded))
	want := []string{"/", "/bootstrap"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLocalDisks_KeepsShortestMountPerDevice(t *testing.T) {
	t.Parallel()
	mounts := []mount{
		{device: "/dev/sdb", point: "/home/alice/data", fsType: "ext4"},
		{device: "/dev/sda", point: "/", fsType: "ext4"},
		{device: "/dev/sdb", point: "/data", fsType: "ext4"},
	}
	got := points(localDisks(mounts, nil))
	want := []string{"/data", "/"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestUnderAny(t *testing.T) {
	t.Parallel()
	prefixes := []string{"/boot", "/var/lib/docker/", "/"}
	tests := map[string]bool{
		"/boot":                    true,
		"/boot/efi":                true,
		"/bootstrap":               false,
		"/var/lib/docker":          true,
		"/var/lib/docker/overlay2": true,
		"/var/lib/dockerd":         false,
		"/data":                    false,
	}
	for path, want := range tests {
		if got := underAny(path, prefixes); got != want {
			t.Errorf("underAny(%q) = %v, want %v", path, got, want)
		}
	}
}
