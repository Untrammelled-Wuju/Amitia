package proactive

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type DeliveryStatus string

const (
	DeliveryStatusPending   DeliveryStatus = "pending"
	DeliveryStatusSent      DeliveryStatus = "sent"
	DeliveryStatusDelivered DeliveryStatus = "delivered"
	DeliveryStatusRead      DeliveryStatus = "read"
	DeliveryStatusFailed    DeliveryStatus = "failed"
	DeliveryStatusUnknown   DeliveryStatus = "unknown"
)

func (s DeliveryStatus) IsTerminal() bool {
	switch s {
	case DeliveryStatusDelivered, DeliveryStatusRead, DeliveryStatusFailed:
		return true
	default:
		return false
	}
}

var validTransitions = map[DeliveryStatus][]DeliveryStatus{
	DeliveryStatusPending:   {DeliveryStatusSent, DeliveryStatusFailed},
	DeliveryStatusSent:      {DeliveryStatusDelivered, DeliveryStatusFailed, DeliveryStatusUnknown},
	DeliveryStatusUnknown:   {DeliveryStatusDelivered, DeliveryStatusFailed, DeliveryStatusRead},
	DeliveryStatusDelivered: {DeliveryStatusRead},
	DeliveryStatusRead:      {},
	DeliveryStatusFailed:    {},
}

func TransitionDelivery(from, to DeliveryStatus) bool {
	valid, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, v := range valid {
		if v == to {
			return true
		}
	}
	return false
}

type DeliveryRecord struct {
	ID             string         `json:"id"`
	CorrelationID  string         `json:"correlationId"`
	CharacterID    string         `json:"characterId"`
	ConversationID string         `json:"conversationId"`
	Channel        string         `json:"channel"`
	Status         DeliveryStatus `json:"status"`
	StatusHistory  []StatusChange `json:"statusHistory"`
	ContentHash    string         `json:"contentHash"`
	RetryCount     int            `json:"retryCount"`
	LastAttemptAt  time.Time      `json:"lastAttemptAt"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
	mu             sync.Mutex
}

type StatusChange struct {
	From      DeliveryStatus `json:"from"`
	To        DeliveryStatus `json:"to"`
	Timestamp time.Time      `json:"timestamp"`
	Reason    string         `json:"reason"`
}

func (r *DeliveryRecord) TransitionTo(newStatus DeliveryStatus, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !TransitionDelivery(r.Status, newStatus) {
		return fmt.Errorf("illegal status transition: %s → %s", r.Status, newStatus)
	}

	now := time.Now()
	change := StatusChange{
		From:      r.Status,
		To:        newStatus,
		Timestamp: now,
		Reason:    reason,
	}
	r.Status = newStatus
	r.StatusHistory = append(r.StatusHistory, change)
	r.UpdatedAt = now
	return nil
}

func (r *DeliveryRecord) ConfirmWindow() time.Duration {
	base := 3 * time.Minute
	retryExtra := time.Duration(r.RetryCount) * 30 * time.Second
	window := base + retryExtra
	if window > 10*time.Minute {
		window = 10 * time.Minute
	}
	return window
}

func (r *DeliveryRecord) IsUnconfirmed(now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Status.IsTerminal() {
		return false
	}
	window := r.ConfirmWindow()
	return now.Sub(r.LastAttemptAt) < window
}

func (r *DeliveryRecord) MarkUnknown(now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Status != DeliveryStatusSent {
		return false
	}
	if now.Sub(r.LastAttemptAt) < r.ConfirmWindow() {
		return false
	}
	change := StatusChange{
		From:      r.Status,
		To:        DeliveryStatusUnknown,
		Timestamp: now,
		Reason:    "confirmation_window_expired",
	}
	r.Status = DeliveryStatusUnknown
	r.StatusHistory = append(r.StatusHistory, change)
	r.UpdatedAt = now
	return true
}

type DedupManager struct {
	records     map[string]map[string]*DeliveryRecord
	contentHash map[string]time.Time
	mu          sync.RWMutex
	ttl         time.Duration
}

var GlobalDedupManager = &DedupManager{
	records:     make(map[string]map[string]*DeliveryRecord),
	contentHash: make(map[string]time.Time),
	ttl:         30 * time.Minute,
}

func GenerateCorrelationID(characterID, ruleID string, content string) string {
	seed := fmt.Sprintf("%s:%s:%s", characterID, ruleID, content)
	hash := sha256.Sum256([]byte(seed))
	return "corr-" + hex.EncodeToString(hash[:16])
}

func (m *DedupManager) IsDuplicate(correlationID, channel string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if chMap, ok := m.records[correlationID]; ok {
		if record, ok2 := chMap[channel]; ok2 {
			return record.Status == DeliveryStatusSent ||
				record.Status == DeliveryStatusDelivered ||
				record.Status == DeliveryStatusRead
		}
	}
	return false
}

func (m *DedupManager) HasSentAnyChannel(correlationID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if chMap, ok := m.records[correlationID]; ok {
		for _, record := range chMap {
			if record.Status == DeliveryStatusSent ||
				record.Status == DeliveryStatusDelivered ||
				record.Status == DeliveryStatusRead {
				return true
			}
		}
	}
	return false
}

func (m *DedupManager) RecordDelivery(correlationID, characterID, conversationID, channel, content string) *DeliveryRecord {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.records[correlationID] == nil {
		m.records[correlationID] = make(map[string]*DeliveryRecord)
	}
	now := time.Now()
	record := &DeliveryRecord{
		ID:             uuid.New().String(),
		CorrelationID:  correlationID,
		CharacterID:    characterID,
		ConversationID: conversationID,
		Channel:        channel,
		Status:         DeliveryStatusPending,
		ContentHash:    hashContent(content),
		LastAttemptAt:  now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	record.StatusHistory = []StatusChange{
		{From: "", To: DeliveryStatusPending, Timestamp: now, Reason: "created"},
	}
	m.records[correlationID][channel] = record

	ch := record.ContentHash
	m.contentHash[ch] = now

	return record
}

func (m *DedupManager) MarkSent(correlationID, channel string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.records[correlationID] == nil {
		return fmt.Errorf("correlation %s not found", correlationID)
	}
	record, ok := m.records[correlationID][channel]
	if !ok {
		return fmt.Errorf("correlation %s channel %s not found", correlationID, channel)
	}
	return record.TransitionTo(DeliveryStatusSent, "message_sent")
}

func (m *DedupManager) MarkDelivered(correlationID, channel string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.records[correlationID] == nil {
		return fmt.Errorf("correlation %s not found", correlationID)
	}
	record, ok := m.records[correlationID][channel]
	if !ok {
		return fmt.Errorf("correlation %s channel %s not found", correlationID, channel)
	}
	return record.TransitionTo(DeliveryStatusDelivered, "message_delivered")
}

func (m *DedupManager) MarkFailed(correlationID, channel string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.records[correlationID] == nil {
		return fmt.Errorf("correlation %s not found", correlationID)
	}
	record, ok := m.records[correlationID][channel]
	if !ok {
		return fmt.Errorf("correlation %s channel %s not found", correlationID, channel)
	}
	return record.TransitionTo(DeliveryStatusFailed, "delivery_failed")
}

func (m *DedupManager) MarkRead(correlationID, channel string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.records[correlationID] == nil {
		return fmt.Errorf("correlation %s not found", correlationID)
	}
	record, ok := m.records[correlationID][channel]
	if !ok {
		return fmt.Errorf("correlation %s channel %s not found", correlationID, channel)
	}
	return record.TransitionTo(DeliveryStatusRead, "message_read")
}

func (m *DedupManager) GetRecord(correlationID, channel string) *DeliveryRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.records[correlationID] == nil {
		return nil
	}
	return m.records[correlationID][channel]
}

func (m *DedupManager) CleanExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	cleaned := 0
	for corrID, chMap := range m.records {
		for ch, record := range chMap {
			if now.Sub(record.CreatedAt) > m.ttl {
				delete(chMap, ch)
				cleaned++
			}
		}
		if len(chMap) == 0 {
			delete(m.records, corrID)
		}
	}
	for hash, ts := range m.contentHash {
		if now.Sub(ts) > m.ttl {
			delete(m.contentHash, hash)
			cleaned++
		}
	}
	if cleaned > 0 {
		log.Printf("[Dedup] cleaned %d expired records", cleaned)
	}
	return cleaned
}

func (m *DedupManager) ResolveUnknown() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	resolved := 0
	for _, chMap := range m.records {
		for _, record := range chMap {
			record.mu.Lock()
			status := record.Status
			record.mu.Unlock()
			if status != DeliveryStatusSent {
				continue
			}
			if record.MarkUnknown(now) {
				resolved++
				log.Printf("[Dedup] resolved unknown delivery corr=%s ch=%s", record.CorrelationID, record.Channel)
			}
		}
	}
	return resolved
}

func (m *DedupManager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = make(map[string]map[string]*DeliveryRecord)
	m.contentHash = make(map[string]time.Time)
}

func hashContent(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:12])
}

func DeliverableChannels(targetChannel string, seen map[string]bool) []string {
	channels := strings.FieldsFunc(targetChannel, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '|'
	})
	var result []string
	seenLocal := make(map[string]bool)
	for _, ch := range channels {
		ch = strings.TrimSpace(ch)
		if ch == "" {
			continue
		}
		if ch == "all" {
			for _, c := range []string{"web", "wechat", "qq"} {
				if !seen[c] && !seenLocal[c] {
					result = append(result, c)
					seenLocal[c] = true
				}
			}
			continue
		}
		if !seen[ch] && !seenLocal[ch] {
			result = append(result, ch)
			seenLocal[ch] = true
		}
	}
	return result
}
