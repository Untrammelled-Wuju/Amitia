package behavior

import (
	"context"
	"sync"

	"github.com/u-ai/backend/log"
)

type ProcessFunc func(ctx context.Context, event BehaviorEventEnvelope)

type charMailbox struct {
	ch     chan BehaviorEventEnvelope
	ctx    context.Context
	cancel context.CancelFunc
}

type Coordinator struct {
	capacity  int
	processFn ProcessFunc
	ctx       context.Context

	mu        sync.Mutex
	mailboxes map[string]*charMailbox
	wg        sync.WaitGroup

	stopOnce sync.Once
	stopped  bool
}

func NewCoordinator(capacity int, processFn ProcessFunc) *Coordinator {
	if capacity <= 0 {
		capacity = MailboxCapacity
	}
	return &Coordinator{
		capacity:  capacity,
		processFn: processFn,
		mailboxes: make(map[string]*charMailbox),
		ctx:       context.Background(),
	}
}

func (c *Coordinator) mailboxKey(userID, characterID string) string {
	return userID + ":" + characterID
}

func (c *Coordinator) Enqueue(userID, characterID string, event BehaviorEventEnvelope) error {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return NewBehaviorError(ErrCodeRulesetInvalid, "coordinator stopped")
	}
	key := c.mailboxKey(userID, characterID)
	mb, exists := c.mailboxes[key]
	if !exists {
		mb = c.createMailboxLocked(key)
	}
	c.mu.Unlock()

	select {
	case mb.ch <- event:
		return nil
	default:
		select {
		case mb.ch <- event:
			return nil
		case <-mb.ctx.Done():
			return NewBehaviorError(ErrCodeRulesetInvalid, "mailbox cancelled")
		}
	}
}

func (c *Coordinator) TryEnqueue(userID, characterID string, event BehaviorEventEnvelope) bool {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return false
	}
	key := c.mailboxKey(userID, characterID)
	mb, exists := c.mailboxes[key]
	if !exists {
		mb = c.createMailboxLocked(key)
	}
	c.mu.Unlock()

	select {
	case mb.ch <- event:
		return true
	default:
		return false
	}
}

func (c *Coordinator) createMailboxLocked(key string) *charMailbox {
	ctx, cancel := context.WithCancel(c.ctx)
	mb := &charMailbox{
		ch:     make(chan BehaviorEventEnvelope, c.capacity),
		ctx:    ctx,
		cancel: cancel,
	}
	c.mailboxes[key] = mb

	c.wg.Add(1)
	go c.runMailbox(key, mb, ctx)
	return mb
}

func (c *Coordinator) runMailbox(key string, mb *charMailbox, ctx context.Context) {
	defer c.wg.Done()

	for {
		select {
		case <-ctx.Done():
			c.drainMailbox(key, mb)
			return
		case event := <-mb.ch:
			safeProcess(c.processFn, ctx, event)
		}
	}
}

func (c *Coordinator) drainMailbox(key string, mb *charMailbox) {
	for {
		select {
		case event := <-mb.ch:
			safeProcess(c.processFn, context.Background(), event)
		default:
			return
		}
	}
}

func (c *Coordinator) Stop() {
	c.stopOnce.Do(func() {
		c.mu.Lock()
		c.stopped = true
		for _, mb := range c.mailboxes {
			mb.cancel()
		}
		c.mu.Unlock()
		c.wg.Wait()
	})
}

func (c *Coordinator) RemoveMailbox(userID, characterID string) {
	key := c.mailboxKey(userID, characterID)
	c.mu.Lock()
	mb, ok := c.mailboxes[key]
	if ok {
		delete(c.mailboxes, key)
	}
	c.mu.Unlock()
	if ok {
		mb.cancel()
	}
}

func (c *Coordinator) MailboxDepth(userID, characterID string) int {
	key := c.mailboxKey(userID, characterID)
	c.mu.Lock()
	mb, ok := c.mailboxes[key]
	c.mu.Unlock()
	if !ok {
		return 0
	}
	return len(mb.ch)
}

func (c *Coordinator) ActiveMailboxes() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.mailboxes)
}

func safeProcess(fn ProcessFunc, ctx context.Context, event BehaviorEventEnvelope) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("behavior coordinator: panic in process function", map[string]interface{}{
				"panic":       r,
				"eventId":     event.EventID,
				"eventType":   event.EventType,
				"characterId": event.CharacterID,
			})
		}
	}()
	fn(ctx, event)
}
