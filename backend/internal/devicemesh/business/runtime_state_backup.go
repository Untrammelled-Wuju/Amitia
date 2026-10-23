package business

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/system/dataportability"
)

const runtimeStateBackupComponent = "runtime_state.records"

type runtimeStateBackupRecord struct {
	OwnerID   string          `json:"ownerId"`
	Kind      string          `json:"kind"`
	ID        string          `json:"id"`
	RoleID    string          `json:"roleId"`
	Revision  int64           `json:"revision"`
	UpdatedAt string          `json:"updatedAt"`
	Body      json.RawMessage `json:"body"`
}

type RuntimeStateBackupContributor struct {
	db *sql.DB
}

func NewRuntimeStateBackupContributor(db *sql.DB) *RuntimeStateBackupContributor {
	return &RuntimeStateBackupContributor{db: db}
}

func (*RuntimeStateBackupContributor) ID() string   { return "runtime-state" }
func (*RuntimeStateBackupContributor) Name() string { return "Runtime State" }
func (*RuntimeStateBackupContributor) ComponentIDs() []string {
	return []string{runtimeStateBackupComponent}
}
func (*RuntimeStateBackupContributor) Dependencies() []string {
	return []string{"character"}
}

func (c *RuntimeStateBackupContributor) Plan(ctx context.Context, req dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	if req.EffectiveScope() != dataportability.ScopeAll && req.EffectiveScope() != dataportability.ScopeCharacter {
		return nil, nil
	}
	count, err := c.count(ctx, req)
	if err != nil {
		return nil, err
	}
	return []dataportability.BackupComponentPlan{{
		ID:            runtimeStateBackupComponent,
		Kind:          dataportability.KindNDJSON,
		LogicalName:   "runtime_state.records.v1",
		Required:      false,
		SourceOfTruth: true,
		ItemCount:     count,
		EstimatedSize: count * 2048,
	}}, nil
}

func (c *RuntimeStateBackupContributor) Export(ctx context.Context, req dataportability.BackupRequest, out dataportability.BackupWriter) error {
	if req.EffectiveScope() != dataportability.ScopeAll && req.EffectiveScope() != dataportability.ScopeCharacter {
		return nil
	}
	writer, err := out.CreateComponent(runtimeStateBackupComponent, "runtime_state.records.v1", dataportability.KindNDJSON)
	if err != nil {
		return err
	}
	defer writer.Close()
	query, args := c.query(req, `owner_id,kind,resource_id,role_id,revision,updated_at,CAST(body AS TEXT)`)
	rows, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	buffer := bufio.NewWriter(writer)
	for rows.Next() {
		var record runtimeStateBackupRecord
		var body string
		if err := rows.Scan(&record.OwnerID, &record.Kind, &record.ID, &record.RoleID, &record.Revision, &record.UpdatedAt, &body); err != nil {
			return err
		}
		record.Body = json.RawMessage(body)
		encoded, err := json.Marshal(record)
		if err != nil {
			return err
		}
		if _, err := buffer.Write(append(encoded, '\n')); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return buffer.Flush()
}

func (c *RuntimeStateBackupContributor) PreviewImport(ctx context.Context, req dataportability.ImportPreviewRequest, in dataportability.BackupReader) ([]dataportability.ImportComponentPreview, error) {
	reader, err := in.ReadComponent(runtimeStateBackupComponent)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	count, err := countRuntimeStateRecords(reader)
	if err != nil {
		return nil, err
	}
	return []dataportability.ImportComponentPreview{{ComponentID: runtimeStateBackupComponent, Kind: dataportability.KindNDJSON, LogicalName: "runtime_state.records.v1", ItemCount: count}}, nil
}

func (c *RuntimeStateBackupContributor) Import(ctx context.Context, req dataportability.ImportRequest, in dataportability.BackupReader) error {
	reader, err := in.ReadComponent(runtimeStateBackupComponent)
	if err != nil {
		return err
	}
	defer reader.Close()
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for scanner.Scan() {
		var record runtimeStateBackupRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return err
		}
		if record.OwnerID == "" || record.ID == "" || record.RoleID == "" || record.Revision < 1 || !coordination.RuntimeStateKind(record.Kind) || !json.Valid(record.Body) {
			return errors.New("角色运行状态备份记录无效")
		}
		if req.CharacterPolicy == dataportability.CollisionSkip {
			var exists int
			if err := tx.QueryRowContext(ctx, `SELECT 1 FROM kernel_device_owned_resources WHERE owner_id=? AND kind=? AND resource_id=?`, record.OwnerID, record.Kind, record.ID).Scan(&exists); err == nil {
				continue
			} else if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,source_id,revision,deleted,body,updated_at) VALUES(?,?,?,?,'',?,0,?,?) ON CONFLICT(owner_id,kind,resource_id) DO UPDATE SET role_id=excluded.role_id,revision=excluded.revision,deleted=0,body=excluded.body,updated_at=excluded.updated_at`, record.OwnerID, record.Kind, record.ID, record.RoleID, record.Revision, []byte(record.Body), record.UpdatedAt); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return tx.Commit()
}

func (c *RuntimeStateBackupContributor) count(ctx context.Context, req dataportability.BackupRequest) (int64, error) {
	query, args := c.query(req, `COUNT(*)`)
	var count int64
	if err := c.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (c *RuntimeStateBackupContributor) query(req dataportability.BackupRequest, selection string) (string, []any) {
	query := fmt.Sprintf(`SELECT %s FROM kernel_device_owned_resources WHERE deleted=0 AND kind IN ('psyche','relationship','needs')`, selection)
	args := make([]any, 0, 1)
	if req.EffectiveScope() == dataportability.ScopeCharacter && req.CharacterID != "" {
		query += ` AND role_id=?`
		args = append(args, req.CharacterID)
	}
	if selection != `COUNT(*)` {
		query += ` ORDER BY owner_id,role_id,kind,resource_id`
	}
	return query, args
}

func countRuntimeStateRecords(reader io.Reader) (int64, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	var count int64
	for scanner.Scan() {
		var record runtimeStateBackupRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return 0, err
		}
		if !coordination.RuntimeStateKind(record.Kind) || !json.Valid(record.Body) {
			return 0, errors.New("角色运行状态备份记录无效")
		}
		count++
	}
	return count, scanner.Err()
}
