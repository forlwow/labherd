package collector

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"

	agentv1 "github.com/forlwow/labherd/gen/labherd/agent/v1"
)

// nvidia-smi 查询的字段。顺序与解析函数中的下标一一对应，修改时两处一起改。
const (
	inventoryFields = "uuid,index,name,memory.total,driver_version"
	metricsFields   = "uuid,utilization.gpu,memory.used,temperature.gpu,power.draw"
)

// mib 是 nvidia-smi 在 nounits 模式下显存的单位。
const mib = 1024 * 1024

// gpuInventory 是 GPU 清单及驱动版本，来自一次 inventoryFields 查询。
type gpuInventory struct {
	gpus          []*agentv1.GpuInfo
	driverVersion string
}

// parseInventory 解析 inventoryFields 查询的输出，每张 GPU 一行。
// 清单字段是 GPU 的身份，任何一项读不到都视为错误。
func parseInventory(r io.Reader) (gpuInventory, error) {
	rows, err := readCSV(r, 5)
	if err != nil {
		return gpuInventory{}, err
	}
	var inv gpuInventory
	for _, row := range rows {
		index, err := strconv.ParseUint(row[1], 10, 32)
		if err != nil {
			return gpuInventory{}, fmt.Errorf("parse index %q: %w", row[1], err)
		}
		memTotal, err := strconv.ParseUint(row[3], 10, 64)
		if err != nil {
			return gpuInventory{}, fmt.Errorf("parse memory.total %q: %w", row[3], err)
		}
		inv.gpus = append(inv.gpus, &agentv1.GpuInfo{
			Uuid:             row[0],
			Index:            uint32(index),
			Model:            row[2],
			MemoryTotalBytes: memTotal * mib,
		})
		// 所有 GPU 共用一个驱动，取第一行即可。
		if inv.driverVersion == "" {
			inv.driverVersion = row[4]
		}
	}
	return inv, nil
}

// parseMetrics 解析 metricsFields 查询的输出，每张 GPU 一行。
// 读不到的指标（[N/A]、[Not Supported] 等）不设置对应字段，而不是填 0。
func parseMetrics(r io.Reader) ([]*agentv1.GpuMetrics, error) {
	rows, err := readCSV(r, 5)
	if err != nil {
		return nil, err
	}
	var gpus []*agentv1.GpuMetrics
	for _, row := range rows {
		m := &agentv1.GpuMetrics{Uuid: row[0]}
		if v, ok, err := parseOptionalFloat(row[1]); err != nil {
			return nil, fmt.Errorf("parse utilization.gpu: %w", err)
		} else if ok {
			m.UtilizationRatio = proto.Float64(v / 100)
		}
		if v, ok, err := parseOptionalFloat(row[2]); err != nil {
			return nil, fmt.Errorf("parse memory.used: %w", err)
		} else if ok {
			m.MemoryUsedBytes = proto.Uint64(uint64(v) * mib)
		}
		if v, ok, err := parseOptionalFloat(row[3]); err != nil {
			return nil, fmt.Errorf("parse temperature.gpu: %w", err)
		} else if ok {
			m.TemperatureCelsius = proto.Float64(v)
		}
		if v, ok, err := parseOptionalFloat(row[4]); err != nil {
			return nil, fmt.Errorf("parse power.draw: %w", err)
		} else if ok {
			m.PowerWatts = proto.Float64(v)
		}
		gpus = append(gpus, m)
	}
	return gpus, nil
}

// readCSV 读取 nvidia-smi 的 csv,noheader,nounits 输出，并检查每行的列数。
func readCSV(r io.Reader, fields int) ([][]string, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	cr.FieldsPerRecord = fields
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}
	for _, row := range rows {
		for i := range row {
			row[i] = strings.TrimSpace(row[i])
		}
	}
	return rows, nil
}

// parseOptionalFloat 解析一个可能读不到的数值。
// nvidia-smi 用 [N/A]、[Not Supported]、[Unknown Error] 等表示读不到，此时 ok 为 false。
func parseOptionalFloat(s string) (v float64, ok bool, err error) {
	if s == "" || s == "N/A" || strings.HasPrefix(s, "[") {
		return 0, false, nil
	}
	v, err = strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false, fmt.Errorf("parse %q: %w", s, err)
	}
	return v, true, nil
}
