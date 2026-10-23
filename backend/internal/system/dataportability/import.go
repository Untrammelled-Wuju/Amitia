package dataportability

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
)

type ImportPreviewResult struct {
	Manifest        *BackupManifest          `json:"manifest"`
	Components      []ImportComponentPreview `json:"components"`
	CanImport       bool                     `json:"canImport"`
	RequiresReindex bool                     `json:"requiresReindex"`
}

func (c *Coordinator) PreviewImport(ctx context.Context, archivePath string) (*ImportPreviewResult, error) {
	staging, err := c.Staging.CreateStaging()
	if err != nil {
		return nil, err
	}

	reader, err := NewArchiveReader(archivePath)
	if err != nil {
		c.Staging.CleanupStaging(staging)
		return nil, err
	}
	defer reader.Close()

	manifest, err := reader.ReadManifest()
	if err != nil {
		c.Staging.CleanupStaging(staging)
		return nil, ErrImportPackageInvalid
	}

	if manifest.Format != FormatName {
		return nil, ErrImportFormatUnsupported
	}
	if manifest.FormatVersion > FormatVersion {
		return nil, ErrImportFormatUnsupported
	}
	if manifest.SchemaFingerprint != "" && !compatibleSchemaFingerprint(manifest.SchemaFingerprint, c.SchemaFinger) {
		return nil, ErrRestoreSchemaUnsupported
	}

	req := ImportPreviewRequest{
		StagingPath: staging,
		Manifest:    manifest,
	}

	br := &archiveBackupReader{r: reader}
	var previews []ImportComponentPreview
	for _, ct := range c.Contributors {
		if !manifestContainsContributor(manifest, ct) {
			continue
		}
		p, err := ct.PreviewImport(ctx, req, br)
		if err != nil {
			return nil, fmt.Errorf("contributor %s preview failed: %w", ct.ID(), err)
		}
		previews = append(previews, p...)
	}

	c.Staging.CleanupStaging(staging)

	return &ImportPreviewResult{
		Manifest:        manifest,
		Components:      previews,
		CanImport:       true,
		RequiresReindex: manifest.RequiresReindex,
	}, nil
}

func (c *Coordinator) ExecuteImport(ctx context.Context, archivePath string, req ImportRequest) (*ImportIdentityMap, error) {
	identityMap, _, err := c.executeImportWithOperation(ctx, archivePath, req)
	return identityMap, err
}

func (c *Coordinator) executeImportWithOperation(ctx context.Context, archivePath string, req ImportRequest) (*ImportIdentityMap, *ImportOperation, error) {
	verification, err := VerifyArchive(archivePath)
	if err != nil {
		return nil, nil, err
	}
	if verification == nil || !verification.PackageIntegrity || verification.ComponentsVerified == 0 || verification.ComponentsFailed != 0 {
		return nil, nil, ErrBackupVerifyFailed
	}
	staging, err := c.Staging.CreateStaging()
	if err != nil {
		return nil, nil, err
	}
	defer c.Staging.CleanupStaging(staging)

	reader, err := NewArchiveReader(archivePath)
	if err != nil {
		return nil, nil, err
	}
	defer reader.Close()

	manifest, err := reader.ReadManifest()
	if err != nil {
		return nil, nil, ErrImportPackageInvalid
	}
	if manifest.Format != FormatName || manifest.FormatVersion > FormatVersion {
		return nil, nil, ErrImportFormatUnsupported
	}
	if manifest.SchemaFingerprint != "" && !compatibleSchemaFingerprint(manifest.SchemaFingerprint, c.SchemaFinger) {
		return nil, nil, ErrRestoreSchemaUnsupported
	}
	if err := c.validateManifestComponents(manifest); err != nil {
		return nil, nil, err
	}
	policy := req.CollisionPolicy
	if policy == "" {
		policy = req.CharacterPolicy
	}
	if policy != CollisionDuplicate && policy != CollisionSkip && policy != CollisionReplace {
		return nil, nil, ErrImportCollisionPolicyInvalid
	}

	req.Manifest = manifest
	req.StagingPath = staging
	req.OperationID = generateOpID()
	req.CollisionPolicy = policy
	req.CharacterPolicy = policy

	op := &ImportOperation{
		ID:        req.OperationID,
		Status:    "mapping",
		Scope:     string(manifest.Scope),
		CreatedAt: nowRFC3339(),
		Stats:     make(map[string]int64),
	}
	c.mu.Lock()
	c.importOps[req.OperationID] = op
	c.mu.Unlock()

	if req.IdentityMap == nil {
		req.IdentityMap = NewImportIdentityMap()
	}
	br := &archiveBackupReader{r: reader}

	sortedContributors := c.sortedContributorsForImport()

	op.Status = "importing"
	for _, ct := range sortedContributors {
		if !manifestContainsContributor(manifest, ct) {
			continue
		}
		if err := ct.Import(ctx, req, br); err != nil {
			op.Status = "failed"
			op.Error = err.Error()
			op.UpdatedAt = nowRFC3339()
			return req.IdentityMap, op, ErrImportTransactionFailed
		}
		op.Stats[ct.ID()]++
	}

	if manifest.RequiresReindex {
		op.Status = "reconciling"
	}

	op.Status = "completed"
	now := nowRFC3339()
	op.UpdatedAt = now
	_ = now

	return req.IdentityMap, op, nil
}

func (c *Coordinator) validateManifestComponents(manifest *BackupManifest) error {
	known := make(map[string]bool)
	for _, ct := range c.Contributors {
		for _, id := range ct.ComponentIDs() {
			known[id] = true
		}
	}
	for _, component := range manifest.Components {
		if !known[component.ID] {
			return fmt.Errorf("%w: %s", ErrImportDatasetUnsupported, component.ID)
		}
	}
	return nil
}

func manifestContainsContributor(manifest *BackupManifest, contributor BackupContributor) bool {
	present := make(map[string]bool)
	for _, component := range manifest.Components {
		present[component.ID] = true
	}
	for _, id := range contributor.ComponentIDs() {
		if present[id] {
			return true
		}
	}
	return false
}

func (c *Coordinator) sortedContributorsForImport() []BackupContributor {
	contributorMap := make(map[string]BackupContributor)
	for _, ct := range c.Contributors {
		contributorMap[ct.ID()] = ct
	}

	visited := make(map[string]bool)
	result := make([]BackupContributor, 0, len(c.Contributors))

	var visit func(id string)
	visit = func(id string) {
		if visited[id] {
			return
		}
		visited[id] = true
		ct, ok := contributorMap[id]
		if !ok {
			return
		}
		for _, dep := range ct.Dependencies() {
			visit(dep)
		}
		result = append(result, ct)
	}

	for _, ct := range c.Contributors {
		visit(ct.ID())
	}
	return result
}

func (c *Coordinator) FailImport(opID string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if op, ok := c.importOps[opID]; ok {
		op.Status = "failed"
		op.Error = err.Error()
	}
}

func generateOpID() string {
	return "import-" + uuid.NewString()
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

type archiveBackupReader struct {
	r *ArchiveReader
}

func (a *archiveBackupReader) ReadComponent(id string) (io.ReadCloser, error) {
	path := fmt.Sprintf("datasets/%s.ndjson", id)
	return a.r.OpenComponent(path)
}

func (a *archiveBackupReader) ReadJSON(id string, v interface{}) error {
	rc, err := a.ReadComponent(id)
	if err != nil {
		return err
	}
	defer rc.Close()
	return readJSONFromReader(rc, v)
}

func (a *archiveBackupReader) ListComponents() []string {
	var result []string
	for name := range a.r.files {
		if len(name) > 9 && name[:9] == "datasets/" {
			result = append(result, name)
		}
	}
	return result
}

func readJSONFromReader(r io.Reader, v interface{}) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
