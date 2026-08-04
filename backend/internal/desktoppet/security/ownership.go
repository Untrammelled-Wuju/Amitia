// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package security

import (
	"context"
	"errors"
	"fmt"

	desktoppetAuth "github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/pkg/comment/response"
)

type CharacterScope struct {
	UserID      string
	CharacterID string
}

type GenerationTaskScope struct {
	UserID      string
	TaskID      string
	CharacterID string
}

type ProcessingTaskScope struct {
	UserID      string
	TaskID      string
	CharacterID string
}

type ActionRevisionScope struct {
	UserID           string
	CharacterID      string
	RevisionID       string
	ProcessingTaskID string
}

type QualityScope struct {
	UserID           string
	EvaluationID     string
	ActionRevisionID string
	CharacterID      string
	ProcessingTaskID string
}

type ReleaseScope struct {
	UserID    string
	ReleaseID string
	PetID     string
}

type InstallationScope struct {
	UserID         string
	DeviceID       string
	InstallationID string
}

type EditSessionScope struct {
	UserID    string
	SessionID string
}

type RegenerationJobScope struct {
	UserID         string
	JobID          string
	SessionID      string
	BaseRevisionID string
}

type CandidateScope struct {
	UserID      string
	CandidateID string
	JobID       string
	SessionID   string
}

type RuntimeCommandScope struct {
	UserID    string
	DeviceID  string
	RuntimeID string
	CommandID string
}

type BehaviorBindingScope struct {
	UserID         string
	BindingID      string
	InstallationID string
}

type ActionStreamScope struct {
	UserID      string
	StreamID    string
	CharacterID string
}

var (
	ErrUnauthorized = errors.New("desktoppet: unauthorized")
	ErrForbidden    = errors.New("desktoppet: forbidden")
	ErrNotFound     = errors.New("desktoppet: resource not found")
)

type OwnershipGuard interface {
	RequireCharacter(ctx context.Context, actor *desktoppetAuth.ActorContext, characterID string) (*CharacterScope, error)
	RequireGenerationTask(ctx context.Context, actor *desktoppetAuth.ActorContext, taskID string) (*GenerationTaskScope, error)
	RequireProcessingTask(ctx context.Context, actor *desktoppetAuth.ActorContext, taskID string) (*ProcessingTaskScope, error)
	RequireActionRevision(ctx context.Context, actor *desktoppetAuth.ActorContext, revisionID string) (*ActionRevisionScope, error)
	RequireActionStream(ctx context.Context, actor *desktoppetAuth.ActorContext, streamID string) (*ActionStreamScope, error)
	RequireQualityEvaluation(ctx context.Context, actor *desktoppetAuth.ActorContext, evaluationID string) (*QualityScope, error)
	RequireRelease(ctx context.Context, actor *desktoppetAuth.ActorContext, releaseID string) (*ReleaseScope, error)
	RequireInstallation(ctx context.Context, actor *desktoppetAuth.ActorContext, deviceID, installationID string) (*InstallationScope, error)
	RequireInstallationStrict(ctx context.Context, actor *desktoppetAuth.ActorContext, deviceID, installationID string) (*InstallationScope, error)
	RequireEditSession(ctx context.Context, actor *desktoppetAuth.ActorContext, sessionID string) (*EditSessionScope, error)
	RequireRegenerationJob(ctx context.Context, actor *desktoppetAuth.ActorContext, jobID string) (*RegenerationJobScope, error)
	RequireCandidate(ctx context.Context, actor *desktoppetAuth.ActorContext, candidateID string) (*CandidateScope, error)
	RequireRuntimeCommand(ctx context.Context, actor *desktoppetAuth.ActorContext, commandID string) (*RuntimeCommandScope, error)
	RequireBehaviorBinding(ctx context.Context, actor *desktoppetAuth.ActorContext, bindingID string) (*BehaviorBindingScope, error)
}

func ResolveResourceError(err error) error {
	if errors.Is(err, ErrUnauthorized) {
		return fmt.Errorf("%w: authentication required", ErrUnauthorized)
	}
	if errors.Is(err, ErrForbidden) {
		return fmt.Errorf("%w: access denied to resource", ErrForbidden)
	}
	if errors.Is(err, ErrNotFound) {
		return fmt.Errorf("%w: resource not found or access denied", ErrNotFound)
	}
	return err
}

func SanitizeErrorForClient(err error) error {
	if errors.Is(err, ErrUnauthorized) {
		return ErrUnauthorized
	}
	if errors.Is(err, ErrForbidden) {
		return ErrForbidden
	}
	if errors.Is(err, ErrNotFound) {
		return ErrNotFound
	}
	return err
}

type OwnershipError struct {
	Code    int
	ErrCode string
	Msg     string
}

func (e *OwnershipError) Error() string { return e.Msg }

func MapOwnershipError(err error) *OwnershipError {
	if errors.Is(err, ErrUnauthorized) {
		return &OwnershipError{Code: response.Unauthorized, ErrCode: "AUTH_REQUIRED", Msg: "认证失败"}
	}
	if errors.Is(err, ErrForbidden) {
		return &OwnershipError{Code: response.Forbidden, ErrCode: "FORBIDDEN", Msg: "无权访问该资源"}
	}
	if errors.Is(err, ErrNotFound) {
		return &OwnershipError{Code: response.NotFound, ErrCode: "NOT_FOUND", Msg: "资源不存在"}
	}
	return nil
}
