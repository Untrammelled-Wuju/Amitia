package scheduler

import "github.com/u-ai/backend/internal/queue"

type PriorityLevel = queue.PriorityLevel

const (
	PriorityP0 = queue.PriorityP0
	PriorityP1 = queue.PriorityP1
	PriorityP2 = queue.PriorityP2
	PriorityP3 = queue.PriorityP3
	PriorityP4 = queue.PriorityP4
	PriorityP5 = queue.PriorityP5
)

type PriorityConfig = queue.PriorityConfig

var DefaultPriorityConfigs = queue.DefaultPriorityConfigs

type TaskStatus = queue.TaskStatus

const (
	TaskPending   = queue.TaskPending
	TaskRunning   = queue.TaskRunning
	TaskCompleted = queue.TaskCompleted
	TaskCancelled = queue.TaskCancelled
	TaskExpired   = queue.TaskExpired
	TaskDropped   = queue.TaskDropped
)

type DropReason = queue.DropReason

const (
	DropReasonQueueFull        = queue.DropReasonQueueFull
	DropReasonExpired          = queue.DropReasonExpired
	DropReasonBudgetLimit      = queue.DropReasonBudgetLimit
	DropReasonDeadlineExceeded = queue.DropReasonDeadlineExceeded
)

type Task = queue.Task

type PriorityQueue = queue.PriorityQueue

type QueueConfig = queue.PriorityQueueConfig

type QueueMetrics = queue.QueueMetricsSnapshot

func NewPriorityQueue(cfg QueueConfig) *PriorityQueue {
	return queue.NewPriorityQueue(cfg)
}
