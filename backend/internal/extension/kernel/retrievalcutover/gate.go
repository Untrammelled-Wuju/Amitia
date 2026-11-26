//go:build rsi_cutover

package retrievalcutover

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrThresholdsNotMet = errors.New("retrieval cutover thresholds not met")
	ErrCutoverInvalid   = errors.New("retrieval cutover unit invalid")
	ErrBaselineDrift    = errors.New("retrieval cutover baseline drifted")
)

type Thresholds struct {
	MinTop1Recall      float64
	MinTop5Recall      float64
	MaxFalseInjection  int
	MaxPermissionLeaks int
	MaxInjectionDrift  int
	MaxP99Latency      time.Duration
	MaxSchemaTokens    int
	MaxTokenDrift      int
	MaxSchemaReads     int
}

func DefaultThresholds() Thresholds {
	return Thresholds{
		MinTop1Recall:      0.95,
		MinTop5Recall:      0.98,
		MaxFalseInjection:  0,
		MaxPermissionLeaks: 0,
		MaxInjectionDrift:  0,
		MaxP99Latency:      500 * time.Millisecond,
		MaxSchemaTokens:    2048,
		MaxTokenDrift:      0,
		MaxSchemaReads:     12,
	}
}

func (t Thresholds) Validate() error {
	if t.MinTop1Recall <= 0 || t.MinTop1Recall > 1 {
		return fmt.Errorf("%w: min top1 recall=%v", ErrCutoverInvalid, t.MinTop1Recall)
	}
	if t.MinTop5Recall < t.MinTop1Recall || t.MinTop5Recall > 1 {
		return fmt.Errorf("%w: min top5 recall=%v top1=%v", ErrCutoverInvalid, t.MinTop5Recall, t.MinTop1Recall)
	}
	if t.MaxFalseInjection < 0 || t.MaxPermissionLeaks < 0 || t.MaxInjectionDrift < 0 || t.MaxTokenDrift < 0 || t.MaxSchemaReads < 0 {
		return fmt.Errorf("%w: negative metric ceiling", ErrCutoverInvalid)
	}
	if t.MaxP99Latency <= 0 {
		return fmt.Errorf("%w: max p99 latency=%s", ErrCutoverInvalid, t.MaxP99Latency)
	}
	if t.MaxSchemaTokens <= 0 {
		return fmt.Errorf("%w: max schema tokens=%d", ErrCutoverInvalid, t.MaxSchemaTokens)
	}
	return nil
}

type Metrics struct {
	Tasks           int
	Top1Hits        int
	Top5Hits        int
	FalseInjection  int
	PermissionLeaks int
	InjectionDrift  int
	P99Latency      time.Duration
	SchemaTokens    int
	TokenDrift      int
	SchemaReads     int
}

func (m Metrics) Top1Recall() float64 {
	if m.Tasks <= 0 {
		return 0
	}
	return float64(m.Top1Hits) / float64(m.Tasks)
}

func (m Metrics) Top5Recall() float64 {
	if m.Tasks <= 0 {
		return 0
	}
	return float64(m.Top5Hits) / float64(m.Tasks)
}

func (m Metrics) Validate() error {
	if m.Tasks <= 0 {
		return fmt.Errorf("%w: tasks=%d", ErrCutoverInvalid, m.Tasks)
	}
	if m.Top1Hits < 0 || m.Top5Hits < 0 || m.Top1Hits > m.Tasks || m.Top5Hits > m.Tasks {
		return fmt.Errorf("%w: hits top1=%d top5=%d tasks=%d", ErrCutoverInvalid, m.Top1Hits, m.Top5Hits, m.Tasks)
	}
	if m.Top1Hits > m.Top5Hits {
		return fmt.Errorf("%w: top1 hits=%d exceed top5 hits=%d", ErrCutoverInvalid, m.Top1Hits, m.Top5Hits)
	}
	if m.FalseInjection < 0 || m.PermissionLeaks < 0 || m.InjectionDrift < 0 || m.TokenDrift < 0 || m.SchemaReads < 0 || m.SchemaTokens < 0 {
		return fmt.Errorf("%w: negative metric value", ErrCutoverInvalid)
	}
	return nil
}

type Failure struct {
	Metric   string
	Observed string
	Required string
}

func (f Failure) String() string {
	return fmt.Sprintf("%s observed=%s required=%s", f.Metric, f.Observed, f.Required)
}

type Verdict struct {
	Passed   bool
	Failures []Failure
}

func Evaluate(m Metrics, t Thresholds) Verdict {
	verdict := Verdict{Passed: true, Failures: []Failure{}}
	fail := func(metric string, observed string, required string) {
		verdict.Passed = false
		verdict.Failures = append(verdict.Failures, Failure{Metric: metric, Observed: observed, Required: required})
	}
	if m.Tasks <= 0 {
		fail("tasks", fmt.Sprintf("%d", m.Tasks), ">0")
		return verdict
	}
	if m.Validate() != nil {
		fail("metrics", m.Validate().Error(), "valid")
		return verdict
	}
	if t.Validate() != nil {
		fail("thresholds", t.Validate().Error(), "valid")
		return verdict
	}
	if recall := m.Top1Recall(); recall < t.MinTop1Recall {
		fail("top1_recall", fmt.Sprintf("%.3f", recall), fmt.Sprintf(">=%.3f", t.MinTop1Recall))
	}
	if recall := m.Top5Recall(); recall < t.MinTop5Recall {
		fail("top5_recall", fmt.Sprintf("%.3f", recall), fmt.Sprintf(">=%.3f", t.MinTop5Recall))
	}
	if m.FalseInjection > t.MaxFalseInjection {
		fail("false_injection", fmt.Sprintf("%d", m.FalseInjection), fmt.Sprintf("<=%d", t.MaxFalseInjection))
	}
	if m.PermissionLeaks > t.MaxPermissionLeaks {
		fail("permission_leaks", fmt.Sprintf("%d", m.PermissionLeaks), fmt.Sprintf("<=%d", t.MaxPermissionLeaks))
	}
	if m.InjectionDrift > t.MaxInjectionDrift {
		fail("injection_drift", fmt.Sprintf("%d", m.InjectionDrift), fmt.Sprintf("<=%d", t.MaxInjectionDrift))
	}
	if m.P99Latency > t.MaxP99Latency {
		fail("p99_latency", m.P99Latency.String(), "<="+t.MaxP99Latency.String())
	}
	if m.SchemaTokens > t.MaxSchemaTokens {
		fail("schema_tokens", fmt.Sprintf("%d", m.SchemaTokens), fmt.Sprintf("<=%d", t.MaxSchemaTokens))
	}
	if m.TokenDrift > t.MaxTokenDrift {
		fail("token_drift", fmt.Sprintf("%d", m.TokenDrift), fmt.Sprintf("<=%d", t.MaxTokenDrift))
	}
	if m.SchemaReads > t.MaxSchemaReads {
		fail("schema_reads", fmt.Sprintf("%d", m.SchemaReads), fmt.Sprintf("<=%d", t.MaxSchemaReads))
	}
	return verdict
}

type Baseline struct {
	Tasks           int `json:"tasks"`
	Top1Hits        int `json:"top1_hits"`
	Top5Hits        int `json:"top5_hits"`
	FalseInjection  int `json:"false_injection"`
	PermissionLeaks int `json:"permission_leaks"`
	InjectionDrift  int `json:"injection_drift"`
	SchemaTokens    int `json:"schema_tokens"`
	TokenDrift      int `json:"token_drift"`
	SchemaReads     int `json:"schema_reads"`
}

func BaselineFromMetrics(m Metrics) Baseline {
	return Baseline{
		Tasks:           m.Tasks,
		Top1Hits:        m.Top1Hits,
		Top5Hits:        m.Top5Hits,
		FalseInjection:  m.FalseInjection,
		PermissionLeaks: m.PermissionLeaks,
		InjectionDrift:  m.InjectionDrift,
		SchemaTokens:    m.SchemaTokens,
		TokenDrift:      m.TokenDrift,
		SchemaReads:     m.SchemaReads,
	}
}

func (b Baseline) Validate() error {
	if b.Tasks <= 0 {
		return fmt.Errorf("%w: baseline tasks=%d", ErrCutoverInvalid, b.Tasks)
	}
	if b.Top1Hits < 0 || b.Top5Hits < 0 || b.Top1Hits > b.Tasks || b.Top5Hits > b.Tasks || b.Top1Hits > b.Top5Hits {
		return fmt.Errorf("%w: baseline hits top1=%d top5=%d tasks=%d", ErrCutoverInvalid, b.Top1Hits, b.Top5Hits, b.Tasks)
	}
	if b.FalseInjection < 0 || b.PermissionLeaks < 0 || b.InjectionDrift < 0 || b.SchemaTokens < 0 || b.TokenDrift < 0 || b.SchemaReads < 0 {
		return fmt.Errorf("%w: baseline negative metric", ErrCutoverInvalid)
	}
	return nil
}

func (b Baseline) Compare(m Metrics) []Failure {
	current := BaselineFromMetrics(m)
	pairs := []struct {
		metric   string
		baseline int
		observed int
	}{
		{"tasks", b.Tasks, current.Tasks},
		{"top1_hits", b.Top1Hits, current.Top1Hits},
		{"top5_hits", b.Top5Hits, current.Top5Hits},
		{"false_injection", b.FalseInjection, current.FalseInjection},
		{"permission_leaks", b.PermissionLeaks, current.PermissionLeaks},
		{"injection_drift", b.InjectionDrift, current.InjectionDrift},
		{"schema_tokens", b.SchemaTokens, current.SchemaTokens},
		{"token_drift", b.TokenDrift, current.TokenDrift},
		{"schema_reads", b.SchemaReads, current.SchemaReads},
	}
	failures := []Failure{}
	for _, pair := range pairs {
		if pair.baseline != pair.observed {
			failures = append(failures, Failure{
				Metric:   "baseline." + pair.metric,
				Observed: fmt.Sprintf("%d", pair.observed),
				Required: fmt.Sprintf("%d", pair.baseline),
			})
		}
	}
	return failures
}

type Unit struct {
	thresholds     Thresholds
	metrics        Metrics
	verdict        Verdict
	recorded       bool
	legacyDisabled bool
}

func NewUnit(thresholds Thresholds) (*Unit, error) {
	if err := thresholds.Validate(); err != nil {
		return nil, err
	}
	return &Unit{thresholds: thresholds}, nil
}

func (u *Unit) Thresholds() Thresholds {
	if u == nil {
		return Thresholds{}
	}
	return u.thresholds
}

func (u *Unit) Record(metrics Metrics) (Verdict, error) {
	if u == nil {
		return Verdict{}, fmt.Errorf("%w: unit is nil", ErrCutoverInvalid)
	}
	if err := metrics.Validate(); err != nil {
		return Verdict{}, err
	}
	u.metrics = metrics
	u.verdict = Evaluate(metrics, u.thresholds)
	u.recorded = true
	return u.verdict, nil
}

func (u *Unit) Metrics() Metrics {
	if u == nil {
		return Metrics{}
	}
	return u.metrics
}

func (u *Unit) Verdict() Verdict {
	if u == nil {
		return Verdict{}
	}
	return Verdict{Passed: u.verdict.Passed, Failures: append([]Failure{}, u.verdict.Failures...)}
}

func (u *Unit) DisableLegacy() error {
	if u == nil {
		return fmt.Errorf("%w: unit is nil", ErrCutoverInvalid)
	}
	if !u.recorded {
		return fmt.Errorf("%w: metrics were never recorded", ErrCutoverInvalid)
	}
	if !u.verdict.Passed {
		return fmt.Errorf("%w: %d failing metrics: %v", ErrThresholdsNotMet, len(u.verdict.Failures), u.verdict.Failures)
	}
	u.legacyDisabled = true
	return nil
}

func (u *Unit) LegacyDisabled() bool {
	if u == nil {
		return false
	}
	return u.legacyDisabled
}

func (u *Unit) Rollback() error {
	if u == nil {
		return fmt.Errorf("%w: unit is nil", ErrCutoverInvalid)
	}
	if !u.legacyDisabled {
		return fmt.Errorf("%w: legacy loading is already enabled", ErrCutoverInvalid)
	}
	u.legacyDisabled = false
	return nil
}
