//go:build rsi_resource

package resource

import (
	"errors"
	"fmt"
	"sort"
)

type ManagedKind string

const (
	ManagedEventSubscription ManagedKind = "event_subscription"
	ManagedPriorityQueue     ManagedKind = "priority_queue"
	ManagedBackpressure      ManagedKind = "backpressure"
	ManagedOutbox            ManagedKind = "outbox"
	ManagedResourceOwnership ManagedKind = "resource_ownership"
	ManagedTaskEntry         ManagedKind = "task_entry"
)

var ErrInventoryInvalid = errors.New("resource: invalid inventory")

type InventoryEntry struct {
	Kind      ManagedKind
	Symbol    string
	Path      string
	Owner     string
	Symmetric bool
	Note      string
}

func (e InventoryEntry) Validate() error {
	if e.Kind == "" || e.Symbol == "" || e.Path == "" || e.Owner == "" {
		return fmt.Errorf("%w: incomplete entry %+v", ErrInventoryInvalid, e)
	}
	return nil
}

type Inventory struct {
	Entries []InventoryEntry
}

func DefaultInventory() []InventoryEntry {
	return []InventoryEntry{
		{Kind: ManagedResourceOwnership, Symbol: "type ResourceOwnershipService interface", Path: "internal/extension/kernel/resource/service.go", Owner: "resource", Symmetric: true, Note: "已有的所有权登记/释放/孤儿扫描入口，不另建资源账本"},
		{Kind: ManagedEventSubscription, Symbol: "type SubscriptionRepository interface", Path: "internal/extension/kernel/event/subscription_repository.go", Owner: "event", Symmetric: true, Note: "订阅持久化与退订的现有入口"},
		{Kind: ManagedEventSubscription, Symbol: "func NewDispatcher(", Path: "internal/extension/kernel/event/event_dispatcher.go", Owner: "event", Symmetric: true, Note: "投递器持有订阅与死信，不复制分发逻辑"},
		{Kind: ManagedPriorityQueue, Symbol: "type PriorityQueue = queue.PriorityQueue", Path: "internal/scheduler/priority.go", Owner: "scheduler", Symmetric: true, Note: "优先级类型词汇与 drop reason 的唯一来源；P0/P1 可突破 maxSize 且无字节预算，M07-S03 的内存准入由 task_runtime 的 BoundedQueue 唯一持有"},
		{Kind: ManagedBackpressure, Symbol: "func NewBackpressureController(", Path: "internal/scheduler/backpressure.go", Owner: "scheduler", Symmetric: true, Note: "现有背压状态机"},
		{Kind: ManagedOutbox, Symbol: "func NewOutbox(", Path: "internal/scheduler/backpressure.go", Owner: "scheduler", Symmetric: true, Note: "批量刷写与重试的现有实现"},
		{Kind: ManagedTaskEntry, Symbol: "type Entry", Path: "internal/extension/kernel/task_runtime/entry.go", Owner: "task_runtime", Symmetric: false, Note: "任务入口记录，本身不持有需关闭资源"},
	}
}

func NewInventory(entries []InventoryEntry) (Inventory, error) {
	inventory := Inventory{Entries: append([]InventoryEntry{}, entries...)}
	sort.Slice(inventory.Entries, func(left, right int) bool {
		if inventory.Entries[left].Kind != inventory.Entries[right].Kind {
			return inventory.Entries[left].Kind < inventory.Entries[right].Kind
		}
		return inventory.Entries[left].Symbol < inventory.Entries[right].Symbol
	})
	seen := map[string]bool{}
	for _, entry := range inventory.Entries {
		if err := entry.Validate(); err != nil {
			return Inventory{}, err
		}
		key := string(entry.Kind) + "|" + entry.Symbol + "|" + entry.Path
		if seen[key] {
			return Inventory{}, fmt.Errorf("%w: duplicate entry %s", ErrInventoryInvalid, key)
		}
		seen[key] = true
	}
	return inventory, nil
}

func DefaultGovernanceInventory() (Inventory, error) {
	return NewInventory(DefaultInventory())
}

func (i Inventory) Kinds() []ManagedKind {
	seen := map[ManagedKind]bool{}
	kinds := make([]ManagedKind, 0, len(i.Entries))
	for _, entry := range i.Entries {
		if seen[entry.Kind] {
			continue
		}
		seen[entry.Kind] = true
		kinds = append(kinds, entry.Kind)
	}
	sort.Slice(kinds, func(left, right int) bool { return kinds[left] < kinds[right] })
	return kinds
}

func (i Inventory) Owners(kind ManagedKind) []string {
	seen := map[string]bool{}
	owners := make([]string, 0, 2)
	for _, entry := range i.Entries {
		if entry.Kind != kind || seen[entry.Owner] {
			continue
		}
		seen[entry.Owner] = true
		owners = append(owners, entry.Owner)
	}
	sort.Strings(owners)
	return owners
}

func (i Inventory) Count(kind ManagedKind) int {
	count := 0
	for _, entry := range i.Entries {
		if entry.Kind == kind {
			count++
		}
	}
	return count
}
