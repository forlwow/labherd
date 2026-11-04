// Package collector 采集本机指标，组成发往 Controller 的 MetricsSample。
package collector

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	agentv1 "github.com/forlwow/labherd/gen/labherd/agent/v1"
)

// Collector 采集一类指标，并填入采样中自己负责的部分。
//
// 部分指标读取失败时，Collector 仍应填入已读到的部分，再返回错误，
// 这样一项指标出问题不会让整个采样缺失。
type Collector interface {
	Collect(ctx context.Context, sample *agentv1.MetricsSample) error
}

// Sampler 依次调用各个 Collector，组成一次完整采样。
type Sampler struct {
	collectors []Collector
	log        *slog.Logger
}

// NewSampler 返回按给定顺序调用 collectors 的 Sampler。
func NewSampler(log *slog.Logger, collectors ...Collector) *Sampler {
	return &Sampler{collectors: collectors, log: log}
}

// Sample 采集一次整机指标，采样时刻为 now。
// 某个 Collector 出错时只记录警告，不影响其他 Collector。
func (s *Sampler) Sample(ctx context.Context, now time.Time) *agentv1.MetricsSample {
	sample := &agentv1.MetricsSample{CollectedAt: timestamppb.New(now)}
	for _, c := range s.collectors {
		if err := c.Collect(ctx, sample); err != nil {
			s.log.Warn("collect metrics failed", "collector", fmt.Sprintf("%T", c), "err", err)
		}
	}
	return sample
}
