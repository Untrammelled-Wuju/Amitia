package domain

import "time"

type HealthStatus string

const (
	HealthUnknown   HealthStatus = "unknown"
	HealthHealthy   HealthStatus = "healthy"
	HealthDegraded  HealthStatus = "degraded"
	HealthUnhealthy HealthStatus = "unhealthy"
)

type HealthState struct {
	Status    HealthStatus
	Message   string
	UpdatedAt time.Time
}

func (h HealthState) IsHealthy() bool {
	return h.Status == HealthHealthy
}

func (h HealthState) IsDegraded() bool {
	return h.Status == HealthDegraded
}

func (h HealthState) IsUnhealthy() bool {
	return h.Status == HealthUnhealthy
}
