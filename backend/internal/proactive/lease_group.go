package proactive

import (
	"log"
	"time"
)

func AcquireLeaseForGroup(priority OutputPriority, characterID, conversationID, group, correlationID string, ttl time.Duration) *OutputLease {
	lease := GlobalLeaseManager.AcquireLease(priority, characterID, conversationID, group, correlationID, ttl)
	lease.ChannelGroup = group
	log.Printf("[OutputLease] acquired lease for group id=%s group=%s char=%s", lease.ID, group, characterID)
	return lease
}

func CancelLowPriorityOnUserInput(characterID string) int {
	return GlobalLeaseManager.CancelByUserInput(characterID)
}

func CancelLeasesForGroup(characterID, group string) int {
	mgr := GlobalLeaseManager
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	cancelled := 0
	now := time.Now()
	for id, lease := range mgr.leases {
		if lease.CharacterID != characterID {
			continue
		}
		if lease.ChannelGroup != group {
			continue
		}
		if lease.IsExpired(now) {
			delete(mgr.leases, id)
			continue
		}
		lease.Cancel()
		delete(mgr.leases, id)
		cancelled++
	}
	if cancelled > 0 {
		log.Printf("[OutputLease] cancelled %d leases for group=%s char=%s", cancelled, group, characterID)
	}
	return cancelled
}

func GetActiveLeasesForGroup(characterID, group string) []*OutputLease {
	mgr := GlobalLeaseManager
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	now := time.Now()
	var active []*OutputLease
	for _, lease := range mgr.leases {
		if characterID != "" && lease.CharacterID != characterID {
			continue
		}
		if group != "" && lease.ChannelGroup != group {
			continue
		}
		if lease.IsExpired(now) {
			continue
		}
		if lease.IsCancelled() {
			continue
		}
		active = append(active, lease)
	}
	if active == nil {
		active = []*OutputLease{}
	}
	return active
}
