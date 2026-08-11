package execution

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type ConcurrencyDimension string

const (
	ConcurrencyGlobal       ConcurrencyDimension = "global"
	ConcurrencyExtension    ConcurrencyDimension = "extension"
	ConcurrencyTool         ConcurrencyDimension = "tool"
	ConcurrencyCharacter    ConcurrencyDimension = "character"
	ConcurrencyConversation ConcurrencyDimension = "conversation"
)

type ConcurrencyKey struct {
	Dimension ConcurrencyDimension
	ID        string
}

type ConcurrencyPolicy struct {
	GlobalLimit          int
	PerToolLimit         int
	PerExtensionLimit    int
	PerCharacterLimit    int
	PerConversationLimit int
}

type EffectiveConcurrencyLimits struct {
	Global       int
	Tool         int
	Extension    int
	Character    int
	Conversation int
}

type ConcurrencyLimitResolver interface {
	Resolve(tool capability.ToolDefinition, inv capability.ToolInvocationContext) (EffectiveConcurrencyLimits, error)
}

type DefaultConcurrencyLimitResolver struct {
	Policy ConcurrencyPolicy
}

func NewConcurrencyLimitResolver(policy ConcurrencyPolicy) ConcurrencyLimitResolver {
	return &DefaultConcurrencyLimitResolver{Policy: policy}
}

func positiveMin(a, b int) int {
	switch {
	case a <= 0 && b <= 0:
		return 0
	case a <= 0:
		return b
	case b <= 0:
		return a
	default:
		if a < b {
			return a
		}
		return b
	}
}

func (r *DefaultConcurrencyLimitResolver) Resolve(tool capability.ToolDefinition, inv capability.ToolInvocationContext) (EffectiveConcurrencyLimits, error) {
	if r.Policy.GlobalLimit < 0 {
		return EffectiveConcurrencyLimits{}, errors.New("invalid global concurrency limit")
	}
	if r.Policy.PerToolLimit < 0 {
		return EffectiveConcurrencyLimits{}, errors.New("invalid per-tool concurrency limit")
	}
	if r.Policy.PerExtensionLimit < 0 {
		return EffectiveConcurrencyLimits{}, errors.New("invalid per-extension concurrency limit")
	}
	if r.Policy.PerCharacterLimit < 0 {
		return EffectiveConcurrencyLimits{}, errors.New("invalid per-character concurrency limit")
	}
	if r.Policy.PerConversationLimit < 0 {
		return EffectiveConcurrencyLimits{}, errors.New("invalid per-conversation concurrency limit")
	}
	toolMax := tool.ExecutionPolicy.MaxConcurrency
	effTool := positiveMin(r.Policy.PerToolLimit, toolMax)
	return EffectiveConcurrencyLimits{
		Global:       r.Policy.GlobalLimit,
		Tool:         effTool,
		Extension:    r.Policy.PerExtensionLimit,
		Character:    r.Policy.PerCharacterLimit,
		Conversation: r.Policy.PerConversationLimit,
	}, nil
}

type ConcurrencyPermit struct {
	Key   ConcurrencyKey
	Limit int
}

type ConcurrencyLease struct {
	controller *ConcurrencyController
	permits    []ConcurrencyPermit
	acquiredAt time.Time
	once       sync.Once
}

func newConcurrencyLease(controller *ConcurrencyController, permits []ConcurrencyPermit, acquiredAt time.Time) *ConcurrencyLease {
	return &ConcurrencyLease{
		controller: controller,
		permits:    permits,
		acquiredAt: acquiredAt,
	}
}

func (l *ConcurrencyLease) Release() {
	if l == nil {
		return
	}
	l.once.Do(func() {
		l.controller.release(l.permits, l.acquiredAt)
	})
}

func (l *ConcurrencyLease) Permits() []ConcurrencyPermit {
	if l == nil {
		return nil
	}
	return l.permits
}

type concurrencyBucketKey struct {
	dimension ConcurrencyDimension
	value     string
}

type concurrencyBucket struct {
	limit int
	inUse int
}

type ConcurrencyController struct {
	mu       sync.Mutex
	policy   ConcurrencyPolicy
	resolver ConcurrencyLimitResolver
	buckets  map[concurrencyBucketKey]*concurrencyBucket
	notify   chan struct{}
	seq      uint64
}

func NewConcurrencyController(policy ConcurrencyPolicy) (*ConcurrencyController, error) {
	if policy.GlobalLimit < 0 || policy.PerToolLimit < 0 || policy.PerExtensionLimit < 0 || policy.PerCharacterLimit < 0 || policy.PerConversationLimit < 0 {
		return nil, errors.New("concurrency policy contains negative limit")
	}
	return &ConcurrencyController{
		policy:   policy,
		resolver: NewConcurrencyLimitResolver(policy),
		buckets:  make(map[concurrencyBucketKey]*concurrencyBucket),
		notify:   make(chan struct{}),
	}, nil
}

func (c *ConcurrencyController) Policy() ConcurrencyPolicy {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.policy
}

func (c *ConcurrencyController) Acquire(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext) (*ConcurrencyLease, error) {
	limits, err := c.resolver.Resolve(tool, inv)
	if err != nil {
		return nil, err
	}

	permitKeys := buildConcurrencyKeys(tool, inv, limits)

	waitStarted := false
	var waitSince time.Time

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		c.mu.Lock()
		if c.canAcquireLocked(permitKeys) {
			permits := c.acquireLocked(permitKeys)
			now := time.Now()
			c.mu.Unlock()
		if waitStarted {
			c.recordWaitDuration(permits, now.Sub(waitSince))
		}
			c.onAcquired(permits)
			return newConcurrencyLease(c, permits, now), nil
		}

		notify := c.notify
		if !waitStarted {
			waitSince = time.Now()
			waitStarted = true
		}
		c.mu.Unlock()

		select {
		case <-notify:
			continue
		case <-ctx.Done():
			if waitStarted {
				return nil, ctx.Err()
			}
			return nil, ctx.Err()
		}
	}
}

func (c *ConcurrencyController) release(permits []ConcurrencyPermit, acquiredAt time.Time) {
	c.mu.Lock()
	for _, p := range permits {
		c.releaseLocked(p)
	}
	c.cleanupBucketsLocked(permits)
	oldNotify := c.notify
	c.notify = make(chan struct{})
	close(oldNotify)
	c.mu.Unlock()
	c.onReleased(permits, time.Since(acquiredAt))
}

func (c *ConcurrencyController) canAcquireLocked(keys []ConcurrencyPermit) bool {
	for _, k := range keys {
		if k.Limit <= 0 {
			continue
		}
		bk := concurrencyBucketKey{dimension: k.Key.Dimension, value: k.Key.ID}
		bucket := c.buckets[bk]
		if bucket != nil && bucket.inUse >= bucket.limit {
			return false
		}
	}
	return true
}

func (c *ConcurrencyController) acquireLocked(keys []ConcurrencyPermit) []ConcurrencyPermit {
	result := make([]ConcurrencyPermit, 0, len(keys))
	for _, k := range keys {
		if k.Limit <= 0 {
			continue
		}
		bk := concurrencyBucketKey{dimension: k.Key.Dimension, value: k.Key.ID}
		bucket := c.buckets[bk]
		if bucket == nil {
			bucket = &concurrencyBucket{limit: k.Limit}
			c.buckets[bk] = bucket
		}
		bucket.inUse++
		result = append(result, k)
	}
	return result
}

func (c *ConcurrencyController) releaseLocked(p ConcurrencyPermit) {
	if p.Limit <= 0 {
		return
	}
	bk := concurrencyBucketKey{dimension: p.Key.Dimension, value: p.Key.ID}
	bucket := c.buckets[bk]
	if bucket == nil {
		return
	}
	if bucket.inUse > 0 {
		bucket.inUse--
	}
}

func (c *ConcurrencyController) cleanupBucketsLocked(permits []ConcurrencyPermit) {
	for _, p := range permits {
		if p.Limit <= 0 {
			continue
		}
		bk := concurrencyBucketKey{dimension: p.Key.Dimension, value: p.Key.ID}
		if b := c.buckets[bk]; b != nil && b.inUse <= 0 {
			delete(c.buckets, bk)
		}
	}
}

func buildConcurrencyKeys(tool capability.ToolDefinition, inv capability.ToolInvocationContext, limits EffectiveConcurrencyLimits) []ConcurrencyPermit {
	keys := make([]ConcurrencyPermit, 0, 5)
	if limits.Global > 0 {
		keys = append(keys, ConcurrencyPermit{
			Key:   ConcurrencyKey{Dimension: ConcurrencyGlobal, ID: ""},
			Limit: limits.Global,
		})
	}
	if limits.Extension > 0 && tool.ExtensionID != "" {
		keys = append(keys, ConcurrencyPermit{
			Key:   ConcurrencyKey{Dimension: ConcurrencyExtension, ID: tool.ExtensionID},
			Limit: limits.Extension,
		})
	}
	if limits.Tool > 0 {
		keys = append(keys, ConcurrencyPermit{
			Key:   ConcurrencyKey{Dimension: ConcurrencyTool, ID: tool.ID},
			Limit: limits.Tool,
		})
	}
	if limits.Character > 0 && inv.CharacterID != "" {
		keys = append(keys, ConcurrencyPermit{
			Key:   ConcurrencyKey{Dimension: ConcurrencyCharacter, ID: characterConcurrencyID(inv)},
			Limit: limits.Character,
		})
	}
	if limits.Conversation > 0 && inv.ConversationID != "" {
		keys = append(keys, ConcurrencyPermit{
			Key:   ConcurrencyKey{Dimension: ConcurrencyConversation, ID: conversationConcurrencyID(inv)},
			Limit: limits.Conversation,
		})
	}
	return keys
}

func characterConcurrencyID(inv capability.ToolInvocationContext) string {
	return inv.UserID + "\x00" + inv.CharacterID
}

func conversationConcurrencyID(inv capability.ToolInvocationContext) string {
	return inv.UserID + "\x00" + inv.CharacterID + "\x00" + inv.ConversationID
}

func (c *ConcurrencyController) Snapshot() ConcurrencySnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	snap := ConcurrencySnapshot{
		BucketsByDimension: make(map[ConcurrencyDimension]int),
	}
	for k, b := range c.buckets {
		if b.inUse > 0 {
			snap.ActiveBuckets++
			snap.BucketsByDimension[k.dimension]++
		}
		if k.dimension == ConcurrencyGlobal {
			snap.GlobalInUse = b.inUse
		}
	}
	return snap
}

type ConcurrencySnapshot struct {
	GlobalInUse      int
	ActiveBuckets    int
	BucketsByDimension map[ConcurrencyDimension]int
}

var (
	onAcquired     func(permits []ConcurrencyPermit)
	onReleased     func(permits []ConcurrencyPermit, waitDuration time.Duration)
	onWaitDuration func(permits []ConcurrencyPermit, waitDuration time.Duration)
)

func (c *ConcurrencyController) onAcquired(permits []ConcurrencyPermit) {
	if onAcquired != nil {
		onAcquired(permits)
	}
}

func (c *ConcurrencyController) onReleased(permits []ConcurrencyPermit, waitDuration time.Duration) {
	if onReleased != nil {
		onReleased(permits, waitDuration)
	}
}

func (c *ConcurrencyController) recordWaitDuration(permits []ConcurrencyPermit, waitDuration time.Duration) {
	if onWaitDuration != nil {
		onWaitDuration(permits, waitDuration)
	}
}

func SetConcurrencyObservabilityHooks(onAcq func(permits []ConcurrencyPermit), onRel func(permits []ConcurrencyPermit, waitDuration time.Duration), onWait func(permits []ConcurrencyPermit, waitDuration time.Duration)) {
	onAcquired = onAcq
	onReleased = onRel
	onWaitDuration = onWait
}
