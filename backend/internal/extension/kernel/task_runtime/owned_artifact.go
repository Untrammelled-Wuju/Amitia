package task_runtime

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type OwnedTaskArtifactPort interface {
	Call(context.Context, *TaskRun, string, string, json.RawMessage) (json.RawMessage, error)
	Result(context.Context, *TaskRun, string) ([]byte, string, error)
}

type AcknowledgedTaskArtifactPort struct {
	Data coordination.DataPort
}

type ownedTaskArtifactMetadata struct {
	ArtifactID string          `json:"artifactId"`
	Kind       string          `json:"kind"`
	MimeType   string          `json:"mimeType,omitempty"`
	Size       int             `json:"size"`
	Handle     string          `json:"handle"`
	Name       string          `json:"name"`
	Hash       string          `json:"hash"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
}

type ownedTaskArtifactIndex struct {
	Scope                 coordination.ExecutionScope `json:"executionScope"`
	DefinitionFingerprint string                      `json:"definitionFingerprint"`
	TaskRunID             string                      `json:"taskRunId"`
	Artifacts             []ownedTaskArtifactMetadata `json:"artifacts"`
}

type ownedTaskArtifactDocument struct {
	Scope                 coordination.ExecutionScope `json:"executionScope"`
	DefinitionFingerprint string                      `json:"definitionFingerprint"`
	TaskRunID             string                      `json:"taskRunId"`
	Artifact              ownedTaskArtifactMetadata   `json:"artifact"`
	Content               []byte                      `json:"contentBytes"`
}

func (p AcknowledgedTaskArtifactPort) Call(ctx context.Context, run *TaskRun, requestID, method string, params json.RawMessage) (json.RawMessage, error) {
	scope, err := taskInputScope(ctx, run)
	if err != nil {
		return nil, err
	}
	port, ok := p.Data.(coordination.ResourcePort)
	if !ok || run.Generation < 1 || run.ExecutionAttemptID == "" || len(requestID) < 1 || len(requestID) > 256 || len(params) > 1500<<10 || method != "task.artifact.saveData" && method != "task.artifact.saveFile" && method != "task.artifact.list" {
		return nil, NewTaskError(ErrTaskScopeDenied, "任务产物端口、执行身份或请求无效")
	}
	var input struct {
		TaskRunID string          `json:"task_run_id"`
		Name      string          `json:"name"`
		Data      json.RawMessage `json:"data"`
		Content   string          `json:"content"`
		Options   struct {
			Kind     string          `json:"kind"`
			MimeType string          `json:"mimeType"`
			Metadata json.RawMessage `json:"metadata"`
		} `json:"options"`
	}
	if json.Unmarshal(params, &input) != nil || input.TaskRunID != run.TaskRunID {
		return nil, NewTaskError(ErrTaskScopeDenied, "任务产物归属无效")
	}
	if _, err := (AcknowledgedTaskInputPort{Data: p.Data}).Input(ctx, run); err != nil {
		return nil, err
	}
	indexID := "task/artifacts/" + run.TaskRunID
	resource, err := port.Resource(ctx, scope, "checkpoint", indexID)
	if err != nil {
		return nil, err
	}
	index := ownedTaskArtifactIndex{Scope: scope, DefinitionFingerprint: run.DefinitionFingerprint, TaskRunID: run.TaskRunID, Artifacts: make([]ownedTaskArtifactMetadata, 0)}
	version := int64(0)
	if resource != nil {
		if resource.Deleted || resource.OwnerID != scope.ResourceOwnerID || resource.RoleID != scope.RoleID || resource.ID != indexID || resource.Kind != "checkpoint" || resource.Revision < 1 || len(resource.Body) > 192<<10 || json.Unmarshal(resource.Body, &index) != nil || index.Scope != scope || index.DefinitionFingerprint != run.DefinitionFingerprint || index.TaskRunID != run.TaskRunID || len(index.Artifacts) > 32 {
			return nil, NewTaskError(ErrTaskScopeDenied, "任务产物索引与原授权不一致")
		}
		version = resource.Revision
	}
	if method == "task.artifact.list" {
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return nil, err
		}
		return json.Marshal(index.Artifacts)
	}
	if len(input.Name) < 1 || len(input.Name) > 256 || strings.ContainsAny(input.Name, "/\\\x00") || len(input.Options.MimeType) > 128 || len(input.Options.Metadata) > 4096 || len(input.Options.Metadata) > 0 && !json.Valid(input.Options.Metadata) {
		return nil, NewTaskError(ErrTaskScopeDenied, "任务产物名称或元数据无效")
	}
	var content []byte
	if method == "task.artifact.saveData" {
		if !json.Valid(input.Data) || len(input.Data) > 1<<20 {
			return nil, coordination.ErrPendingLimit
		}
		content = append([]byte(nil), input.Data...)
		if input.Options.Kind == "" {
			input.Options.Kind = "data"
		}
		if input.Options.MimeType == "" {
			input.Options.MimeType = "application/json"
		}
	} else {
		if len(input.Content) > (1<<20)*4/3+4 {
			return nil, coordination.ErrPendingLimit
		}
		content, err = base64.StdEncoding.DecodeString(input.Content)
		if err != nil || len(content) > 1<<20 {
			return nil, coordination.ErrPendingLimit
		}
		if input.Options.Kind == "" {
			input.Options.Kind = "file"
		}
	}
	switch input.Options.Kind {
	case "file", "image", "audio", "data", "report":
	default:
		return nil, NewTaskError(ErrTaskScopeDenied, "任务产物类型无效")
	}
	artifactID := "artifact-" + hashBytes([]byte(run.TaskRunID+"\x00"+run.ExecutionAttemptID.String()+"\x00"+requestID))
	hash := hashBytes(content)
	size := len(content)
	metadata := ownedTaskArtifactMetadata{ArtifactID: artifactID, Kind: input.Options.Kind, MimeType: input.Options.MimeType, Size: size, Handle: "owned-task-artifact/" + url.PathEscape(scope.ResourceOwnerID) + "/" + url.PathEscape(run.TaskRunID) + "/" + artifactID, Name: input.Name, Hash: hash, Metadata: input.Options.Metadata}
	document := ownedTaskArtifactDocument{Scope: scope, DefinitionFingerprint: run.DefinitionFingerprint, TaskRunID: run.TaskRunID, Artifact: metadata, Content: content}
	body, err := json.Marshal(document)
	if err != nil || len(body) > 1500<<10 {
		return nil, coordination.ErrPendingLimit
	}
	total := size
	for _, existing := range index.Artifacts {
		if existing.Size < 0 || existing.Size > 1<<20 {
			return nil, coordination.ErrPendingLimit
		}
		total += existing.Size
		if existing.ArtifactID == artifactID {
			storedMetadata, err := json.Marshal(existing)
			if err != nil {
				return nil, coordination.ErrRequestConflict
			}
			requestedMetadata, err := json.Marshal(metadata)
			if err != nil || !bytes.Equal(storedMetadata, requestedMetadata) {
				return nil, coordination.ErrRequestConflict
			}
			stored, err := port.Resource(ctx, scope, "checkpoint", "task/artifact/"+artifactID)
			if err != nil {
				return nil, err
			}
			if stored == nil || stored.Deleted || stored.OwnerID != scope.ResourceOwnerID || stored.RoleID != scope.RoleID || stored.Kind != "checkpoint" || stored.ID != "task/artifact/"+artifactID || stored.Revision != 1 || !bytes.Equal(stored.Body, body) {
				return nil, coordination.ErrRequestConflict
			}
			if err := coordination.ValidateCurrent(ctx); err != nil {
				return nil, err
			}
			return requestedMetadata, nil
		}
	}
	if len(index.Artifacts) >= 32 || total > 8<<20 {
		return nil, coordination.ErrPendingLimit
	}
	index.Artifacts = append(index.Artifacts, metadata)
	indexBody, err := json.Marshal(index)
	if err != nil || len(indexBody) > 192<<10 {
		return nil, coordination.ErrPendingLimit
	}
	commitScope := scope
	commitScope.RequestID += "|task-artifact|" + artifactID
	id := "task/artifact/" + artifactID
	ack, err := p.Data.Commit(ctx, coordination.Commit{Scope: commitScope, Dependencies: []coordination.ResourceVersion{{Kind: "checkpoint", ID: "task/input/" + run.TaskRunID, Revision: 1}}, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: id, RoleID: scope.RoleID, Body: body}, {Kind: "checkpoint", ID: indexID, RoleID: scope.RoleID, ExpectedRevision: version, Body: indexBody}}})
	if err != nil {
		return nil, err
	}
	if ack.OwnerID != scope.ResourceOwnerID || ack.RequestID != commitScope.RequestID || ack.Versions["checkpoint/"+id] != 1 || ack.Versions["checkpoint/"+indexID] != version+1 {
		return nil, NewTaskError(ErrTaskScopeDenied, "任务产物所有者尚未确认保存")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	return json.Marshal(metadata)
}

func (p AcknowledgedTaskArtifactPort) Result(ctx context.Context, run *TaskRun, artifactID string) ([]byte, string, error) {
	scope, err := taskInputScope(ctx, run)
	if err != nil {
		return nil, "", err
	}
	port, ok := p.Data.(coordination.ResourcePort)
	if !ok || len(artifactID) != 73 || !strings.HasPrefix(artifactID, "artifact-") {
		return nil, "", NewTaskError(ErrTaskScopeDenied, "任务产物结果引用无效")
	}
	if _, err := hex.DecodeString(strings.TrimPrefix(artifactID, "artifact-")); err != nil {
		return nil, "", NewTaskError(ErrTaskScopeDenied, "任务产物编号无效")
	}
	if _, err := (AcknowledgedTaskInputPort{Data: p.Data}).Input(ctx, run); err != nil {
		return nil, "", err
	}
	id := "task/artifact/" + artifactID
	resource, err := port.Resource(ctx, scope, "checkpoint", id)
	if err != nil {
		return nil, "", err
	}
	if resource == nil || resource.Deleted || resource.OwnerID != scope.ResourceOwnerID || resource.RoleID != scope.RoleID || resource.Kind != "checkpoint" || resource.ID != id || resource.Revision != 1 || len(resource.Body) > 1500<<10 {
		return nil, "", coordination.ErrWrongOwner
	}
	var document ownedTaskArtifactDocument
	if json.Unmarshal(resource.Body, &document) != nil || document.Scope != scope || document.DefinitionFingerprint != run.DefinitionFingerprint || document.TaskRunID != run.TaskRunID || document.Artifact.ArtifactID != artifactID || document.Artifact.MimeType != "application/json" || document.Artifact.Kind != "data" || len(document.Content) > 1<<20 || !json.Valid(document.Content) || document.Artifact.Size != len(document.Content) || document.Artifact.Hash != hashBytes(document.Content) {
		return nil, "", NewTaskError(ErrTaskScopeDenied, "任务产物结果与原授权或摘要不一致")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, "", err
	}
	return append([]byte(nil), document.Content...), document.Artifact.Hash, nil
}
