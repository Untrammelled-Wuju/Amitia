// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"time"

	"gorm.io/gorm"
)

type RemoteCoreProbe interface {
	Probe() ProbeResult
}

type ProbeResult struct {
	Healthy    bool                   `json:"healthy"`
	LatencyMS  int64                  `json:"latencyMs"`
	Checks     map[string]ProbeCheck `json:"checks"`
	ProbeTime  time.Time              `json:"probeTime"`
}

type ProbeCheck struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

type DefaultRemoteCoreProbe struct {
	db *gorm.DB
}

func NewDefaultRemoteCoreProbe(db *gorm.DB) *DefaultRemoteCoreProbe {
	return &DefaultRemoteCoreProbe{db: db}
}

func (p *DefaultRemoteCoreProbe) Probe() ProbeResult {
	result := ProbeResult{
		Healthy:   true,
		Checks:    make(map[string]ProbeCheck),
		ProbeTime: time.Now().UTC(),
	}

	start := time.Now()

	dbCheck := ProbeCheck{OK: true, Message: "database connected"}
	if p.db != nil {
		sqlDB, err := p.db.DB()
		if err != nil {
			dbCheck = ProbeCheck{OK: false, Message: "failed to get db connection: " + err.Error()}
			result.Healthy = false
		} else if err := sqlDB.Ping(); err != nil {
			dbCheck = ProbeCheck{OK: false, Message: "database ping failed: " + err.Error()}
			result.Healthy = false
		}
	} else {
		dbCheck = ProbeCheck{OK: false, Message: "database not configured"}
		result.Healthy = false
	}
	result.Checks["database"] = dbCheck

	result.LatencyMS = time.Since(start).Milliseconds()
	return result
}
