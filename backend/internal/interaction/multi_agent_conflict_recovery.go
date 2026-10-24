package interaction

import (
	"context"
	"fmt"
)

func (c *MultiAgentCoordinator) markCoordinationStale(ac *activeCoordination) {
	c.mu.Lock()
	ac.staleSnapshot = true
	c.mu.Unlock()
}

func (c *MultiAgentCoordinator) currentCoordinationRecord(ctx context.Context, ac *activeCoordination) (*InteractionRecord, string, error) {
	if c.tracker == nil {
		return nil, "", fmt.Errorf("multi_agent: tracker unavailable for coordination fencing")
	}
	record, found, err := c.tracker.Get(ctx, ac.parentInteractionID)
	if err != nil {
		return nil, "", err
	}
	if !found || record == nil || record.RecoveryDescriptor == nil || record.RecoveryDescriptor.MultiAgent == nil ||
		record.RecoveryDescriptor.MultiAgent.CoordinationID != string(ac.id) {
		return nil, "", fmt.Errorf("multi_agent: parent coordination checkpoint missing or replaced")
	}
	json, err := record.RecoveryDescriptor.NormalizeOnSerialize()
	if err != nil {
		return nil, "", err
	}
	return record, string(json), nil
}

func (c *MultiAgentCoordinator) requireCurrentCoordinationSnapshot(ctx context.Context, ac *activeCoordination) error {
	if ac.parentInteractionID == "" {
		return nil
	}
	ac.persistMu.Lock()
	current := ac.lastSnapshotJSON
	initialized := ac.snapshotInitialized
	ac.persistMu.Unlock()
	c.mu.Lock()
	stale := ac.staleSnapshot
	c.mu.Unlock()
	if stale || !initialized {
		return ErrInteractionCASConflict
	}
	_, latest, err := c.currentCoordinationRecord(ctx, ac)
	if err != nil {
		return err
	}
	if current != latest {
		c.markCoordinationStale(ac)
		return ErrInteractionCASConflict
	}
	return nil
}

func (c *MultiAgentCoordinator) refreshCoordinationIfStale(ctx context.Context, ac *activeCoordination) (*activeCoordination, error) {
	if ac == nil || ac.parentInteractionID == "" {
		return ac, nil
	}
	ac.persistMu.Lock()
	baseline := ac.lastSnapshotJSON
	initialized := ac.snapshotInitialized
	ac.persistMu.Unlock()
	record, latest, err := c.currentCoordinationRecord(ctx, ac)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	stale := ac.staleSnapshot
	current := c.coordinations[string(ac.id)]
	c.mu.Unlock()
	if current != ac {
		return current, nil
	}
	if initialized && baseline == latest && !stale {
		return ac, nil
	}
	c.markCoordinationStale(ac)
	shadow := NewMultiAgentCoordinator(c.tracker, c.goals, c.recovery, c.starter, c.pauseSvc, c.policy)
	if err := shadow.restoreCoordinationFromRecord(record); err != nil {
		return nil, fmt.Errorf("multi_agent: authoritative checkpoint could not be restored: %w", err)
	}
	replacement := shadow.coordinations[string(ac.id)]
	if replacement == nil {
		return nil, fmt.Errorf("multi_agent: missing replacement coordination checkpoint")
	}
	c.mu.Lock()
	if c.coordinations[string(ac.id)] == ac {
		c.coordinations[string(ac.id)] = replacement
	} else {
		replacement = c.coordinations[string(ac.id)]
	}
	c.mu.Unlock()
	return replacement, nil
}
