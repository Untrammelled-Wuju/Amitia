package business

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

const EmotionSnapshotKind = "emotion"

type EmotionSnapshot struct {
	ID                 string          `json:"id"`
	SpaceID            string          `json:"spaceId"`
	RoleID             string          `json:"roleId"`
	ConversationID     string          `json:"conversationId,omitempty"`
	MessageID          string          `json:"messageId,omitempty"`
	RequestID          string          `json:"requestId"`
	Source             string          `json:"source"`
	DetectionEnabled   bool            `json:"detectionEnabled"`
	DetectionThreshold float64         `json:"detectionThreshold"`
	Confidence         float64         `json:"confidence"`
	Valence            float64         `json:"valence"`
	Arousal            float64         `json:"arousal"`
	Dominance          float64         `json:"dominance"`
	Stress             float64         `json:"stress"`
	Fatigue            float64         `json:"fatigue"`
	MoodValence        float64         `json:"moodValence"`
	MoodArousal        float64         `json:"moodArousal"`
	MoodPressure       float64         `json:"moodPressure"`
	EmotionLabel       string          `json:"emotionLabel"`
	MoodLabel          string          `json:"moodLabel"`
	AffectLabel        string          `json:"affectLabel"`
	RecoveryApplied    bool            `json:"recoveryApplied,omitempty"`
	RecoveredMood      string          `json:"recoveredMood,omitempty"`
	RecoveredLevel     int             `json:"recoveredLevel,omitempty"`
	LegacyKind         string          `json:"legacyKind,omitempty"`
	LegacyPayload      json.RawMessage `json:"legacyPayload,omitempty"`
	CreatedAt          string          `json:"createdAt"`
}

func (e *Engine) ImportEmotionSnapshot(ctx context.Context, request Request, snapshot EmotionSnapshot) error {
	if e == nil || e.coordination == nil || e.data == nil {
		return errors.New("情绪历史服务未就绪")
	}
	snapshot.ID = strings.TrimSpace(snapshot.ID)
	snapshot.RoleID = strings.TrimSpace(snapshot.RoleID)
	if snapshot.ID == "" || snapshot.RoleID == "" || snapshot.RequestID == "" {
		return errors.New("情绪历史缺少编号或角色")
	}
	request.RoleID = snapshot.RoleID
	ctx, scope, finish, err := e.coordination.Begin(ctx, request.SpaceID, request.DeviceID, request.TargetDeviceID, request.CoreID, request.RoleID, snapshot.RequestID)
	if err != nil {
		return err
	}
	defer finish()
	if err := e.coordination.RequireCapability(ctx, scope.SpaceID, scope.InitiatorDeviceID, scope.TargetDeviceID, "ai.chat"); err != nil {
		return err
	}
	roles, err := e.data.Roles(ctx, scope)
	if err != nil {
		return err
	}
	role, err := coordination.ResolveRole(snapshot.RoleID, roles)
	if err != nil {
		return err
	}
	scope.RoleID, scope.RoleRevision = role.ID, role.Revision
	ctx = coordination.WithScope(ctx, scope)
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return errors.New("情绪历史数据端口未就绪")
	}
	existing, err := port.Resource(ctx, scope, EmotionSnapshotKind, snapshot.ID)
	if err != nil {
		return err
	}
	if existing != nil {
		if existing.Deleted {
			return nil
		}
		var current EmotionSnapshot
		if json.Unmarshal(existing.Body, &current) != nil || current.RoleID != snapshot.RoleID {
			return coordination.ErrWrongOwner
		}
		return nil
	}
	snapshot.SpaceID = scope.SpaceID
	snapshot.RoleID = scope.RoleID
	if snapshot.CreatedAt == "" {
		snapshot.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	_, err = e.commit(ctx, coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: EmotionSnapshotKind, ID: snapshot.ID, RoleID: scope.RoleID, Body: body(snapshot)}}})
	return err
}
