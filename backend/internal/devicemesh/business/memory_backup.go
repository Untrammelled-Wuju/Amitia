package business

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/system/dataportability"
)

const OwnedMemoryBackupComponent = "memory.owned"

type MemoryArchive struct {
	Format  string                `json:"format"`
	Entries []MemoryConsumerEntry `json:"entries"`
}

type MemoryArchivePort interface {
	Archive(context.Context, string) (MemoryArchive, error)
	RestoreArchive(context.Context, dataportability.ImportRequest, MemoryArchive) error
}

type MemoryBackupContributor struct{ port MemoryArchivePort }

func NewMemoryBackupContributor(port MemoryArchivePort) *MemoryBackupContributor {
	return &MemoryBackupContributor{port: port}
}
func (*MemoryBackupContributor) ID() string             { return "memory" }
func (*MemoryBackupContributor) Name() string           { return "Memory" }
func (*MemoryBackupContributor) Dependencies() []string { return []string{"character", "chat"} }
func (*MemoryBackupContributor) ComponentIDs() []string { return []string{OwnedMemoryBackupComponent} }
func (c *MemoryBackupContributor) Plan(ctx context.Context, req dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	if req.EffectiveScope() != dataportability.ScopeAll && req.EffectiveScope() != dataportability.ScopeCharacter && req.EffectiveScope() != dataportability.ScopeMemory {
		return nil, nil
	}
	if c.port == nil {
		return nil, errors.New("统一记忆备份入口不可用")
	}
	return []dataportability.BackupComponentPlan{{ID: OwnedMemoryBackupComponent, LogicalName: "memory.owned.v1", Kind: dataportability.KindDataset, Required: true, SourceOfTruth: true, Sensitive: true}}, nil
}
func (c *MemoryBackupContributor) Export(ctx context.Context, req dataportability.BackupRequest, out dataportability.BackupWriter) error {
	if req.EffectiveScope() != dataportability.ScopeAll && req.EffectiveScope() != dataportability.ScopeCharacter && req.EffectiveScope() != dataportability.ScopeMemory {
		return nil
	}
	if c.port == nil {
		return errors.New("统一记忆备份入口不可用")
	}
	roleID := ""
	if req.EffectiveScope() == dataportability.ScopeCharacter {
		roleID = req.CharacterID
	}
	archive, err := c.port.Archive(ctx, roleID)
	if err != nil {
		return err
	}
	w, err := out.CreateComponent(OwnedMemoryBackupComponent, "memory.owned.v1", dataportability.KindDataset)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(w).Encode(archive); err != nil {
		_ = w.Close()
		return err
	}
	return w.Close()
}
func readMemoryArchive(in dataportability.BackupReader) (*MemoryArchive, error) {
	present := false
	for _, id := range in.ListComponents() {
		if id == OwnedMemoryBackupComponent {
			present = true
		}
		if strings.HasPrefix(id, "memory.") && id != OwnedMemoryBackupComponent || strings.HasPrefix(id, "episodic.") {
			return nil, errors.New("历史记忆备份格式不再支持，请使用统一记忆系统备份")
		}
	}
	if !present {
		return nil, nil
	}
	r, err := in.ReadComponent(OwnedMemoryBackupComponent)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	limited := &io.LimitedReader{R: r, N: 64<<20 + 1}
	decoder := json.NewDecoder(limited)
	decoder.DisallowUnknownFields()
	var archive MemoryArchive
	if err := decoder.Decode(&archive); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, errors.New("记忆备份包含额外内容")
	}
	if limited.N <= 0 || archive.Format != "amitia-owned-memory-v1" || len(archive.Entries) > 32768 {
		return nil, errors.New("记忆备份格式或大小无效")
	}
	seen := map[string]bool{}
	for _, entry := range archive.Entries {
		key := entry.Kind + "/" + entry.ID
		if seen[key] || entry.ID == "" || entry.OwnerID == "" || entry.RoleID == "" || entry.Deleted || entry.Revision < 1 || len(entry.Body) > 256<<10 || !json.Valid(entry.Body) {
			return nil, errors.New("记忆备份记录无效")
		}
		seen[key] = true
		switch entry.Kind {
		case "memory", "fact", "profile", "episodic", "graph", "vector", "working", "summary":
		case "checkpoint":
			if !strings.HasPrefix(entry.ID, "memory-candidate/") {
				return nil, errors.New("记忆备份候选记录无效")
			}
		default:
			return nil, fmt.Errorf("记忆备份类型无效：%s", entry.Kind)
		}
	}
	return &archive, nil
}
func (c *MemoryBackupContributor) PreviewImport(ctx context.Context, req dataportability.ImportPreviewRequest, in dataportability.BackupReader) ([]dataportability.ImportComponentPreview, error) {
	archive, err := readMemoryArchive(in)
	if err != nil || archive == nil {
		return nil, err
	}
	return []dataportability.ImportComponentPreview{{ComponentID: OwnedMemoryBackupComponent, Kind: dataportability.KindDataset, LogicalName: "memory.owned.v1", ItemCount: int64(len(archive.Entries))}}, ctx.Err()
}
func (c *MemoryBackupContributor) Import(ctx context.Context, req dataportability.ImportRequest, in dataportability.BackupReader) error {
	archive, err := readMemoryArchive(in)
	if err != nil || archive == nil {
		return err
	}
	if c.port == nil {
		return errors.New("统一记忆恢复入口不可用")
	}
	return c.port.RestoreArchive(ctx, req, *archive)
}

func (e *Engine) ExportMemoryArchive(ctx context.Context, request Request, role string) (MemoryArchive, error) {
	ctx, scope, finish, err := e.memoryManagementScope(ctx, request, false)
	if err != nil {
		return MemoryArchive{}, err
	}
	defer finish()
	roles, err := e.data.Roles(ctx, scope)
	if err != nil {
		return MemoryArchive{}, err
	}
	archive := MemoryArchive{Format: "amitia-owned-memory-v1", Entries: []MemoryConsumerEntry{}}
	seen := map[string]bool{}
	for _, actor := range roles {
		if role != "" && actor.ID != role {
			continue
		}
		current := scope
		current.RoleID, current.RoleRevision, current.MemoryViewAll = actor.ID, actor.Revision, false
		current.MemoryRoleID = actor.ID
		for _, kind := range []string{"memory", "fact", "episodic", "profile", "graph", "vector", "working", "summary", "checkpoint"} {
			snapshot, err := e.memoryManagementSnapshot(coordination.WithScope(ctx, current), current, kind, "")
			if err != nil {
				return MemoryArchive{}, err
			}
			for _, resource := range snapshot.Resources {
				if kind == "checkpoint" && !strings.HasPrefix(resource.ID, "memory-candidate/") {
					continue
				}
				key := kind + "/" + resource.ID
				if !seen[key] {
					seen[key] = true
					archive.Entries = append(archive.Entries, MemoryConsumerEntry{Resource: resource, Scope: current})
				}
			}
		}
	}
	if role != "" {
		if _, err := coordination.ResolveRole(role, roles); err != nil {
			return MemoryArchive{}, err
		}
	}
	if err := e.coordination.Validate(ctx, scope); err != nil {
		return MemoryArchive{}, err
	}
	if len(body(archive)) > 64<<20 {
		return MemoryArchive{}, coordination.ErrPendingLimit
	}
	return archive, nil
}
