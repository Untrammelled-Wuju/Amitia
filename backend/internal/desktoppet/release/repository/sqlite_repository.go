package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/release"
	"gorm.io/gorm"
)

type SQLiteRepository struct {
	db *gorm.DB
}

func NewSQLiteRepository(db *gorm.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) DB() *gorm.DB {
	return r.db
}

func (r *SQLiteRepository) Transaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

func (r *SQLiteRepository) CreateBuildSnapshot(snapshot *release.ReleaseBuildSnapshot) error {
	return r.db.Create(snapshot).Error
}

func (r *SQLiteRepository) GetBuildSnapshot(id string) (*release.ReleaseBuildSnapshot, error) {
	var s release.ReleaseBuildSnapshot
	if err := r.db.First(&s, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SQLiteRepository) GetBuildSnapshotByInputHash(inputHash string) (*release.ReleaseBuildSnapshot, error) {
	var s release.ReleaseBuildSnapshot
	if err := r.db.First(&s, "input_hash = ?", inputHash).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SQLiteRepository) CreateBuildOperation(op *release.ReleaseBuildOperation) error {
	return r.db.Create(op).Error
}

func (r *SQLiteRepository) GetBuildOperation(id string) (*release.ReleaseBuildOperation, error) {
	var op release.ReleaseBuildOperation
	if err := r.db.First(&op, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &op, nil
}

func (r *SQLiteRepository) GetBuildOperationByIdempotencyKey(userID, idempotencyKey string) (*release.ReleaseBuildOperation, error) {
	var op release.ReleaseBuildOperation
	if err := r.db.First(&op, "user_id = ? AND idempotency_key = ?", userID, idempotencyKey).Error; err != nil {
		return nil, err
	}
	return &op, nil
}

func (r *SQLiteRepository) UpdateBuildOperation(op *release.ReleaseBuildOperation) error {
	return r.db.Save(op).Error
}

func (r *SQLiteRepository) ListPendingBuildOperations() ([]*release.ReleaseBuildOperation, error) {
	var ops []*release.ReleaseBuildOperation
	if err := r.db.Where("state IN (?)", []string{
		release.BuildOpStateCreated,
		release.BuildOpStateBuilding,
		release.BuildOpStateValidating,
		release.BuildOpStatePublishing,
	}).Find(&ops).Error; err != nil {
		return nil, err
	}
	return ops, nil
}

func (r *SQLiteRepository) ListStaleBuildOperations(leaseExpiryBefore string) ([]*release.ReleaseBuildOperation, error) {
	var ops []*release.ReleaseBuildOperation
	if err := r.db.Where("lease_expires_at < ? AND state NOT IN (?)",
		leaseExpiryBefore,
		[]string{release.BuildOpStateCompleted, release.BuildOpStateCancelled},
	).Find(&ops).Error; err != nil {
		return nil, err
	}
	return ops, nil
}

func (r *SQLiteRepository) CreatePublishJournal(journal *release.ReleasePublishJournal) error {
	return r.db.Create(journal).Error
}

func (r *SQLiteRepository) GetPublishJournalByOperation(operationID string) (*release.ReleasePublishJournal, error) {
	var j release.ReleasePublishJournal
	if err := r.db.First(&j, "operation_id = ?", operationID).Error; err != nil {
		return nil, err
	}
	return &j, nil
}

func (r *SQLiteRepository) UpdatePublishJournal(journal *release.ReleasePublishJournal) error {
	return r.db.Save(journal).Error
}

func (r *SQLiteRepository) ListPendingPublishJournals() ([]*release.ReleasePublishJournal, error) {
	var journals []*release.ReleasePublishJournal
	if err := r.db.Where("stage NOT IN (?)", []string{
		release.JournalStageCompleted,
		release.JournalStageFailed,
	}).Find(&journals).Error; err != nil {
		return nil, err
	}
	return journals, nil
}

func (r *SQLiteRepository) CreateLegacyPackageMapping(mapping *release.LegacyPackageMapping) error {
	return r.db.Create(mapping).Error
}

func (r *SQLiteRepository) GetLegacyPackageMapping(legacyPackageID string) (*release.LegacyPackageMapping, error) {
	var m release.LegacyPackageMapping
	if err := r.db.First(&m, "legacy_package_id = ?", legacyPackageID).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *SQLiteRepository) UpdateLegacyPackageMapping(mapping *release.LegacyPackageMapping) error {
	return r.db.Save(mapping).Error
}

func (r *SQLiteRepository) ListPendingLegacyMappings() ([]*release.LegacyPackageMapping, error) {
	var mappings []*release.LegacyPackageMapping
	if err := r.db.Where("migration_status = ?", release.LegacyMigrationStatusPending).Find(&mappings).Error; err != nil {
		return nil, err
	}
	return mappings, nil
}

func (r *SQLiteRepository) CreateLegacyMigrationOperation(op *release.LegacyPackageMigrationOperation) error {
	return r.db.Create(op).Error
}

func (r *SQLiteRepository) GetLegacyMigrationOperation(id string) (*release.LegacyPackageMigrationOperation, error) {
	var op release.LegacyPackageMigrationOperation
	if err := r.db.First(&op, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &op, nil
}

func (r *SQLiteRepository) UpdateLegacyMigrationOperation(op *release.LegacyPackageMigrationOperation) error {
	return r.db.Save(op).Error
}

func (r *SQLiteRepository) GetPetIdentity(petID string) (*release.PetIdentityData, error) {
	var record petIdentityRecord
	if err := r.db.First(&record, "id = ?", petID).Error; err != nil {
		return nil, err
	}
	return record.toData(), nil
}

func (r *SQLiteRepository) GetPetIdentityByCharacter(userID, characterID string) (*release.PetIdentityData, error) {
	var record petIdentityRecord
	if err := r.db.First(&record, "owner_user_id = ? AND source_character_id = ?", userID, characterID).Error; err != nil {
		return nil, err
	}
	return record.toData(), nil
}

func (r *SQLiteRepository) CreatePetIdentity(identity *release.PetIdentityData) error {
	record := petIdentityRecord{
		ID:                  identity.ID,
		OwnerUserID:         identity.OwnerUserID,
		SourceCharacterID:   identity.SourceCharacterID,
		Name:                identity.Name,
		Slug:                identity.Slug,
		BindingPolicy:       identity.BindingPolicy,
		UpstreamPetID:       identity.UpstreamPetID,
		DefaultActionKey:    identity.DefaultActionKey,
		NextReleaseSequence: identity.NextReleaseSequence,
		CreatedAt:           identity.CreatedAt,
		UpdatedAt:           identity.UpdatedAt,
	}
	return r.db.Create(&record).Error
}

func (r *SQLiteRepository) CreatePetIdentityTx(tx *gorm.DB, identity *release.PetIdentityData) error {
	record := petIdentityRecord{
		ID:                  identity.ID,
		OwnerUserID:         identity.OwnerUserID,
		SourceCharacterID:   identity.SourceCharacterID,
		Name:                identity.Name,
		Slug:                identity.Slug,
		BindingPolicy:       identity.BindingPolicy,
		UpstreamPetID:       identity.UpstreamPetID,
		DefaultActionKey:    identity.DefaultActionKey,
		NextReleaseSequence: identity.NextReleaseSequence,
		CreatedAt:           identity.CreatedAt,
		UpdatedAt:           identity.UpdatedAt,
	}
	return tx.Create(&record).Error
}

func (r *SQLiteRepository) UpdatePetIdentity(identity *release.PetIdentityData) error {
	record := petIdentityRecord{
		ID:                  identity.ID,
		OwnerUserID:         identity.OwnerUserID,
		SourceCharacterID:   identity.SourceCharacterID,
		Name:                identity.Name,
		Slug:                identity.Slug,
		BindingPolicy:       identity.BindingPolicy,
		UpstreamPetID:       identity.UpstreamPetID,
		DefaultActionKey:    identity.DefaultActionKey,
		NextReleaseSequence: identity.NextReleaseSequence,
		CreatedAt:           identity.CreatedAt,
		UpdatedAt:           identity.UpdatedAt,
	}
	return r.db.Save(&record).Error
}

func (r *SQLiteRepository) UpdatePetIdentityTx(tx *gorm.DB, identity *release.PetIdentityData) error {
	record := petIdentityRecord{
		ID:                  identity.ID,
		OwnerUserID:         identity.OwnerUserID,
		SourceCharacterID:   identity.SourceCharacterID,
		Name:                identity.Name,
		Slug:                identity.Slug,
		BindingPolicy:       identity.BindingPolicy,
		UpstreamPetID:       identity.UpstreamPetID,
		DefaultActionKey:    identity.DefaultActionKey,
		NextReleaseSequence: identity.NextReleaseSequence,
		CreatedAt:           identity.CreatedAt,
		UpdatedAt:           identity.UpdatedAt,
	}
	return tx.Save(&record).Error
}

func (r *SQLiteRepository) GetReleaseByContentHash(contentRootHash string) (*release.ReleaseData, error) {
	var data release.ReleaseData
	if err := r.db.First(&data, "content_root_hash = ?", contentRootHash).Error; err != nil {
		return nil, err
	}
	return &data, nil
}

func (r *SQLiteRepository) GetRelease(releaseID string) (*release.ReleaseData, error) {
	var data release.ReleaseData
	if err := r.db.First(&data, "id = ?", releaseID).Error; err != nil {
		return nil, err
	}
	return &data, nil
}

func (r *SQLiteRepository) CreateRelease(releaseData *release.ReleaseData) error {
	return r.db.Create(releaseData).Error
}

func (r *SQLiteRepository) UpdateRelease(releaseData *release.ReleaseData) error {
	return r.db.Save(releaseData).Error
}

func (r *SQLiteRepository) ListReleasesByPet(petID string) ([]*release.ReleaseData, error) {
	var releases []*release.ReleaseData
	if err := r.db.Where("pet_id = ?", petID).Find(&releases).Error; err != nil {
		return nil, err
	}
	return releases, nil
}

func (r *SQLiteRepository) ListPublishedReleases(userID string) ([]*release.ReleaseData, error) {
	var releases []*release.ReleaseData
	if err := r.db.Where("owner_user_id = ?", userID).Find(&releases).Error; err != nil {
		return nil, err
	}
	return releases, nil
}

func (r *SQLiteRepository) CreateReleaseFiles(files []release.ReleaseFileData) error {
	if len(files) == 0 {
		return nil
	}
	return r.db.CreateInBatches(files, 100).Error
}

func (r *SQLiteRepository) GetReleaseFiles(releaseID string) ([]release.ReleaseFileData, error) {
	var files []release.ReleaseFileData
	if err := r.db.Where("release_id = ?", releaseID).Find(&files).Error; err != nil {
		return nil, err
	}
	return files, nil
}

func (r *SQLiteRepository) CreateValidationReport(report *release.ReleaseValidationReport) error {
	return r.db.Create(report).Error
}

func (r *SQLiteRepository) GetValidationReport(releaseID string) (*release.ReleaseValidationReport, error) {
	var report release.ReleaseValidationReport
	if err := r.db.First(&report, "release_id = ?", releaseID).Error; err != nil {
		return nil, err
	}
	return &report, nil
}

func (r *SQLiteRepository) CreateEventOutbox(event *release.ReleaseEventOutbox) error {
	return r.db.Create(event).Error
}

func (r *SQLiteRepository) ListPendingOutboxEvents(limit int) ([]*release.ReleaseEventOutbox, error) {
	var events []*release.ReleaseEventOutbox
	if err := r.db.Where("status = ?", "pending").
		Order("created_at ASC").
		Limit(limit).
		Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

func (r *SQLiteRepository) UpdateOutboxEvent(event *release.ReleaseEventOutbox) error {
	return r.db.Save(event).Error
}

func (r *SQLiteRepository) CreateBuildRequestInbox(inbox *release.ReleaseBuildRequestInbox) error {
	return r.db.Create(inbox).Error
}

func (r *SQLiteRepository) GetBuildRequestInbox(requestID string) (*release.ReleaseBuildRequestInbox, error) {
	var inbox release.ReleaseBuildRequestInbox
	if err := r.db.First(&inbox, "request_id = ?", requestID).Error; err != nil {
		return nil, err
	}
	return &inbox, nil
}

func (r *SQLiteRepository) UpdateBuildRequestInbox(inbox *release.ReleaseBuildRequestInbox) error {
	return r.db.Save(inbox).Error
}

func (r *SQLiteRepository) CreateImportSnapshot(snapshot *release.ImportPackageSnapshot) error {
	return r.db.Create(snapshot).Error
}

func (r *SQLiteRepository) CreateImportSnapshotTx(tx *gorm.DB, snapshot *release.ImportPackageSnapshot) error {
	return tx.Create(snapshot).Error
}

func (r *SQLiteRepository) GetImportSnapshot(stagingID string) (*release.ImportPackageSnapshot, error) {
	var snapshot release.ImportPackageSnapshot
	if err := r.db.First(&snapshot, "import_staging_id = ?", stagingID).Error; err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (r *SQLiteRepository) GetImportSnapshotByReleaseID(releaseID string) (*release.ImportPackageSnapshot, error) {
	var snapshot release.ImportPackageSnapshot
	if err := r.db.First(&snapshot, "release_id = ?", releaseID).Error; err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (r *SQLiteRepository) UpdateImportSnapshot(snapshot *release.ImportPackageSnapshot) error {
	return r.db.Save(snapshot).Error
}

func (r *SQLiteRepository) AcquireLeaseCAS(op *release.ReleaseBuildOperation, owner, executionID string) error {
	now := time.Now()
	expiresAt := now.Add(5 * time.Minute)

	result := r.db.Model(&release.ReleaseBuildOperation{}).
		Where("id = ? AND state IN (?) AND (lease_owner = '' OR lease_expires_at <= ?)",
			op.ID, []string{
				release.BuildOpStateCreated,
				release.BuildOpStateFailedRetryable,
			}, now).
		Updates(map[string]interface{}{
			"lease_owner":      owner,
			"execution_id":     executionID,
			"lease_expires_at": expiresAt.Format("2006-01-02 15:04:05"),
			"heartbeat_at":     now.Format("2006-01-02 15:04:05"),
			"updated_at":       now.Format("2006-01-02 15:04:05"),
		})

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("lease acquisition failed: operation %s is not in acquirable state or already leased", op.ID)
	}
	return nil
}

func (r *SQLiteRepository) CreateSnapshotTx(tx *gorm.DB, snapshot *release.ReleaseBuildSnapshot) error {
	return tx.Create(snapshot).Error
}

func (r *SQLiteRepository) CreateOperationTx(tx *gorm.DB, op *release.ReleaseBuildOperation) error {
	return tx.Create(op).Error
}

func (r *SQLiteRepository) CreateReleaseTx(tx *gorm.DB, releaseData *release.ReleaseData) error {
	return tx.Create(releaseData).Error
}

func (r *SQLiteRepository) CreateReleaseFilesTx(tx *gorm.DB, files []release.ReleaseFileData) error {
	if len(files) == 0 {
		return nil
	}
	return tx.CreateInBatches(files, 100).Error
}

func (r *SQLiteRepository) CreateValidationReportTx(tx *gorm.DB, report *release.ReleaseValidationReport) error {
	return tx.Create(report).Error
}

func (r *SQLiteRepository) CreateOutboxTx(tx *gorm.DB, event *release.ReleaseEventOutbox) error {
	return tx.Create(event).Error
}

func (r *SQLiteRepository) UpdateOperationOwned(tx *gorm.DB, op *release.ReleaseBuildOperation, expectedState string) error {
	now := time.Now()
	data := map[string]interface{}{
		"state":      op.State,
		"stage":      op.Stage,
		"updated_at": now.Format("2006-01-02 15:04:05"),
	}
	if op.ErrorCode != "" {
		data["error_code"] = op.ErrorCode
	}
	if op.ErrorMessage != "" {
		data["error_message"] = op.ErrorMessage
	}
	if op.ResultJSON != "" {
		data["result_json"] = op.ResultJSON
	}
	if op.CompletedAt != "" {
		data["completed_at"] = op.CompletedAt
	}

	result := tx.Model(&release.ReleaseBuildOperation{}).
		Where("id = ? AND lease_owner = ? AND state = ?",
			op.ID, op.LeaseOwner, expectedState).
		Updates(data)

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("operation %s state update conflict: expected state=%s owner=%s", op.ID, expectedState, op.LeaseOwner)
	}
	return nil
}

type petIdentityRecord struct {
	ID                  string `gorm:"column:id;primaryKey;type:text"`
	OwnerUserID         string `gorm:"column:owner_user_id;type:text;index:idx_identity_owner_character,unique"`
	SourceCharacterID   string `gorm:"column:source_character_id;type:text;index:idx_identity_owner_character,unique"`
	Name                string `gorm:"column:name;type:text"`
	Slug                string `gorm:"column:slug;type:text"`
	BindingPolicy       string `gorm:"column:binding_policy;type:text;index:idx_identity_owner_character,unique"`
	UpstreamPetID       string `gorm:"column:upstream_pet_id;type:text"`
	DefaultActionKey    string `gorm:"column:default_action_key;type:text"`
	NextReleaseSequence int    `gorm:"column:next_release_sequence;type:integer;default:0"`
	CreatedAt           string `gorm:"column:created_at;type:text"`
	UpdatedAt           string `gorm:"column:updated_at;type:text"`
}

func (petIdentityRecord) TableName() string {
	return "desktop_pet_identities"
}

func (r *petIdentityRecord) toData() *release.PetIdentityData {
	return &release.PetIdentityData{
		ID:                  r.ID,
		OwnerUserID:         r.OwnerUserID,
		SourceCharacterID:   r.SourceCharacterID,
		Name:                r.Name,
		Slug:                r.Slug,
		BindingPolicy:       r.BindingPolicy,
		UpstreamPetID:       r.UpstreamPetID,
		DefaultActionKey:    r.DefaultActionKey,
		NextReleaseSequence: r.NextReleaseSequence,
		CreatedAt:           r.CreatedAt,
		UpdatedAt:           r.UpdatedAt,
	}
}

func hashCanonical(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
