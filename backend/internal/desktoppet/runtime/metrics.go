// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtime

import (
	"sync/atomic"
	"time"
)

type Metrics struct {
	connectionsCurrent     atomic.Int64
	sessionsTotal          atomic.Int64
	handshakeFailuresTotal atomic.Int64
	commandsTotal          atomic.Int64
	commandRetriesTotal    atomic.Int64
	sendQueueOverflowTotal atomic.Int64
	reconcileTotal         atomic.Int64
	protocolErrorsTotal    atomic.Int64
	goroutinesCurrent      atomic.Int64
}

func NewMetrics() *Metrics {
	return &Metrics{}
}

func (m *Metrics) IncConnections()       { m.connectionsCurrent.Add(1) }
func (m *Metrics) DecConnections()       { m.connectionsCurrent.Add(-1) }
func (m *Metrics) IncSessions()          { m.sessionsTotal.Add(1) }
func (m *Metrics) IncHandshakeFailures() { m.handshakeFailuresTotal.Add(1) }
func (m *Metrics) IncCommands()          { m.commandsTotal.Add(1) }
func (m *Metrics) IncCommandRetries()    { m.commandRetriesTotal.Add(1) }
func (m *Metrics) IncSendQueueOverflow() { m.sendQueueOverflowTotal.Add(1) }
func (m *Metrics) IncReconcile()         { m.reconcileTotal.Add(1) }
func (m *Metrics) IncProtocolErrors()    { m.protocolErrorsTotal.Add(1) }
func (m *Metrics) IncGoroutines()        { m.goroutinesCurrent.Add(1) }
func (m *Metrics) DecGoroutines()        { m.goroutinesCurrent.Add(-1) }

type MetricsSnapshot struct {
	ConnectionsCurrent     int64 `json:"connectionsCurrent"`
	SessionsTotal          int64 `json:"sessionsTotal"`
	HandshakeFailuresTotal int64 `json:"handshakeFailuresTotal"`
	CommandsTotal          int64 `json:"commandsTotal"`
	CommandRetriesTotal    int64 `json:"commandRetriesTotal"`
	SendQueueOverflowTotal int64 `json:"sendQueueOverflowTotal"`
	ReconcileTotal         int64 `json:"reconcileTotal"`
	ProtocolErrorsTotal    int64 `json:"protocolErrorsTotal"`
	GoroutinesCurrent      int64 `json:"goroutinesCurrent"`
}

func (m *Metrics) Snapshot() MetricsSnapshot {
	return MetricsSnapshot{
		ConnectionsCurrent:     m.connectionsCurrent.Load(),
		SessionsTotal:          m.sessionsTotal.Load(),
		HandshakeFailuresTotal: m.handshakeFailuresTotal.Load(),
		CommandsTotal:          m.commandsTotal.Load(),
		CommandRetriesTotal:    m.commandRetriesTotal.Load(),
		SendQueueOverflowTotal: m.sendQueueOverflowTotal.Load(),
		ReconcileTotal:         m.reconcileTotal.Load(),
		ProtocolErrorsTotal:    m.protocolErrorsTotal.Load(),
		GoroutinesCurrent:      m.goroutinesCurrent.Load(),
	}
}

type StatusView struct {
	RuntimeID  string          `json:"runtimeId"`
	Connection *ConnectionView `json:"connection,omitempty"`
	Desired    *DesiredView    `json:"desired,omitempty"`
	Actual     *ActualView     `json:"actual,omitempty"`
	Sync       *SyncView       `json:"sync,omitempty"`
}

type ConnectionView struct {
	State           string    `json:"state"`
	SessionID       string    `json:"sessionId"`
	LastHeartbeatAt time.Time `json:"lastHeartbeatAt"`
	ProtocolVersion string    `json:"protocolVersion"`
	Capabilities    []string  `json:"capabilities"`
}

type DesiredView struct {
	InstallationID string `json:"installationId"`
	Revision       int64  `json:"revision"`
	Enabled        bool   `json:"enabled"`
}

type ActualView struct {
	InstallationID   string    `json:"installationId"`
	Revision         int64     `json:"revision"`
	Visible          bool      `json:"visible"`
	CurrentActionKey string    `json:"currentActionKey"`
	ObservedAt       time.Time `json:"observedAt"`
	Stale            bool      `json:"stale"`
}

type SyncView struct {
	Status          string `json:"status"`
	PendingCommands int    `json:"pendingCommands"`
}
