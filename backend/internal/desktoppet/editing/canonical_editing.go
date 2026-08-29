package editing

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
)

const (
	canonicalSourceManualEdit             = "manual_edit"
	canonicalSourceFullActionRegeneration = "full_action_regeneration"
	canonicalOriginUser                   = "user"
	canonicalContentHashVersionManifestV1 = "manifest_v1"
)

type draftActionConfigSnapshot struct {
	DefaultFPS         int    `json:"defaultFps"`
	LoopType           string `json:"loopType"`
	ReturnAction       string `json:"returnAction"`
	Interruptible      bool   `json:"interruptible"`
	PriorityOverride   *int   `json:"priorityOverride,omitempty"`
	CooldownMSOverride *int   `json:"cooldownMsOverride,omitempty"`
}

type draftSnapshotHashInput struct {
	SessionID           string `json:"sessionId"`
	SessionVersion      int64  `json:"sessionVersion"`
	BaseRevisionID      string `json:"baseRevisionId"`
	BaseContentHash     string `json:"baseContentHash"`
	BaseBindingRevision int64  `json:"baseBindingRevision"`
	ActionConfigHash    string `json:"actionConfigHash"`
	FrameSetHash        string `json:"frameSetHash"`
}

func marshalCanonicalJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func draftActionConfig(ds *draftState) draftActionConfigSnapshot {
	return draftActionConfigSnapshot{
		DefaultFPS:         ds.DefaultFPS,
		LoopType:           ds.LoopType,
		ReturnAction:       ds.ReturnAction,
		Interruptible:      ds.Interruptible,
		PriorityOverride:   ds.PriorityOverride,
		CooldownMSOverride: ds.CooldownMSOverride,
	}
}

// allocateActionStreamRevisionNumber serializes revision-number allocation on
// the canonical ActionStream. The old per-processing-task allocator is only
// used for genuinely legacy revisions that do not belong to an ActionStream.
func allocateActionStreamRevisionNumber(repo Repository, streamID string) (int, error) {
	if streamID == "" {
		return 0, fmt.Errorf("action stream id is required")
	}
	allocated := 0
	err := repo.DB().Transaction(func(tx *gorm.DB) error {
		for attempt := 0; attempt < 8; attempt++ {
			var stream ActionStream
			if err := tx.Where("id = ?", streamID).First(&stream).Error; err != nil {
				return err
			}
			stored := stream.NextRevisionNumber
			next := stored
			if next < 1 {
				next = 1
			}
			result := tx.Model(&ActionStream{}).
				Where("id = ? AND next_revision_number = ?", streamID, stored).
				Update("next_revision_number", next+1)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 1 {
				allocated = int(next)
				return nil
			}
		}
		return fmt.Errorf("action stream revision allocation conflict: %s", streamID)
	})
	if err != nil {
		return 0, err
	}
	return allocated, nil
}

func (s *service) validateSessionBaseBinding(session *EditSession) (*resolvedActiveRevisionBinding, error) {
	binding, err := resolveActiveRevisionBinding(s.repo, session.ProcessingTaskID, session.ActionKey)
	if err != nil {
		return nil, err
	}
	if binding == nil || binding.RevisionID != session.BaseRevisionID || binding.BindingRevision != session.BaseBindingRevision {
		return nil, ErrSessionStale
	}
	if session.ActionStreamID != "" && binding.ActionStreamID != session.ActionStreamID {
		return nil, ErrSessionStale
	}
	return binding, nil
}

func (s *service) ensureDraftSnapshot(ctx context.Context, session *EditSession) (*EditDraftSnapshot, error) {
	if session == nil {
		return nil, ErrSessionNotFound
	}
	if existing, err := s.repo.GetDraftSnapshotBySession(session.ID, session.SessionVersion); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}

	ds, err := s.rebuildDraftState(ctx, session)
	if err != nil {
		return nil, err
	}
	baseRev, err := s.repo.GetActionRevision(session.BaseRevisionID)
	if err != nil {
		return nil, err
	}

	configJSON, err := marshalCanonicalJSON(draftActionConfig(ds))
	if err != nil {
		return nil, err
	}
	framesJSON, err := marshalCanonicalJSON(ds.Frames)
	if err != nil {
		return nil, err
	}
	configHash := s.assetStore.ComputeHash([]byte(configJSON))
	frameSetHash := s.assetStore.ComputeHash([]byte(framesJSON))
	baseContentHash := session.BaseActionContentHash
	if baseContentHash == "" {
		baseContentHash = baseRev.ContentHash
	}
	hashInputJSON, err := marshalCanonicalJSON(draftSnapshotHashInput{
		SessionID:           session.ID,
		SessionVersion:      session.SessionVersion,
		BaseRevisionID:      session.BaseRevisionID,
		BaseContentHash:     baseContentHash,
		BaseBindingRevision: session.BaseBindingRevision,
		ActionConfigHash:    configHash,
		FrameSetHash:        frameSetHash,
	})
	if err != nil {
		return nil, err
	}

	snapshot := &EditDraftSnapshot{
		ID:                       generateID("draft"),
		SessionID:                session.ID,
		SessionVersion:           session.SessionVersion,
		UserID:                   session.UserID,
		CharacterID:              session.CharacterID,
		ActionStreamID:           session.ActionStreamID,
		ActionKey:                session.ActionKey,
		BaseRevisionID:           session.BaseRevisionID,
		BaseContentHash:          baseContentHash,
		BaseBindingRevision:      session.BaseBindingRevision,
		ActionConfigSnapshotJSON: configJSON,
		ActionConfigHash:         configHash,
		FramesJSON:               framesJSON,
		FrameSetHash:             frameSetHash,
		SnapshotHash:             s.assetStore.ComputeHash([]byte(hashInputJSON)),
		CreatedAt:                nowUTC(),
	}
	if err := s.repo.CreateDraftSnapshot(snapshot); err != nil {
		// A concurrent identical request can win the unique (session, version)
		// insert. Read that immutable snapshot instead of creating a fork.
		existing, readErr := s.repo.GetDraftSnapshotBySession(session.ID, session.SessionVersion)
		if readErr != nil || existing == nil {
			return nil, err
		}
		snapshot = existing
	}

	result := s.db.Model(&EditSession{}).
		Where("id = ? AND session_version = ?", session.ID, session.SessionVersion).
		Updates(map[string]any{
			"draft_snapshot_id":   snapshot.ID,
			"draft_snapshot_hash": snapshot.SnapshotHash,
			"updated_at":          nowUTC(),
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, ErrSessionStale
	}
	session.DraftSnapshotID = snapshot.ID
	session.DraftSnapshotHash = snapshot.SnapshotHash
	return snapshot, nil
}

func (s *service) revisionHashesFromDraft(ds *draftState, manifestHash string) (actionConfigJSON, actionConfigHash, frameSetHash, revisionSnapshotJSON, revisionSnapshotHash string, err error) {
	actionConfigJSON, err = marshalCanonicalJSON(draftActionConfig(ds))
	if err != nil {
		return
	}
	actionConfigHash = s.assetStore.ComputeHash([]byte(actionConfigJSON))
	framesJSON, marshalErr := marshalCanonicalJSON(ds.Frames)
	if marshalErr != nil {
		err = marshalErr
		return
	}
	frameSetHash = s.assetStore.ComputeHash([]byte(framesJSON))
	revisionSnapshotJSON, err = marshalCanonicalJSON(map[string]any{
		"manifestHash":     manifestHash,
		"actionConfigHash": actionConfigHash,
		"frameSetHash":     frameSetHash,
	})
	if err != nil {
		return
	}
	revisionSnapshotHash = s.assetStore.ComputeHash([]byte(revisionSnapshotJSON))
	return
}
