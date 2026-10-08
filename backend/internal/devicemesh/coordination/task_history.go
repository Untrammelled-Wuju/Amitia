package coordination

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"strings"
)

type TaskReadProof struct {
	Scope                 ExecutionScope `json:"executionScope"`
	TaskRunID             string         `json:"taskRunId"`
	DefinitionFingerprint string         `json:"definitionFingerprint"`
}

type TaskHistoryResourcePort interface {
	ReadTaskResource(context.Context, ExecutionScope, TaskReadProof, string, string) (*Resource, error)
}

type TaskHistoryAuthorityPort interface {
	OpenTaskRead(context.Context, TaskReadProof) (context.Context, func(), error)
}

type taskReadAuthorityKey struct{}

type taskReadAuthority struct {
	current ExecutionScope
	proof   TaskReadProof
}

func ValidateTaskReadProof(current ExecutionScope, proof TaskReadProof) error {
	saved := proof.Scope
	hash, err := hex.DecodeString(proof.DefinitionFingerprint)
	coreConsole := saved.Coordinated && saved.ResourceOwnerID == saved.CoreID && current.Coordinated && current.ResourceOwnerID == current.CoreID && current.RoleOwnerID == current.CoreID && current.InitiatorDeviceID != "" && current.TargetDeviceID == current.InitiatorDeviceID
	if err != nil || len(hash) != 32 || proof.TaskRunID == "" || len(proof.TaskRunID) > 256 || strings.ContainsRune(proof.TaskRunID, 0) || saved.CoreID == "" || saved.SpaceID != saved.CoreID || saved.AuthorizationRealm != saved.CoreID || saved.CoreID != current.CoreID || saved.SpaceID != current.SpaceID || current.AuthorizationRealm != current.CoreID || saved.TargetDeviceID == "" || saved.TargetDeviceID != current.TargetDeviceID && !coreConsole || saved.InitiatorDeviceID == "" || saved.RoleID == "" || saved.RoleRevision < 1 || saved.ProviderEpoch < 1 || saved.TargetProviderEpoch < 1 || saved.PermissionRevision < 1 || saved.TargetPermissionRevision < 1 || saved.ModeRevision < 1 || saved.RequestID == "" || saved.ExecutionID == "" || saved.TurnID == "" || saved.RoleOwnerID != saved.ResourceOwnerID {
		return ErrWrongOwner
	}
	owner := saved.TargetDeviceID
	if saved.Coordinated {
		owner = saved.CoreID
	}
	if saved.ResourceOwnerID != owner {
		return ErrWrongOwner
	}
	return nil
}

func WithTaskReadAuthority(ctx context.Context, current ExecutionScope, proof TaskReadProof) (context.Context, error) {
	actual, ok := FromContext(ctx)
	if !ok || actual != current {
		return ctx, ErrWrongOwner
	}
	if err := ValidateTaskReadProof(current, proof); err != nil {
		return ctx, err
	}
	if err := ValidateCurrent(ctx); err != nil {
		return ctx, err
	}
	return context.WithValue(WithScope(ctx, proof.Scope), taskReadAuthorityKey{}, taskReadAuthority{current: current, proof: proof}), nil
}

func TaskReadAuthority(ctx context.Context) (ExecutionScope, TaskReadProof, bool) {
	authority, ok := ctx.Value(taskReadAuthorityKey{}).(taskReadAuthority)
	return authority.current, authority.proof, ok
}

func ValidateTaskReadResourceID(proof TaskReadProof, kind, id string) error {
	if kind != "checkpoint" || id == "" || len(id) > 512 || strings.ContainsRune(id, 0) {
		return ErrWrongOwner
	}
	if id == "task/input/"+proof.TaskRunID || id == "task/storage/"+proof.TaskRunID || id == "task/artifacts/"+proof.TaskRunID || strings.HasPrefix(id, "task/outcome/"+proof.TaskRunID+"/") || strings.HasPrefix(id, "task/progress/"+proof.TaskRunID+"/") || strings.HasPrefix(id, "task/artifact/") || strings.HasPrefix(id, "task/checkpoint/") {
		return nil
	}
	return ErrWrongOwner
}

func ValidateTaskReadResource(proof TaskReadProof, kind, id string, resource *Resource) error {
	if err := ValidateTaskReadResourceID(proof, kind, id); err != nil {
		return err
	}
	if resource == nil {
		return nil
	}
	if resource.Deleted || resource.OwnerID != proof.Scope.ResourceOwnerID || resource.RoleID != proof.Scope.RoleID || resource.Kind != kind || resource.ID != id || len(resource.Body) > 4<<20 {
		return ErrWrongOwner
	}
	var document struct {
		Scope                 ExecutionScope `json:"executionScope"`
		DefinitionFingerprint string         `json:"definitionFingerprint"`
		TaskRunID             string         `json:"taskRunId"`
		Checkpoint            struct {
			TaskRunID string `json:"taskRunId"`
		} `json:"checkpoint"`
	}
	if json.Unmarshal(resource.Body, &document) != nil || document.Scope != proof.Scope || document.DefinitionFingerprint != proof.DefinitionFingerprint {
		return ErrWrongOwner
	}
	if document.TaskRunID == proof.TaskRunID || document.Checkpoint.TaskRunID == proof.TaskRunID || strings.HasPrefix(id, "task/progress/"+proof.TaskRunID+"/") {
		return nil
	}
	return ErrWrongOwner
}
