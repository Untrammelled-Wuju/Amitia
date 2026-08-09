package stream

import (
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type OverflowPolicy string

const (
	OverflowReject     OverflowPolicy = "reject"
	OverflowDropOldest OverflowPolicy = "drop_oldest"
	OverflowDropNewest OverflowPolicy = "drop_newest"
	OverflowCoalesce   OverflowPolicy = "coalesce"
	OverflowBlock      OverflowPolicy = "block"
)

type ResumePolicy string

const (
	ResumeNone          ResumePolicy = "none"
	ResumeLatest        ResumePolicy = "latest"
	ResumeBoundedReplay ResumePolicy = "bounded_replay"
)

type RateLimitPolicy struct {
	MessagesPerSecond int
	Burst             int
}

func (p RateLimitPolicy) Enabled() bool {
	return p.MessagesPerSecond > 0
}

func (p RateLimitPolicy) Validate() error {
	if p.MessagesPerSecond < 0 {
		return domain.NewHostError(domain.ErrInvalidArgument, "rate limit: messages per second must not be negative")
	}
	if p.Burst < 0 {
		return domain.NewHostError(domain.ErrInvalidArgument, "rate limit: burst must not be negative")
	}
	if p.MessagesPerSecond > 0 && p.Burst == 0 {
		p.Burst = p.MessagesPerSecond
	}
	return nil
}

type StreamPolicy struct {
	QueueCapacity  int
	ReplayCapacity int
	MaxReplayBytes int64
	Overflow       OverflowPolicy
	RateLimit      RateLimitPolicy
	Resume         ResumePolicy
	BlockTimeout   time.Duration
}

func (p StreamPolicy) Validate() error {
	if p.QueueCapacity <= 0 {
		return domain.NewHostError(domain.ErrInvalidArgument, "stream policy: queue capacity must be positive")
	}
	if p.ReplayCapacity < 0 {
		return domain.NewHostError(domain.ErrInvalidArgument, "stream policy: replay capacity must not be negative")
	}
	if p.MaxReplayBytes < 0 {
		return domain.NewHostError(domain.ErrInvalidArgument, "stream policy: max replay bytes must not be negative")
	}
	switch p.Overflow {
	case OverflowReject, OverflowDropOldest, OverflowDropNewest, OverflowCoalesce, OverflowBlock:
	default:
		return domain.NewHostError(domain.ErrInvalidArgument, "stream policy: invalid overflow policy")
	}
	if p.BlockTimeout < 0 {
		return domain.NewHostError(domain.ErrInvalidArgument, "stream policy: block timeout must not be negative")
	}
	if err := p.RateLimit.Validate(); err != nil {
		return err
	}
	return nil
}

type PolicyInput struct {
	Kind      domain.ChannelKind
	Frequency *protocol.FrequencyHint
}

type PolicyResolver interface {
	Resolve(input PolicyInput) StreamPolicy
}

var defaultQueueCapacities = map[domain.ChannelKind]int{
	domain.ChannelKindEvent:  1024,
	domain.ChannelKindState:  64,
	domain.ChannelKindLog:    512,
	domain.ChannelKindMetric: 256,
	domain.ChannelKindBinary: 128,
	domain.ChannelKindCustom: 256,
}

var defaultReplayCapacities = map[domain.ChannelKind]int{
	domain.ChannelKindEvent:  256,
	domain.ChannelKindState:  1,
	domain.ChannelKindLog:    128,
	domain.ChannelKindMetric: 64,
	domain.ChannelKindBinary: 64,
	domain.ChannelKindCustom: 0,
}

var defaultOverflowPolicies = map[domain.ChannelKind]OverflowPolicy{
	domain.ChannelKindEvent:  OverflowReject,
	domain.ChannelKindState:  OverflowCoalesce,
	domain.ChannelKindLog:    OverflowDropOldest,
	domain.ChannelKindMetric: OverflowDropOldest,
	domain.ChannelKindBinary: OverflowDropOldest,
	domain.ChannelKindCustom: OverflowReject,
}

var defaultResumePolicies = map[domain.ChannelKind]ResumePolicy{
	domain.ChannelKindEvent:  ResumeBoundedReplay,
	domain.ChannelKindState:  ResumeLatest,
	domain.ChannelKindLog:    ResumeBoundedReplay,
	domain.ChannelKindMetric: ResumeLatest,
	domain.ChannelKindBinary: ResumeBoundedReplay,
	domain.ChannelKindCustom: ResumeNone,
}

var frequencyQueueMultipliers = map[protocol.FrequencyHint]float64{
	protocol.FrequencyHintLow:      0.5,
	protocol.FrequencyHintNormal:   1.0,
	protocol.FrequencyHintHigh:     2.0,
	protocol.FrequencyHintRealtime: 1.0,
}

type defaultPolicyResolver struct {
	maxQueueCapacity  int
	maxReplayCapacity int
	maxReplayBytes    int64
	rateLimit         RateLimitPolicy
}

func NewPolicyResolver() PolicyResolver {
	return &defaultPolicyResolver{
		maxQueueCapacity:  8192,
		maxReplayCapacity: 1024,
		maxReplayBytes:    64 * 1024 * 1024,
	}
}

func NewPolicyResolverWithLimits(maxQueue, maxReplay int, maxReplayBytes int64, rateLimit RateLimitPolicy) PolicyResolver {
	return &defaultPolicyResolver{
		maxQueueCapacity:  maxQueue,
		maxReplayCapacity: maxReplay,
		maxReplayBytes:    maxReplayBytes,
		rateLimit:         rateLimit,
	}
}

func (r *defaultPolicyResolver) Resolve(input PolicyInput) StreamPolicy {
	queueCap := defaultQueueCapacities[input.Kind]
	replayCap := defaultReplayCapacities[input.Kind]
	overflow := defaultOverflowPolicies[input.Kind]
	resume := defaultResumePolicies[input.Kind]

	if input.Frequency != nil {
		multiplier, ok := frequencyQueueMultipliers[*input.Frequency]
		if ok {
			queueCap = int(float64(queueCap) * multiplier)
		}
	}

	if queueCap > r.maxQueueCapacity {
		queueCap = r.maxQueueCapacity
	}
	if queueCap < 1 {
		queueCap = 1
	}

	if replayCap > r.maxReplayCapacity {
		replayCap = r.maxReplayCapacity
	}

	return StreamPolicy{
		QueueCapacity:  queueCap,
		ReplayCapacity: replayCap,
		MaxReplayBytes: r.maxReplayBytes,
		Overflow:       overflow,
		RateLimit:      r.rateLimit,
		Resume:         resume,
	}
}
