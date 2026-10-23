package safety

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/system/dataportability"
	"gorm.io/gorm"
)

const SafetyGovernanceComponentID = "safety.governance.v2"

type BackupContributor struct {
	db           *gorm.DB
	coordination *coordination.Service
}

type safetyArchive struct {
	SchemaVersion int           `json:"schemaVersion"`
	Policy        RuntimePolicy `json:"policy"`
	Incidents     []Incident    `json:"incidents"`
}

func NewBackupContributor(db *gorm.DB, coordinator ...*coordination.Service) *BackupContributor {
	contributor := &BackupContributor{db: db}
	if len(coordinator) > 0 {
		contributor.coordination = coordinator[0]
	}
	return contributor
}

func (*BackupContributor) ID() string   { return "safety" }
func (*BackupContributor) Name() string { return "安全治理" }
func (*BackupContributor) Dependencies() []string {
	return nil
}
func (*BackupContributor) ComponentIDs() []string {
	return []string{SafetyGovernanceComponentID}
}

func (c *BackupContributor) Plan(ctx context.Context, req dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	if req.EffectiveScope() != dataportability.ScopeAll && req.EffectiveScope() != dataportability.ScopeSettings {
		return nil, nil
	}
	scope, err := ScopeFromContextWithCoordinator(ctx, c.coordination)
	if err != nil {
		return nil, err
	}
	_, err = NewRepository(c.db).Policy(ctx, scope)
	if err != nil {
		return nil, err
	}
	incidents, err := NewRepository(c.db).AllIncidents(ctx, scope)
	if err != nil {
		return nil, err
	}
	return []dataportability.BackupComponentPlan{{
		ID:            SafetyGovernanceComponentID,
		Kind:          dataportability.KindMetadata,
		LogicalName:   "safety.governance.v2",
		Required:      false,
		SourceOfTruth: true,
		Rebuildable:   false,
		Sensitive:     true,
		ItemCount:     int64(len(incidents) + 1),
		EstimatedSize: int64(len(incidents)*2048 + 4096),
	}}, nil
}

func (c *BackupContributor) Export(ctx context.Context, req dataportability.BackupRequest, out dataportability.BackupWriter) error {
	if req.EffectiveScope() != dataportability.ScopeAll && req.EffectiveScope() != dataportability.ScopeSettings {
		return nil
	}
	scope, err := ScopeFromContextWithCoordinator(ctx, c.coordination)
	if err != nil {
		return err
	}
	repository := NewRepository(c.db)
	policy, err := repository.Policy(ctx, scope)
	if err != nil {
		return err
	}
	incidents, err := repository.AllIncidents(ctx, scope)
	if err != nil {
		return err
	}
	return out.WriteJSON(SafetyGovernanceComponentID, safetyArchive{SchemaVersion: 1, Policy: policy, Incidents: incidents})
}

func (c *BackupContributor) PreviewImport(ctx context.Context, req dataportability.ImportPreviewRequest, in dataportability.BackupReader) ([]dataportability.ImportComponentPreview, error) {
	archive, err := readSafetyArchive(in)
	if err != nil {
		return nil, err
	}
	return []dataportability.ImportComponentPreview{{
		ComponentID: SafetyGovernanceComponentID,
		Kind:        dataportability.KindMetadata,
		LogicalName: "safety.governance.v2",
		ItemCount:   int64(len(archive.Incidents) + 1),
	}}, nil
}

func (c *BackupContributor) Import(ctx context.Context, req dataportability.ImportRequest, in dataportability.BackupReader) error {
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}
	archive, err := readSafetyArchive(in)
	if err != nil {
		return err
	}
	if archive.SchemaVersion != 1 {
		return errors.New("安全治理备份版本不受支持")
	}
	scope, err := ScopeFromContextWithCoordinator(ctx, c.coordination)
	if err != nil {
		return err
	}
	return c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		revision := archive.Policy.Revision
		if revision < 1 {
			revision = 1
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		policyRaw, err := json.Marshal(archive.Policy.Config)
		if err != nil {
			return err
		}
		bdiRaw, err := json.Marshal(archive.Policy.BDI)
		if err != nil {
			return err
		}
		if req.CollisionPolicy == dataportability.CollisionReplace {
			err = tx.Exec(`INSERT INTO safety_policies(space_id,owner_id,core_id,revision,config_json,bdi_json,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(space_id,owner_id) DO UPDATE SET core_id=excluded.core_id,revision=safety_policies.revision+1,config_json=excluded.config_json,bdi_json=excluded.bdi_json,updated_at=excluded.updated_at`, scope.SpaceID, scope.OwnerID, scope.CoreID, revision, string(policyRaw), string(bdiRaw), now, now).Error
		} else {
			err = tx.Exec(`INSERT INTO safety_policies(space_id,owner_id,core_id,revision,config_json,bdi_json,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(space_id,owner_id) DO NOTHING`, scope.SpaceID, scope.OwnerID, scope.CoreID, revision, string(policyRaw), string(bdiRaw), now, now).Error
		}
		if err != nil {
			return err
		}
		for _, incident := range archive.Incidents {
			incident.SpaceID, incident.OwnerID, incident.CoreID = scope.SpaceID, scope.OwnerID, scope.CoreID
			incident.LegacyEventID = ""
			var existing Incident
			lookup := tx.Where("id=?", incident.ID).Take(&existing)
			if lookup.Error == nil {
				if req.CollisionPolicy == dataportability.CollisionSkip {
					continue
				}
				if req.CollisionPolicy == dataportability.CollisionDuplicate {
					incident.ID = "safety-incident-" + uuid.NewString()
				}
			}
			if lookup.Error != nil && !errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
				return lookup.Error
			}
			if existing.ID != "" && req.CollisionPolicy == dataportability.CollisionReplace {
				if err := tx.Where("id=?", existing.ID).Delete(&Incident{}).Error; err != nil {
					return err
				}
			}
			incident.FeedbackID = nil
			if err := tx.Create(&incident).Error; err != nil {
				if req.CollisionPolicy == dataportability.CollisionDuplicate {
					incident.ID = "safety-incident-" + uuid.NewString()
					if retryErr := tx.Create(&incident).Error; retryErr != nil {
						return retryErr
					}
					continue
				}
				return err
			}
		}
		return nil
	})
}

func readSafetyArchive(in dataportability.BackupReader) (safetyArchive, error) {
	var archive safetyArchive
	if err := in.ReadJSON(SafetyGovernanceComponentID, &archive); err != nil {
		return safetyArchive{}, err
	}
	return archive, nil
}
