package execution

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

var (
	onRateLimitAdmitted    func(dimensions []string)
	onRateLimitRejected    func(dimensions []string, reason string, retryAfterMs int64)
	onBackpressureRejected func(dimensions []string, reason string, retryAfterMs int64)
	onRateLimitWait        func(dimensions []string, waitMs int64)
)

func SetRateLimitObservabilityHooks(admitted func([]string), rejected func([]string, string, int64), backpressure func([]string, string, int64), wait func([]string, int64)) {
	onRateLimitAdmitted = admitted
	onRateLimitRejected = rejected
	onBackpressureRejected = backpressure
	onRateLimitWait = wait
}

type RateLimitDimension string

const (
	RateLimitGlobal       RateLimitDimension = "global"
	RateLimitTool         RateLimitDimension = "tool"
	RateLimitExtension    RateLimitDimension = "extension"
	RateLimitCharacter    RateLimitDimension = "character"
	RateLimitConversation RateLimitDimension = "conversation"
)

type RateLimitKey struct {
	Dimension      RateLimitDimension
	ToolID         string
	ExtensionID    string
	UserID         string
	CharacterID    string
	ConversationID string
}

type RateLimitSpec struct {
	Tokens   int
	Interval time.Duration
	Burst    int
}

type RateLimitPolicy struct {
	Global       RateLimitSpec
	PerTool      RateLimitSpec
	PerExtension RateLimitSpec
	PerCharacter RateLimitSpec
	PerConversation RateLimitSpec
	Backpressure BackpressurePolicy
	Enabled      bool
}

type BackpressureMode string

const (
	BackpressureReject BackpressureMode = "reject"
	BackpressureWait   BackpressureMode = "wait"
)

type BackpressurePolicy struct {
	Mode             BackpressureMode
	MaxWait          time.Duration
	MaxWaiters       int
	MaxWaitersPerKey int
}

type RateLimitDecision string

const (
	RateLimitAdmitted          RateLimitDecision = "admitted"
	RateLimitWaiting           RateLimitDecision = "waiting"
	RateLimitRejected          RateLimitDecision = "rejected"
	RateLimitBackpressureRejected RateLimitDecision = "backpressure_rejected"
)

type RateLimitAdmission struct {
	Decision           RateLimitDecision
	RetryAfter         time.Duration
	BlockingDimensions []RateLimitDimension
	WaitDuration       time.Duration
	Reason             string
}

type RateLimitTimer interface {
	C() <-chan time.Time
	Stop() bool
}

type RateLimitClock interface {
	Now() time.Time
	NewTimer(duration time.Duration) RateLimitTimer
}

type systemRateLimitClock struct{}

func (systemRateLimitClock) Now() time.Time { return time.Now().UTC() }

func (systemRateLimitClock) NewTimer(d time.Duration) RateLimitTimer {
	return systemTimer{Timer: time.NewTimer(d)}
}

type systemTimer struct{ *time.Timer }

func (t systemTimer) C() <-chan time.Time { return t.Timer.C }

type rateKeyType struct {
	dim          RateLimitDimension
	toolID       string
	extensionID  string
	userID       string
	characterID  string
	conversation string
}

type rateBucket struct {
	key       rateKeyType
	spec      RateLimitSpec
	tokens    float64
	lastRefill time.Time
	lastSeen   time.Time
}

type waiterRegistration struct {
	mu      sync.Mutex
	keys    []rateKeyType
	applied bool
}

func (w *waiterRegistration) Keys() []rateKeyType {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.keys
}

func (w *waiterRegistration) setApplied(keys []rateKeyType) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.keys = keys
	w.applied = true
}

type RateLimiter struct {
	mu             sync.Mutex
	policy         RateLimitPolicy
	clock          RateLimitClock
	buckets        map[rateKeyType]*rateBucket
	waiters        int
	waitersByKey   map[rateKeyType]int
	pruneScanLimit int
}

func NewRateLimiter(policy RateLimitPolicy) (*RateLimiter, error) {
	return NewRateLimiterWithClock(policy, systemRateLimitClock{})
}

func NewRateLimiterWithClock(policy RateLimitPolicy, clock RateLimitClock) (*RateLimiter, error) {
	if !policy.Enabled {
		return &RateLimiter{policy: policy, clock: clock}, nil
	}
	if err := validatePolicy(policy); err != nil {
		return nil, err
	}
	if clock == nil {
		clock = systemRateLimitClock{}
	}
	return &RateLimiter{
		policy:         policy,
		clock:          clock,
		buckets:        make(map[rateKeyType]*rateBucket),
		waitersByKey:   make(map[rateKeyType]int),
		pruneScanLimit: 64,
	}, nil
}

func validatePolicy(policy RateLimitPolicy) error {
	specs := map[string]RateLimitSpec{
		"global":       policy.Global,
		"perTool":      policy.PerTool,
		"perExtension": policy.PerExtension,
		"perCharacter": policy.PerCharacter,
		"perConversation": policy.PerConversation,
	}
	for name, spec := range specs {
		if spec.Tokens < 0 {
			return errors.New("rate_limit_policy_invalid: " + name + " tokens negative")
		}
		if spec.Interval < 0 {
			return errors.New("rate_limit_policy_invalid: " + name + " interval negative")
		}
		if spec.Tokens > 0 {
			if spec.Interval <= 0 {
				return errors.New("rate_limit_policy_invalid: " + name + " interval must be positive")
			}
			if spec.Burst <= 0 {
				return errors.New("rate_limit_policy_invalid: " + name + " burst must be positive")
			}
		}
	}
	switch policy.Backpressure.Mode {
	case BackpressureReject, BackpressureWait, "":
	default:
		return errors.New("rate_limit_policy_invalid: unknown backpressure mode")
	}
	if policy.Backpressure.Mode == BackpressureWait {
		if policy.Backpressure.MaxWait <= 0 {
			return errors.New("rate_limit_policy_invalid: MaxWait must be positive in wait mode")
		}
		if policy.Backpressure.MaxWaiters <= 0 {
			return errors.New("rate_limit_policy_invalid: MaxWaiters must be positive in wait mode")
		}
		if policy.Backpressure.MaxWaitersPerKey <= 0 {
			return errors.New("rate_limit_policy_invalid: MaxWaitersPerKey must be positive in wait mode")
		}
	}
	return nil
}

func (r *RateLimiter) Admit(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext) (RateLimitAdmission, error) {
	if !r.policy.Enabled {
		return RateLimitAdmission{Decision: RateLimitAdmitted}, nil
	}

	_, relevantKeys, err := r.prepareRelevantKeys(tool, inv)
	if err != nil {
		return RateLimitAdmission{
			Decision: RateLimitRejected,
			Reason:   "policy_invalid",
		}, err
	}

	if len(relevantKeys) == 0 {
		return RateLimitAdmission{Decision: RateLimitAdmitted}, nil
	}

	reg := &waiterRegistration{}

	for {
		if err := ctx.Err(); err != nil {
			r.unregister(reg)
			return RateLimitAdmission{
				Decision: rateDecisionFromContextError(err),
				Reason:   err.Error(),
			}, err
		}

		r.mu.Lock()
		now := r.clock.Now()
		buckets := r.refillLocked(relevantKeys, now)
		allReady, blocking := r.checkAllReadyLocked(buckets)
		if allReady {
			for _, b := range buckets {
				b.tokens -= 1
				b.lastSeen = now
			}
			r.tokensPruneScanLocked()
			r.mu.Unlock()
			if onRateLimitAdmitted != nil {
				onRateLimitAdmitted(dimNames(relevantKeys))
			}
			return RateLimitAdmission{Decision: RateLimitAdmitted}, nil
		}
		wait := r.computeWaitDuration(blocking)
		if r.policy.Backpressure.Mode == BackpressureReject {
			r.mu.Unlock()
			return RateLimitAdmission{
				Decision:           RateLimitRejected,
				RetryAfter:         wait,
				BlockingDimensions: dimsFromKeys(blocking),
				WaitDuration:       wait,
				Reason:             "rate_limited",
			}, nil
		}
		remainingDeadline := invocationDeadline(ctx)
		if remainingDeadline > 0 && wait > remainingDeadline {
			r.mu.Unlock()
			if onRateLimitRejected != nil {
				onRateLimitRejected(dimsToStrings(dimsFromKeys(blocking)), "deadline_before_refill", wait.Milliseconds())
			}
			return RateLimitAdmission{
				Decision:           RateLimitRejected,
				RetryAfter:         wait,
				BlockingDimensions: dimsFromKeys(blocking),
				WaitDuration:       wait,
				Reason:             "deadline_before_refill",
			}, nil
		}
		if wait > r.policy.Backpressure.MaxWait {
			r.mu.Unlock()
			if onBackpressureRejected != nil {
				onBackpressureRejected(dimsToStrings(dimsFromKeys(blocking)), "max_wait_exceeded", wait.Milliseconds())
			}
			return RateLimitAdmission{
				Decision:           RateLimitBackpressureRejected,
				RetryAfter:         wait,
				BlockingDimensions: dimsFromKeys(blocking),
				WaitDuration:       wait,
				Reason:             "max_wait_exceeded",
			}, nil
		}
		if !r.canRegisterWaiterLocked(relevantKeys) {
			r.mu.Unlock()
			if onBackpressureRejected != nil {
				onBackpressureRejected(dimsToStrings(dimsFromKeys(blocking)), "queue_full", wait.Milliseconds())
			}
			return RateLimitAdmission{
				Decision:           RateLimitBackpressureRejected,
				RetryAfter:         wait,
				BlockingDimensions: dimsFromKeys(blocking),
				WaitDuration:       wait,
				Reason:             "queue_full",
			}, nil
		}
		reg.setApplied(relevantKeys)
		regKeys := reg.Keys()
		for _, k := range regKeys {
			r.waitersByKey[k]++
		}
		r.waiters++
		r.mu.Unlock()

		timer := r.clock.NewTimer(clampWait(wait))
		select {
		case <-timer.C():
			timer.Stop()
			r.unregister(reg)
			continue
		case <-ctx.Done():
			timer.Stop()
			r.unregister(reg)
			return RateLimitAdmission{
				Decision: rateDecisionFromContextError(ctx.Err()),
				Reason:   ctx.Err().Error(),
			}, ctx.Err()
		}
	}
}

func (r *RateLimiter) unregister(reg *waiterRegistration) {
	if reg == nil {
		return
	}
	reg.mu.Lock()
	if !reg.applied {
		reg.mu.Unlock()
		return
	}
	keys := reg.keys
	reg.applied = false
	reg.mu.Unlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	r.waiters--
	for _, k := range keys {
		if c, ok := r.waitersByKey[k]; ok {
			if c <= 1 {
				delete(r.waitersByKey, k)
			} else {
				r.waitersByKey[k] = c - 1
			}
		}
	}
}

func (r *RateLimiter) canRegisterWaiterLocked(keys []rateKeyType) bool {
	if r.waiters >= r.policy.Backpressure.MaxWaiters {
		return false
	}
	for _, k := range keys {
		if r.waitersByKey[k] >= r.policy.Backpressure.MaxWaitersPerKey {
			return false
		}
	}
	return true
}

func (r *RateLimiter) prepareRelevantKeys(tool capability.ToolDefinition, inv capability.ToolInvocationContext) ([]RateLimitDimension, []rateKeyType, error) {
	var dims []RateLimitDimension
	var keys []rateKeyType

	if r.policy.Global.Tokens > 0 {
		dims = append(dims, RateLimitGlobal)
		keys = append(keys, rateKeyType{dim: RateLimitGlobal})
	}
	if r.policy.PerTool.Tokens > 0 {
		dims = append(dims, RateLimitTool)
		keys = append(keys, rateKeyType{dim: RateLimitTool, toolID: tool.ID})
	}
	if r.policy.PerExtension.Tokens > 0 && tool.ExtensionID != "" {
		dims = append(dims, RateLimitExtension)
		keys = append(keys, rateKeyType{dim: RateLimitExtension, extensionID: tool.ExtensionID})
	}
	if r.policy.PerCharacter.Tokens > 0 && inv.CharacterID != "" {
		dims = append(dims, RateLimitCharacter)
		keys = append(keys, rateKeyType{dim: RateLimitCharacter, userID: inv.UserID, characterID: inv.CharacterID})
	}
	if r.policy.PerConversation.Tokens > 0 && inv.ConversationID != "" {
		dims = append(dims, RateLimitConversation)
		keys = append(keys, rateKeyType{dim: RateLimitConversation, userID: inv.UserID, characterID: inv.CharacterID, conversation: inv.ConversationID})
	}
	return dims, keys, nil
}

func (r *RateLimiter) refillLocked(keys []rateKeyType, now time.Time) []*rateBucket {
	result := make([]*rateBucket, 0, len(keys))
	for _, k := range keys {
		b, ok := r.buckets[k]
		if !ok {
			spec := r.specFor(k)
			b = &rateBucket{
				key:        k,
				spec:       spec,
				tokens:     float64(spec.Burst),
				lastRefill: now,
				lastSeen:   now,
			}
			r.buckets[k] = b
			result = append(result, b)
			continue
		}
		elapsed := now.Sub(b.lastRefill)
		if elapsed < 0 {
			elapsed = 0
		}
		if elapsed > 0 && b.tokens < float64(b.spec.Burst) {
			interval := b.spec.Interval
			if interval > 0 && b.spec.Tokens > 0 {
				refill := float64(elapsed) / float64(interval) * float64(b.spec.Tokens)
				b.tokens = math.Min(float64(b.spec.Burst), b.tokens + refill)
			}
			b.lastRefill = now
		}
		b.lastSeen = now
		result = append(result, b)
	}
	return result
}

func (r *RateLimiter) checkAllReadyLocked(buckets []*rateBucket) (bool, []*rateBucket) {
	var blocking []*rateBucket
	for _, b := range buckets {
		if b.tokens < 1 {
			blocking = append(blocking, b)
		}
	}
	return len(blocking) == 0, blocking
}

func (r *RateLimiter) computeWaitDuration(blocking []*rateBucket) time.Duration {
	var maxWait time.Duration
	for _, b := range blocking {
		missing := 1.0 - b.tokens
		if missing <= 0 {
			continue
		}
		interval := b.spec.Interval
		tokens := b.spec.Tokens
		if interval <= 0 || tokens <= 0 {
			continue
		}
		waitSec := missing * float64(interval) / float64(tokens)
		d := time.Duration(waitSec * float64(time.Second))
		if d < time.Millisecond {
			d = time.Millisecond
		}
		if d > maxWait {
			maxWait = d
		}
	}
	return maxWait
}

func clampWait(d time.Duration) time.Duration {
	if d <= 0 {
		return time.Millisecond
	}
	return d
}

func (r *RateLimiter) tokensPruneScanLocked() {
	if len(r.buckets) == 0 {
		return
	}
	scanned := 0
	for k, b := range r.buckets {
		if scanned >= r.pruneScanLimit {
			break
		}
		scanned++
		if b.tokens >= float64(b.spec.Burst) {
			delete(r.buckets, k)
		}
	}
}

func (r *RateLimiter) specFor(k rateKeyType) RateLimitSpec {
	switch k.dim {
	case RateLimitGlobal:
		return r.policy.Global
	case RateLimitTool:
		return r.policy.PerTool
	case RateLimitExtension:
		return r.policy.PerExtension
	case RateLimitCharacter:
		return r.policy.PerCharacter
	case RateLimitConversation:
		return r.policy.PerConversation
	}
	return RateLimitSpec{}
}

func invocationDeadline(ctx context.Context) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return 0
	}
	remaining := time.Until(deadline)
	if remaining < 0 {
		return 0
	}
	return remaining
}

func rateDecisionFromContextError(err error) RateLimitDecision {
	if errors.Is(err, context.DeadlineExceeded) {
		return RateLimitRejected
	}
	return RateLimitRejected
}

func dimNames(keys []rateKeyType) []string {
	seen := make(map[RateLimitDimension]bool)
	var dims []string
	for _, k := range keys {
		if !seen[k.dim] {
			seen[k.dim] = true
			dims = append(dims, string(k.dim))
		}
	}
	return dims
}

func dimsFromKeys(buckets []*rateBucket) []RateLimitDimension {
	seen := make(map[RateLimitDimension]bool)
	var dims []RateLimitDimension
	for _, b := range buckets {
		if !seen[b.key.dim] {
			seen[b.key.dim] = true
			dims = append(dims, b.key.dim)
		}
	}
	return dims
}

func dimsToStrings(dims []RateLimitDimension) []string {
	out := make([]string, 0, len(dims))
	for _, d := range dims {
		out = append(out, string(d))
	}
	return out
}

type RateLimitSnapshot struct {
	Enabled         bool
	BucketCount     int
	Waiters         int
	BucketsByDimension map[RateLimitDimension]int
}

func (r *RateLimiter) Snapshot() RateLimitSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap := RateLimitSnapshot{
		Enabled:            r.policy.Enabled,
		BucketCount:        len(r.buckets),
		Waiters:            r.waiters,
		BucketsByDimension: make(map[RateLimitDimension]int),
	}
	for k := range r.buckets {
		snap.BucketsByDimension[k.dim]++
	}
	return snap
}
