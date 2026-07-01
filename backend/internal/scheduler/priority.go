package scheduler

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
	TaskPending    TaskStatus = "pending"
	TaskRunning    TaskStatus = "running"
	TaskCompleted  TaskStatus = "completed"
	TaskCancelled  TaskStatus = "cancelled"
	TaskExpired    TaskStatus = "expired"
	TaskDropped    TaskStatus = "dropped"
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
	metrics      *QueueMetrics
}

type QueueConfig struct {
	MaxQueueSize int
	Configs      map[PriorityLevel]PriorityConfig
	OnDrop       func(*Task, DropReason)
}

type QueueMetrics struct {
	Mu                  sync.Mutex
	TotalEnqueued       int64
	TotalDropped        int64
	TotalCompleted      int64
	TotalCancelled      int64
	TotalExpired        int64
	DropReasons         map[DropReason]int64
	MergeReasons        map[string]int64
	QueueDepthSnapshots []int64
	TaskAgeSamples      []time.Duration
}

func NewQueueMetrics() *QueueMetrics {
	return &QueueMetrics{
		DropReasons:  make(map[DropReason]int64),
		MergeReasons: make(map[string]int64),
	}
}

func NewPriorityQueue(cfg QueueConfig) *PriorityQueue {
	configs := cfg.Configs
	if configs == nil {
		configs = DefaultPriorityConfigs
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
		metrics:      NewQueueMetrics(),
	}
	for level := PriorityP0; level <= PriorityP5; level++ {
		pq.queues[level] = make([]*Task, 0)
	}
	return pq
}

func (pq *PriorityQueue) Enqueue(task *Task) (bool, DropReason) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	task.CreatedAt = time.Now().UTC()
	if task.Priority < PriorityP0 || task.Priority > PriorityP5 {
		task.Priority = PriorityP5
	}

	totalSize := pq.totalLocked()
	if totalSize >= pq.maxSize {
		_ = pq.evictLowestPriorityLocked()
		if pq.totalLocked() >= pq.maxSize {
			task.Status = TaskDropped
			task.DropReason = DropReasonQueueFull
			pq.metrics.TotalDropped++
			pq.metrics.DropReasons[DropReasonQueueFull]++
			if pq.onDrop != nil {
				pq.onDrop(task, DropReasonQueueFull)
			}
			return false, DropReasonQueueFull
		}
	}

	task.Status = TaskPending
	task.Done = make(chan struct{})

	if !task.Deadline.IsZero() && time.Now().UTC().After(task.Deadline) {
		task.Status = TaskExpired
		pq.metrics.TotalExpired++
		if pq.onDrop != nil {
			pq.onDrop(task, DropReasonExpired)
		}
		return false, DropReasonExpired
	}

	pq.queues[task.Priority] = append(pq.queues[task.Priority], task)
	pq.metrics.TotalEnqueued++
	return true, ""
}

func (pq *PriorityQueue) Dequeue() *Task {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	now := time.Now().UTC()
	pq.cleanupExpiredLocked(now)

	for level := PriorityP0; level <= PriorityP5; level++ {
		cfg := pq.configs[level]

		if pq.activeCounts[level] >= cfg.MaxConcurrency {
			continue
		}

		queue := pq.queues[level]
		if len(queue) == 0 {
			continue
		}

		idx := -1
		for i, t := range queue {
			if t.Status == TaskPending {
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
		return task
	}

	return nil
}

func (pq *PriorityQueue) Complete(task *Task) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if task.Status == TaskRunning {
		pq.activeCounts[task.Priority]--
		task.Status = TaskCompleted
		if task.Done != nil {
			close(task.Done)
		}
		pq.metrics.TotalCompleted++
	}
}

func (pq *PriorityQueue) Cancel(task *Task) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if task.Status == TaskPending || task.Status == TaskRunning {
		if task.Status == TaskRunning {
			pq.activeCounts[task.Priority]--
		}
		task.Status = TaskCancelled
		if task.Done != nil {
			close(task.Done)
		}
		pq.metrics.TotalCancelled++
	}
}

func (pq *PriorityQueue) ExpireStaleTasks(maxAge time.Duration) int {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	now := time.Now().UTC()
	expired := 0

	for level := PriorityP0; level <= PriorityP5; level++ {
		queue := pq.queues[level]
		remaining := make([]*Task, 0, len(queue))
		for _, t := range queue {
			if t.Status == TaskPending && now.Sub(t.CreatedAt) > maxAge {
				t.Status = TaskExpired
				pq.metrics.TotalExpired++
				if pq.onDrop != nil {
					pq.onDrop(t, DropReasonExpired)
				}
				expired++
			} else {
				remaining = append(remaining, t)
			}
		}
		pq.queues[level] = remaining
	}

	return expired
}

func (pq *PriorityQueue) cleanupExpiredLocked(now time.Time) {
	for level := PriorityP0; level <= PriorityP5; level++ {
		queue := pq.queues[level]
		remaining := make([]*Task, 0, len(queue))
		for _, t := range queue {
			if t.Status == TaskPending && !t.Deadline.IsZero() && now.After(t.Deadline) {
				t.Status = TaskExpired
				pq.metrics.TotalExpired++
				if pq.onDrop != nil {
					pq.onDrop(t, DropReasonExpired)
				}
			} else {
				remaining = append(remaining, t)
			}
		}
		pq.queues[level] = remaining
	}
}

func (pq *PriorityQueue) CancelByScope(scope string) int {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	cancelled := 0
	for level := PriorityP0; level <= PriorityP5; level++ {
		queue := pq.queues[level]
		remaining := make([]*Task, 0, len(queue))
		for _, t := range queue {
			if t.Scope == scope && t.Status == TaskPending {
				t.Status = TaskCancelled
				cancelled++
			} else {
				remaining = append(remaining, t)
			}
		}
		pq.queues[level] = remaining
	}
	pq.metrics.TotalCancelled += int64(cancelled)
	return cancelled
}

func (pq *PriorityQueue) Depth() int {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	total := pq.totalLocked()
	pq.metrics.QueueDepthSnapshots = append(pq.metrics.QueueDepthSnapshots, int64(total))
	if len(pq.metrics.QueueDepthSnapshots) > 100 {
		pq.metrics.QueueDepthSnapshots = pq.metrics.QueueDepthSnapshots[1:]
	}
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

func (pq *PriorityQueue) MetricsSnapshot() QueueMetrics {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	snapshot := *pq.metrics

	snapshot.DropReasons = make(map[DropReason]int64)
	for k, v := range pq.metrics.DropReasons {
		snapshot.DropReasons[k] = v
	}
	snapshot.MergeReasons = make(map[string]int64)
	for k, v := range pq.metrics.MergeReasons {
		snapshot.MergeReasons[k] = v
	}
	return snapshot
}

func (pq *PriorityQueue) RecordMerge(reason string) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	pq.metrics.MergeReasons[reason]++
}

func (pq *PriorityQueue) RecordDrop(reason DropReason) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	pq.metrics.DropReasons[reason]++
}

func (pq *PriorityQueue) totalLocked() int {
	total := 0
	for _, q := range pq.queues {
		for _, t := range q {
			if t.Status == TaskPending {
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
				if pq.onDrop != nil {
					pq.onDrop(task, DropReasonQueueFull)
				}
				return DropReasonQueueFull
			}
		}
	}
	return ""
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
