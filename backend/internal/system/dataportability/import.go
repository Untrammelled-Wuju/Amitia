package dataportability

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
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

	req := ImportPreviewRequest{
		StagingPath: staging,
		Manifest:    manifest,
	}

	br := &archiveBackupReader{r: reader}
	var previews []ImportComponentPreview
	for _, ct := range c.Contributors {
		p, err := ct.PreviewImport(ctx, req, br)
		if err != nil {
			continue
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
	staging, err := c.Staging.CreateStaging()
	if err != nil {
		return nil, err
	}
	defer c.Staging.CleanupStaging(staging)

	reader, err := NewArchiveReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	manifest, err := reader.ReadManifest()
	if err != nil {
		return nil, ErrImportPackageInvalid
	}

	req.Manifest = manifest
	req.StagingPath = staging
	req.OperationID = generateOpID()

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
		if err := ct.Import(ctx, req, br); err != nil {
			op.Status = "failed"
			op.Error = err.Error()
			return req.IdentityMap, ErrImportTransactionFailed
		}
		if count, ok := op.Stats[ct.ID()]; ok {
			_ = count
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

	return req.IdentityMap, nil
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
	return fmt.Sprintf("import-%d", time.Now().Unix())
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func currentUnixTime() int64 {
	return time.Now().Unix()
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
