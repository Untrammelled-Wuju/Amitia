// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtimehost

import "time"

type ProcessState string

const (
	StateRegistered    ProcessState = "registered"
	StateStarting      ProcessState = "starting"
	StateRunning       ProcessState = "running"
	StateReady         ProcessState = "ready"
	StateRestartBackoff ProcessState = "restart-backoff"
	StateStopping      ProcessState = "stopping"
	StateStopped       ProcessState = "stopped"
	StateFailed        ProcessState = "failed"
)

type ProcessSnapshot struct {
	ID            ProcessID    `json:"id"`
	State         ProcessState `json:"state"`
	PID           int          `json:"pid"`
	Executable    string       `json:"executable"`
	StartedAt     time.Time    `json:"started_at"`
	ReadyAt       time.Time    `json:"ready_at"`
	StoppedAt     time.Time    `json:"stopped_at"`
	RestartCount  int          `json:"restart_count"`
	LastExitCode  int          `json:"last_exit_code"`
	LastError     string       `json:"last_error"`
	HealthFailures int         `json:"health_failures"`
}

type ProcessEvent struct {
	ProcessID    ProcessID    `json:"process_id"`
	Type         ProcessEventType `json:"type"`
	PID          int          `json:"pid"`
	RestartCount int          `json:"restart_count"`
	Timestamp    time.Time    `json:"timestamp"`
	Error        string       `json:"error,omitempty"`
}

type ProcessEventType string

const (
	EventRegistered      ProcessEventType = "registered"
	EventStarting        ProcessEventType = "starting"
	EventStarted         ProcessEventType = "started"
	EventReady           ProcessEventType = "ready"
	EventUnhealthy       ProcessEventType = "unhealthy"
	EventExited          ProcessEventType = "exited"
	EventRestartScheduled ProcessEventType = "restart-scheduled"
	EventRestarted       ProcessEventType = "restarted"
	EventFailed          ProcessEventType = "failed"
	EventStopping        ProcessEventType = "stopping"
	EventStopped         ProcessEventType = "stopped"
)
