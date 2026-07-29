package event

import (
	"fmt"
	"sort"
	"sync"
)

type PartitionKey string

type OrderingCoordinator struct {
	mu          sync.Mutex
	partitions  map[string]*partitionQueue
	maxInFlight int
}

type partitionQueue struct {
	partitionKey string
	pending      []string
	leased       map[string]bool
}

func NewOrderingCoordinator(maxInFlight int) *OrderingCoordinator {
	if maxInFlight <= 0 {
		maxInFlight = 1
	}
	return &OrderingCoordinator{
		partitions:  make(map[string]*partitionQueue),
		maxInFlight: maxInFlight,
	}
}

func (o *OrderingCoordinator) AcquireSlot(partitionKey string, deliveryID string) bool {
	if partitionKey == "" {
		return true
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	pq, ok := o.partitions[partitionKey]
	if !ok {
		pq = &partitionQueue{
			partitionKey: partitionKey,
			leased:       make(map[string]bool),
		}
		o.partitions[partitionKey] = pq
	}
	if len(pq.leased) >= o.maxInFlight {
		pq.pending = append(pq.pending, deliveryID)
		return false
	}
	pq.leased[deliveryID] = true
	return true
}

func (o *OrderingCoordinator) ReleaseSlot(partitionKey string, deliveryID string) {
	if partitionKey == "" {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	pq, ok := o.partitions[partitionKey]
	if !ok {
		return
	}
	delete(pq.leased, deliveryID)
	if len(pq.pending) > 0 {
		next := pq.pending[0]
		pq.pending = pq.pending[1:]
		pq.leased[next] = true
	}
	if len(pq.leased) == 0 && len(pq.pending) == 0 {
		delete(o.partitions, partitionKey)
	}
}

func (o *OrderingCoordinator) PendingCount(partitionKey string) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	pq, ok := o.partitions[partitionKey]
	if !ok {
		return 0
	}
	return len(pq.pending)
}

func (o *OrderingCoordinator) ActiveCount(partitionKey string) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	pq, ok := o.partitions[partitionKey]
	if !ok {
		return 0
	}
	return len(pq.leased)
}

func (o *OrderingCoordinator) ActivePartitions() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	keys := make([]string, 0, len(o.partitions))
	for k := range o.partitions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func ComputePartitionKey(aggregateType, aggregateID string) string {
	if aggregateType == "" || aggregateID == "" {
		return ""
	}
	return fmt.Sprintf("%s:%s", aggregateType, aggregateID)
}

func ComputeConversationPartition(conversationID string) string {
	if conversationID == "" {
		return ""
	}
	return fmt.Sprintf("conversation:%s", conversationID)
}

func ComputeCharacterPartition(characterID string) string {
	if characterID == "" {
		return ""
	}
	return fmt.Sprintf("character:%s", characterID)
}

func ComputeExtensionPartition(extensionID string) string {
	if extensionID == "" {
		return ""
	}
	return fmt.Sprintf("extension:%s", extensionID)
}

func ComputeWorkflowPartition(workflowRunID string) string {
	if workflowRunID == "" {
		return ""
	}
	return fmt.Sprintf("workflow:%s", workflowRunID)
}

type SequenceAllocator struct {
	mu        sync.Mutex
	sequences map[string]int64
}

func NewSequenceAllocator() *SequenceAllocator {
	return &SequenceAllocator{
		sequences: make(map[string]int64),
	}
}

func (s *SequenceAllocator) Next(partitionKey string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sequences[partitionKey]++
	return s.sequences[partitionKey]
}

func (s *SequenceAllocator) Current(partitionKey string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sequences[partitionKey]
}

func (s *SequenceAllocator) Reset(partitionKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sequences, partitionKey)
}
