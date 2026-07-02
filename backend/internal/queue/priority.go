package queue

import (
	"sort"
	"sync"
	"time"
)

type PriorityLevel int

const (
	PriorityP0 PriorityLevel = iota
	PriorityP1
	PriorityP2
	PriorityP3
	PriorityP4
	PriorityP5
)

type PriorityConfig struct {
	MaxConcurrency  int
	MaxWaitDuration time.Duration
	AllowPreempt    bool
}

var DefaultPriorityConfigs = map[PriorityLevel]PriorityConfig{
	PriorityP0: {MaxConcurrency: 10, MaxWaitDuration: 0, AllowPreempt: true},
	PriorityP1: {MaxConcurrency: 5, MaxWaitDuration: 30 * time.Second, AllowPreempt: false},
	PriorityP2: {MaxConcurrency: 3, MaxWaitDuration: 60 * time.Second, AllowPreempt: false},
	PriorityP3: {MaxConcurrency: 2, MaxWaitDuration: 120 * time.Second, AllowPreempt: false},
	PriorityP4: {MaxConcurrency: 1, MaxWaitDuration: 300 * time.Second, AllowPreempt: false},
	PriorityP5: {MaxConcurrency: 1, MaxWaitDuration: 600 * time.Second, AllowPreempt: false},
}

type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskCancelled TaskStatus = "cancelled"
	TaskExpired   TaskStatus = "expired"
	TaskDropped   TaskStatus = "dropped"
)

type DropReason string

const (
	DropReasonQueueFull        DropReason = "queue_full"
	DropReasonExpired          DropReason = "expired"
	DropReasonBudgetLimit      DropReason = "budget_limit"
	DropReasonDeadlineExceeded DropReason = "deadline_exceeded"
)

type Task struct {
	ID         string
	Path       string
	Priority   PriorityLevel
	Scope      string
	CreatedAt  time.Time
	Deadline   time.Time
	Status     TaskStatus
	DropReason DropReason
	Result     interface{}
	Err        error
	Done       chan struct{}
}

type PriorityQueue struct {
	mu           sync.Mutex
	queues       map[PriorityLevel][]*Task
	maxSize      int
	configs      map[PriorityLevel]PriorityConfig
	activeCounts map[PriorityLevel]int
	onDrop       func(*Task, DropReason)
	metrics      *QueueMetricsRecord
}

type PriorityQueueConfig struct {
	MaxQueueSize int
	Configs      map[PriorityLevel]PriorityConfig
	OnDrop       func(*Task, DropReason)
}

func NewPriorityQueue(cfg PriorityQueueConfig) *PriorityQueue {
	configs := make(map[PriorityLevel]PriorityConfig, len(DefaultPriorityConfigs))
	for level, config := range DefaultPriorityConfigs {
		configs[level] = config
	}
	for level, config := range cfg.Configs {
		configs[level] = config
	}

	maxSize := cfg.MaxQueueSize
	if maxSize <= 0 {
		maxSize = 1000
	}

	pq := &PriorityQueue{
		queues:       make(map[PriorityLevel][]*Task),
		maxSize:      maxSize,
		configs:      configs,
		activeCounts: make(map[PriorityLevel]int),
		onDrop:       cfg.OnDrop,
		metrics:      NewQueueMetricsRecord(),
	}
	for level := PriorityP0; level <= PriorityP5; level++ {
		pq.queues[level] = make([]*Task, 0)
	}
	return pq
}

func (pq *PriorityQueue) Enqueue(task *Task) (bool, DropReason) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	now := time.Now().UTC()
	task.CreatedAt = now
	if task.Priority < PriorityP0 || task.Priority > PriorityP5 {
		task.Priority = PriorityP5
	}

	if !task.Deadline.IsZero() && now.After(task.Deadline) {
		task.Status = TaskExpired
		task.DropReason = DropReasonExpired
		pq.recordDropLocked(task, DropReasonExpired)
		return false, DropReasonExpired
	}

	if pq.totalLocked() >= pq.maxSize {
		_ = pq.evictLowestPriorityLocked()
		if pq.totalLocked() >= pq.maxSize {
			task.Status = TaskDropped
			task.DropReason = DropReasonQueueFull
			pq.recordDropLocked(task, DropReasonQueueFull)
			return false, DropReasonQueueFull
		}
	}

	task.Status = TaskPending
	task.DropReason = ""
	task.Done = make(chan struct{})
	pq.queues[task.Priority] = append(pq.queues[task.Priority], task)
	pq.metrics.RecordEnqueue()
	pq.metrics.RecordQueueDepth(int64(pq.totalLocked()))
	return true, ""
}

func (pq *PriorityQueue) Dequeue() *Task {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	now := time.Now().UTC()
	pq.cleanupExpiredLocked(now)

	for level := PriorityP0; level <= PriorityP5; level++ {
		cfg := pq.configs[level]
		if cfg.MaxConcurrency <= 0 || pq.activeCounts[level] >= cfg.MaxConcurrency {
			continue
		}

		queue := pq.queues[level]
		if len(queue) == 0 {
			continue
		}

		idx := -1
		for i, task := range queue {
			if task.Status == TaskPending {
				idx = i
				break
			}
		}
		if idx < 0 {
			pq.queues[level] = queue[:0]
			continue
		}

		task := queue[idx]
		pq.queues[level] = append(queue[:idx], queue[idx+1:]...)
		task.Status = TaskRunning
		pq.activeCounts[task.Priority]++
		pq.metrics.RecordDequeue()
		pq.metrics.RecordTaskAge(now.Sub(task.CreatedAt))
		pq.metrics.RecordQueueDepth(int64(pq.totalLocked()))
		return task
	}

	pq.metrics.RecordQueueDepth(int64(pq.totalLocked()))
	return nil
}

func (pq *PriorityQueue) Complete(task *Task) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if task.Status != TaskRunning {
		return
	}
	pq.activeCounts[task.Priority]--
	task.Status = TaskCompleted
	if task.Done != nil {
		close(task.Done)
	}
	pq.metrics.RecordComplete()
	pq.metrics.RecordQueueDepth(int64(pq.totalLocked()))
}

func (pq *PriorityQueue) Cancel(task *Task) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if task.Status != TaskPending && task.Status != TaskRunning {
		return
	}
	if task.Status == TaskRunning {
		pq.activeCounts[task.Priority]--
	}
	task.Status = TaskCancelled
	if task.Done != nil {
		close(task.Done)
	}
	pq.metrics.RecordCancel()
	pq.metrics.RecordQueueDepth(int64(pq.totalLocked()))
}

func (pq *PriorityQueue) ExpireStaleTasks(maxAge time.Duration) int {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	now := time.Now().UTC()
	expired := 0
	for level := PriorityP0; level <= PriorityP5; level++ {
		queue := pq.queues[level]
		remaining := make([]*Task, 0, len(queue))
		for _, task := range queue {
			if task.Status == TaskPending && now.Sub(task.CreatedAt) > maxAge {
				task.Status = TaskExpired
				task.DropReason = DropReasonExpired
				pq.recordDropLocked(task, DropReasonExpired)
				expired++
			} else {
				remaining = append(remaining, task)
			}
		}
		pq.queues[level] = remaining
	}
	pq.metrics.RecordQueueDepth(int64(pq.totalLocked()))
	return expired
}

func (pq *PriorityQueue) CancelByScope(scope string) int {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	cancelled := 0
	for level := PriorityP0; level <= PriorityP5; level++ {
		queue := pq.queues[level]
		remaining := make([]*Task, 0, len(queue))
		for _, task := range queue {
			if task.Scope == scope && task.Status == TaskPending {
				task.Status = TaskCancelled
				pq.metrics.RecordCancel()
				cancelled++
			} else {
				remaining = append(remaining, task)
			}
		}
		pq.queues[level] = remaining
	}
	pq.metrics.RecordQueueDepth(int64(pq.totalLocked()))
	return cancelled
}

func (pq *PriorityQueue) Depth() int {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	total := pq.totalLocked()
	pq.metrics.RecordQueueDepth(int64(total))
	return total
}

func (pq *PriorityQueue) ActiveCount() int {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	total := 0
	for _, count := range pq.activeCounts {
		total += count
	}
	return total
}

func (pq *PriorityQueue) MetricsSnapshot() QueueMetricsSnapshot {
	return pq.metrics.Snapshot(time.Time{})
}

func (pq *PriorityQueue) RecordMerge(reason string) {
	pq.metrics.RecordMerge(reason)
}

func (pq *PriorityQueue) RecordDrop(reason DropReason) {
	pq.metrics.RecordDrop(string(reason))
}

func (pq *PriorityQueue) ReorderByDeadline() {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	for level := PriorityP0; level <= PriorityP5; level++ {
		queue := pq.queues[level]
		sort.SliceStable(queue, func(i, j int) bool {
			if queue[i].Status != queue[j].Status {
				return queue[i].Status == TaskPending
			}
			di := queue[i].Deadline
			dj := queue[j].Deadline
			if di.IsZero() && dj.IsZero() {
				return queue[i].CreatedAt.Before(queue[j].CreatedAt)
			}
			if di.IsZero() {
				return false
			}
			if dj.IsZero() {
				return true
			}
			return di.Before(dj)
		})
		pq.queues[level] = queue
	}
}

func (pq *PriorityQueue) cleanupExpiredLocked(now time.Time) {
	for level := PriorityP0; level <= PriorityP5; level++ {
		queue := pq.queues[level]
		remaining := make([]*Task, 0, len(queue))
		for _, task := range queue {
			if task.Status == TaskPending && !task.Deadline.IsZero() && now.After(task.Deadline) {
				task.Status = TaskExpired
				task.DropReason = DropReasonExpired
				pq.recordDropLocked(task, DropReasonExpired)
			} else {
				remaining = append(remaining, task)
			}
		}
		pq.queues[level] = remaining
	}
}

func (pq *PriorityQueue) totalLocked() int {
	total := 0
	for _, tasks := range pq.queues {
		for _, task := range tasks {
			if task.Status == TaskPending {
				total++
			}
		}
	}
	for _, count := range pq.activeCounts {
		total += count
	}
	return total
}

func (pq *PriorityQueue) evictLowestPriorityLocked() DropReason {
	for level := PriorityP5; level >= PriorityP0; level-- {
		queue := pq.queues[level]
		for i := len(queue) - 1; i >= 0; i-- {
			if queue[i].Status == TaskPending {
				task := queue[i]
				pq.queues[level] = append(queue[:i], queue[i+1:]...)
				task.Status = TaskDropped
				task.DropReason = DropReasonQueueFull
				pq.recordDropLocked(task, DropReasonQueueFull)
				return DropReasonQueueFull
			}
		}
	}
	return ""
}

func (pq *PriorityQueue) recordDropLocked(task *Task, reason DropReason) {
	pq.metrics.RecordDrop(string(reason))
	if pq.onDrop != nil {
		pq.onDrop(task, reason)
	}
}
