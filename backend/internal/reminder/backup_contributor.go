package reminder

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/u-ai/backend/internal/system/dataportability"
)

const TaskBackupComponent = "reminder.owned.v1"
const taskArchiveSignatureDomain = "amitia-owned-tasks-v1\x00"

type TaskArchiveRuntime func(context.Context) (*Service, Caller, error)
type TaskBackupContributor struct {
	runtime TaskArchiveRuntime
	signer  crypto.Signer
}
type signedTaskArchive struct {
	Archive   TaskArchive `json:"archive"`
	Signature string      `json:"signature"`
}

func NewTaskBackupContributor(runtime TaskArchiveRuntime, signer crypto.Signer) *TaskBackupContributor {
	return &TaskBackupContributor{runtime: runtime, signer: signer}
}
func (*TaskBackupContributor) ID() string   { return "reminder" }
func (*TaskBackupContributor) Name() string { return "日程提醒与主动任务" }
func (*TaskBackupContributor) Dependencies() []string {
	return []string{"character", "chat", "extension"}
}
func (c *TaskBackupContributor) current(ctx context.Context) (*Service, Caller, error) {
	if c.runtime == nil || c.signer == nil {
		return nil, Caller{}, ErrUnavailable
	}
	service, caller, err := c.runtime(ctx)
	if err != nil {
		return nil, Caller{}, err
	}
	if !service.Ready() || caller.SpaceID == "" || caller.DeviceID == "" || caller.PluginID != "" {
		return nil, Caller{}, ErrForbidden
	}
	if _, ok := c.signer.Public().(ed25519.PublicKey); !ok {
		return nil, Caller{}, errors.New("统一任务备份身份签名不可用")
	}
	return service, caller, nil
}
func (c *TaskBackupContributor) Plan(ctx context.Context, req dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	if req.Scope == dataportability.ScopeSettings {
		return nil, nil
	}
	service, caller, err := c.current(ctx)
	if err != nil {
		return nil, err
	}
	archive, err := service.ExportTaskArchive(ctx, caller, req.CharacterID)
	if err != nil {
		return nil, err
	}
	return []dataportability.BackupComponentPlan{{ID: TaskBackupComponent, LogicalName: TaskBackupComponent, Kind: dataportability.KindDataset, Required: true, SourceOfTruth: true, Sensitive: true, ItemCount: int64(len(archive.Tasks))}}, nil
}
func taskArchiveSigningBytes(archive TaskArchive) ([]byte, error) {
	bytes, err := json.Marshal(archive)
	return append([]byte(taskArchiveSignatureDomain), bytes...), err
}
func (c *TaskBackupContributor) Export(ctx context.Context, req dataportability.BackupRequest, out dataportability.BackupWriter) error {
	if req.Scope == dataportability.ScopeSettings {
		return nil
	}
	service, caller, err := c.current(ctx)
	if err != nil {
		return err
	}
	archive, err := service.ExportTaskArchive(ctx, caller, req.CharacterID)
	if err != nil {
		return err
	}
	bytes, err := taskArchiveSigningBytes(archive)
	if err != nil {
		return err
	}
	signature, err := c.signer.Sign(rand.Reader, bytes, crypto.Hash(0))
	if err != nil {
		return err
	}
	w, err := out.CreateComponent(TaskBackupComponent, TaskBackupComponent, dataportability.KindDataset)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(w).Encode(signedTaskArchive{Archive: archive, Signature: base64.RawURLEncoding.EncodeToString(signature)}); err != nil {
		_ = w.Close()
		return err
	}
	return w.Close()
}
func (c *TaskBackupContributor) read(ctx context.Context, in dataportability.BackupReader) (*TaskArchive, error) {
	present := false
	for _, component := range in.ListComponents() {
		if component == TaskBackupComponent {
			present = true
		}
		if (strings.HasPrefix(component, "reminder.") || strings.HasPrefix(component, "schedule.") || strings.HasPrefix(component, "proactive.")) && component != TaskBackupComponent {
			return nil, errors.New("历史日程提醒备份格式不再支持")
		}
	}
	if !present {
		return nil, nil
	}
	service, caller, err := c.current(ctx)
	if err != nil {
		return nil, err
	}
	r, err := in.ReadComponent(TaskBackupComponent)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	limited := &io.LimitedReader{R: r, N: 64<<20 + 1}
	decoder := json.NewDecoder(limited)
	decoder.DisallowUnknownFields()
	var signed signedTaskArchive
	if err := decoder.Decode(&signed); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF || limited.N <= 0 {
		return nil, errors.New("统一任务备份包含额外内容或超过大小限制")
	}
	bytes, err := taskArchiveSigningBytes(signed.Archive)
	if err != nil {
		return nil, err
	}
	signature, err := base64.RawURLEncoding.DecodeString(signed.Signature)
	if err != nil || !ed25519.Verify(c.signer.Public().(ed25519.PublicKey), bytes, signature) {
		return nil, errors.New("统一任务备份来源签名校验失败")
	}
	if err := validateTaskArchive(signed.Archive); err != nil {
		return nil, err
	}
	if signed.Archive.OwnerID != service.deps.OwnerID || signed.Archive.Exporter.SpaceID != caller.SpaceID || signed.Archive.Exporter.DeviceID != caller.DeviceID {
		return nil, ErrForbidden
	}
	return &signed.Archive, nil
}
func (c *TaskBackupContributor) PreviewImport(ctx context.Context, _ dataportability.ImportPreviewRequest, in dataportability.BackupReader) ([]dataportability.ImportComponentPreview, error) {
	archive, err := c.read(ctx, in)
	if err != nil || archive == nil {
		return nil, err
	}
	return []dataportability.ImportComponentPreview{{ComponentID: TaskBackupComponent, Kind: dataportability.KindDataset, LogicalName: TaskBackupComponent, ItemCount: int64(len(archive.Tasks)), Warnings: []string{"恢复后任务默认暂停；启用前重新校验所属角色、插件权限与当前执行 Owner。执行历史仅供查看，不会重新触发。"}}}, nil
}
func (c *TaskBackupContributor) Import(ctx context.Context, req dataportability.ImportRequest, in dataportability.BackupReader) error {
	archive, err := c.read(ctx, in)
	if err != nil || archive == nil {
		return err
	}
	service, caller, err := c.current(ctx)
	if err != nil {
		return err
	}
	return service.RestoreTaskArchive(ctx, caller, req, *archive)
}
