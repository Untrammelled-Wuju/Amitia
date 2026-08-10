package control

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

const (
	DefaultPermitTTL = 5 * time.Second
)

type OutputPermit struct {
	PermitID      string
	RuntimeID     domain.RuntimeInstanceID
	ServiceID     domain.ServiceID
	PluginID      domain.PluginID
	OutputEpoch   uint64
	OutputKind    ControlOutputKind
	IssuedAt      time.Time
	ExpiresAt     time.Time
	ControlMode   domain.ControlMode
}

func NewOutputPermit(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, pluginID domain.PluginID, epoch uint64, kind ControlOutputKind, mode domain.ControlMode, ttl time.Duration, now time.Time) OutputPermit {
	return OutputPermit{
		PermitID:    generatePermitID(),
		RuntimeID:   runtimeID,
		ServiceID:   serviceID,
		PluginID:    pluginID,
		OutputEpoch: epoch,
		OutputKind:  kind,
		IssuedAt:    now,
		ExpiresAt:   now.Add(ttl),
		ControlMode: mode,
	}
}

func (p OutputPermit) IsExpired(now time.Time) bool {
	return !now.Before(p.ExpiresAt)
}

func (p OutputPermit) IsCurrent(epoch uint64) bool {
	return p.OutputEpoch == epoch
}

func (p OutputPermit) Validate(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, epoch uint64, now time.Time) error {
	if p.RuntimeID != runtimeID {
		return errPermitRuntimeMismatch(p.RuntimeID, runtimeID)
	}
	if p.ServiceID != serviceID {
		return errPermitServiceMismatch(p.ServiceID, serviceID)
	}
	if !p.IsCurrent(epoch) {
		return errPermitStale(p.OutputEpoch, epoch)
	}
	if p.IsExpired(now) {
		return errPermitExpired(p.PermitID, p.ExpiresAt, now)
	}
	return nil
}

func generatePermitID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return "permit-" + hex.EncodeToString(b[:])
}
