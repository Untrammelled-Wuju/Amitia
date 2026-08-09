package rpc

import (
	"container/list"
	"encoding/json"
	"sync"
	"time"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type CompletedRequest struct {
	Key         RequestKey
	Fingerprint RequestFingerprint
	Response    protocol.Envelope
	FinishedAt  time.Time
}

type CompletedResponseCache struct {
	mu           sync.Mutex
	maxEntries   int
	retentionTTL time.Duration
	entries      map[RequestKey]*list.Element
	order        *list.List
}

type CompletedResponseCacheConfig struct {
	MaxEntries   int
	RetentionTTL time.Duration
}

func DefaultCompletedResponseCacheConfig() CompletedResponseCacheConfig {
	return CompletedResponseCacheConfig{
		MaxEntries:   1024,
		RetentionTTL: 5 * time.Minute,
	}
}

func NewCompletedResponseCache(config CompletedResponseCacheConfig) *CompletedResponseCache {
	if config.MaxEntries <= 0 {
		config.MaxEntries = 1024
	}
	if config.RetentionTTL <= 0 {
		config.RetentionTTL = 5 * time.Minute
	}
	return &CompletedResponseCache{
		maxEntries:   config.MaxEntries,
		retentionTTL: config.RetentionTTL,
		entries:      make(map[RequestKey]*list.Element),
		order:        list.New(),
	}
}

func (c *CompletedResponseCache) Save(req CompletedRequest) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, exists := c.entries[req.Key]; exists {
		c.order.MoveToBack(elem)
		elem.Value = req
		return
	}

	if c.order.Len() >= c.maxEntries {
		front := c.order.Front()
		if front != nil {
			c.order.Remove(front)
			delete(c.entries, front.Value.(CompletedRequest).Key)
		}
	}

	elem := c.order.PushBack(req)
	c.entries[req.Key] = elem
}

func (c *CompletedResponseCache) Lookup(key RequestKey, fp RequestFingerprint) (protocol.Envelope, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, exists := c.entries[key]
	if !exists {
		return protocol.Envelope{}, false
	}

	req := elem.Value.(CompletedRequest)

	if time.Since(req.FinishedAt) > c.retentionTTL {
		c.order.Remove(elem)
		delete(c.entries, key)
		return protocol.Envelope{}, false
	}

	if req.Fingerprint != fp {
		return protocol.Envelope{}, false
	}

	return cloneEnvelope(req.Response), true
}

func (c *CompletedResponseCache) Invalidate(key RequestKey) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, exists := c.entries[key]; exists {
		c.order.Remove(elem)
		delete(c.entries, key)
	}
}

func (c *CompletedResponseCache) Cleanup(before time.Time) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := 0
	for elem := c.order.Front(); elem != nil; {
		req := elem.Value.(CompletedRequest)
		next := elem.Next()
		if req.FinishedAt.Before(before) {
			c.order.Remove(elem)
			delete(c.entries, req.Key)
			count++
		}
		elem = next
	}

	return count
}

func (c *CompletedResponseCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

func (c *CompletedResponseCache) Entries() int {
	return c.Len()
}

func cloneEnvelope(src protocol.Envelope) protocol.Envelope {
	dst := src
	if src.Payload != nil {
		dst.Payload = cloneRawMessage(src.Payload)
	}
	if src.Metadata != nil {
		dst.Metadata = make(map[string]json.RawMessage, len(src.Metadata))
		for k, v := range src.Metadata {
			dst.Metadata[k] = cloneRawMessage(v)
		}
	}
	return dst
}
