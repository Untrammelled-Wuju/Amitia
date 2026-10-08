package task_runtime

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type TaskArtifactDownload struct {
	ArtifactID string
	Content    []byte
	Name       string
	MimeType   string
	Hash       string
}

type TaskArtifactDownloadPort interface {
	Download(context.Context, *TaskRun, string) (*TaskArtifactDownload, error)
}

func (p AcknowledgedTaskArtifactPort) Download(ctx context.Context, run *TaskRun, artifactID string) (*TaskArtifactDownload, error) {
	scope, err := taskInputScope(ctx, run)
	if err != nil {
		return nil, err
	}
	port, ok := p.Data.(coordination.ResourcePort)
	if !ok || len(artifactID) != 73 || !strings.HasPrefix(artifactID, "artifact-") {
		return nil, NewTaskError(ErrTaskScopeDenied, "任务产物下载引用无效")
	}
	if _, err := hex.DecodeString(strings.TrimPrefix(artifactID, "artifact-")); err != nil {
		return nil, NewTaskError(ErrTaskScopeDenied, "任务产物下载编号无效")
	}
	if _, err := (AcknowledgedTaskInputPort{Data: p.Data}).Input(ctx, run); err != nil {
		return nil, err
	}
	id := "task/artifact/" + artifactID
	resource, err := port.Resource(ctx, scope, "checkpoint", id)
	if err != nil {
		return nil, err
	}
	if resource == nil || resource.Deleted || resource.OwnerID != scope.ResourceOwnerID || resource.RoleID != scope.RoleID || resource.ID != id || resource.Kind != "checkpoint" || resource.Revision != 1 || len(resource.Body) > 1500<<10 {
		return nil, coordination.ErrWrongOwner
	}
	var document ownedTaskArtifactDocument
	if json.Unmarshal(resource.Body, &document) != nil || document.Scope != scope || document.DefinitionFingerprint != run.DefinitionFingerprint || document.TaskRunID != run.TaskRunID || document.Artifact.ArtifactID != artifactID || len(document.Content) > 1<<20 || document.Artifact.Size != len(document.Content) || document.Artifact.Hash != hashBytes(document.Content) || len(document.Artifact.Name) > 256 || len(document.Artifact.MimeType) > 128 {
		return nil, NewTaskError(ErrTaskScopeDenied, "任务产物下载内容与原授权或摘要不一致")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	return &TaskArtifactDownload{ArtifactID: artifactID, Content: append([]byte(nil), document.Content...), Name: document.Artifact.Name, MimeType: document.Artifact.MimeType, Hash: document.Artifact.Hash}, nil
}

func (s *TaskRuntimeService) DownloadTaskArtifact(ctx context.Context, taskRunID, artifactID string) (*TaskArtifactDownload, error) {
	if _, owned := coordination.FromContext(ctx); !owned {
		return nil, NewTaskError(ErrTaskScopeDenied, "此产物需要按任务的原始数据归属读取")
	}
	run, err := s.store.GetTaskRun(ctx, taskRunID)
	if err != nil || run == nil {
		return nil, NewTaskError(ErrTaskNotFound, "任务不存在")
	}
	if err := s.validateTaskReadScope(ctx, run); err != nil {
		return nil, err
	}
	port, ok := s.config.OwnedArtifacts.(TaskArtifactDownloadPort)
	if !ok {
		return nil, NewTaskError(ErrTaskScopeDenied, "任务所有者产物下载端口尚未就绪")
	}
	return port.Download(ctx, run, artifactID)
}

func (s *TaskRuntimeService) DownloadTaskResultArtifact(ctx context.Context, taskRunID, requestedArtifactID string) (*TaskArtifactDownload, error) {
	metadata, err := s.GetResult(ctx, taskRunID)
	if err != nil {
		return nil, err
	}
	if metadata == nil || metadata.TaskRunID != taskRunID || metadata.ResultType != ResultArtifact || metadata.ArtifactID == "" {
		return nil, NewTaskError(ErrTaskNotFound, "任务没有可下载的结果产物")
	}
	if requestedArtifactID != "" && requestedArtifactID != metadata.ArtifactID {
		return nil, NewTaskError(ErrTaskScopeDenied, "下载编号与任务确认的结果产物不一致")
	}
	download, err := s.DownloadTaskArtifact(ctx, taskRunID, metadata.ArtifactID)
	if err != nil {
		return nil, err
	}
	if download.Hash != metadata.ResultHash {
		return nil, NewTaskError(ErrTaskScopeDenied, "结果产物与调度保存的摘要不一致")
	}
	return download, nil
}
