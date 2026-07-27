package amitiax

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type InstallPhase string

const (
	PhaseValidate        InstallPhase = "validate"
	PhaseExtract         InstallPhase = "extract"
	PhaseVerify          InstallPhase = "verify"
	PhaseVerifySignature InstallPhase = "verify_signature"
	PhaseRegister        InstallPhase = "register"
	PhaseFinalize        InstallPhase = "finalize"
	PhaseCleanup         InstallPhase = "cleanup"
)

type InstallStatus string

const (
	InstallPending    InstallStatus = "pending"
	InstallInProgress InstallStatus = "in_progress"
	InstallSucceeded  InstallStatus = "succeeded"
	InstallFailed     InstallStatus = "failed"
	InstallRolledback InstallStatus = "rolled_back"
)

type InstallRequest struct {
	ArchivePath   string
	TargetDir     string
	ExtensionID   domain.ExtensionID
	ExpectedHash  string
	EnableAfter   bool
	DeveloperMode bool
	PublicKey     ed25519.PublicKey
	RequireSigned bool
}

type InstallResult struct {
	InstallID      string
	ExtensionID    domain.ExtensionID
	Version        domain.SemanticVersion
	Status         InstallStatus
	Definition     domain.ExtensionDefinition
	InstalledFiles []string
	Errors         []InstallError
	StartedAt      time.Time
	FinishedAt     *time.Time
	RollbackLog    []string
}

type InstallError struct {
	Phase   InstallPhase
	Code    string
	Message string
}

type InstallStep struct {
	Phase      InstallPhase
	StepID     string
	Status     InstallStatus
	StartedAt  time.Time
	FinishedAt *time.Time
	Error      *InstallError
}

type Installer struct {
	mu            sync.Mutex
	installations map[string]*InstallResult
}

func NewInstaller() *Installer {
	return &Installer{
		installations: make(map[string]*InstallResult),
	}
}

func (i *Installer) Install(ctx context.Context, request InstallRequest) InstallResult {
	installID := fmt.Sprintf("install-%d", time.Now().UnixNano())
	result := InstallResult{
		InstallID: installID,
		Status:    InstallInProgress,
		StartedAt: time.Now().UTC(),
	}
	i.mu.Lock()
	i.installations[installID] = &result
	i.mu.Unlock()
	rollback := &rollbackTracker{result: &result}
	pkg, err := i.validate(ctx, request, &result)
	if err != nil {
		result.Status = InstallFailed
		now := time.Now().UTC()
		result.FinishedAt = &now
		return result
	}
	rollback.markExtract(request.TargetDir)
	if err := i.extract(ctx, request, pkg, &result); err != nil {
		i.rollback(rollback, &result)
		result.Status = InstallFailed
		now := time.Now().UTC()
		result.FinishedAt = &now
		return result
	}
	if err := i.verify(ctx, pkg, &result); err != nil {
		i.rollback(rollback, &result)
		result.Status = InstallFailed
		now := time.Now().UTC()
		result.FinishedAt = &now
		return result
	}
	if err := i.verifySignature(ctx, request, pkg, &result); err != nil {
		i.rollback(rollback, &result)
		result.Status = InstallFailed
		now := time.Now().UTC()
		result.FinishedAt = &now
		return result
	}
	def, err := i.register(ctx, pkg, &result)
	if err != nil {
		i.rollback(rollback, &result)
		result.Status = InstallFailed
		now := time.Now().UTC()
		result.FinishedAt = &now
		return result
	}
	result.Definition = def
	result.ExtensionID = def.ID
	result.Version = def.Version
	result.Status = InstallSucceeded
	now := time.Now().UTC()
	result.FinishedAt = &now
	return result
}

func (i *Installer) validate(_ context.Context, request InstallRequest, result *InstallResult) (*Package, error) {
	if request.ArchivePath == "" {
		result.Errors = append(result.Errors, InstallError{Phase: PhaseValidate, Code: "missing_archive", Message: "archive path required"})
		return nil, fmt.Errorf("validate: missing archive")
	}
	if _, err := os.Stat(request.ArchivePath); err != nil {
		result.Errors = append(result.Errors, InstallError{Phase: PhaseValidate, Code: "archive_not_found", Message: err.Error()})
		return nil, fmt.Errorf("validate: archive not found")
	}
	pkg, err := OpenArchive(request.ArchivePath)
	if err != nil {
		result.Errors = append(result.Errors, InstallError{Phase: PhaseValidate, Code: "open_archive", Message: err.Error()})
		return nil, fmt.Errorf("validate: %w", err)
	}
	report := pkg.Manifest.Validate()
	if report.HasErrors() {
		for _, e := range report.Errors {
			result.Errors = append(result.Errors, InstallError{
				Phase: PhaseValidate, Code: e.Code, Message: fmt.Sprintf("%s: %s", e.Path, e.Message),
			})
		}
		return nil, fmt.Errorf("validate: manifest validation failed")
	}
	if request.ExpectedHash != "" {
		if pkg.Tree.TreeHash != request.ExpectedHash {
			result.Errors = append(result.Errors, InstallError{
				Phase: PhaseVerify, Code: "hash_mismatch", Message: "expected hash does not match",
			})
			return nil, fmt.Errorf("validate: hash mismatch")
		}
	}
	if request.ExtensionID != "" && pkg.Manifest.Extension.ID != string(request.ExtensionID) {
		result.Errors = append(result.Errors, InstallError{
			Phase: PhaseValidate, Code: "id_mismatch",
			Message: fmt.Sprintf("expected %s, got %s", request.ExtensionID, pkg.Manifest.Extension.ID),
		})
		return nil, fmt.Errorf("validate: extension id mismatch")
	}
	return pkg, nil
}

func (i *Installer) extract(ctx context.Context, request InstallRequest, pkg *Package, result *InstallResult) error {
	if request.TargetDir == "" {
		result.Errors = append(result.Errors, InstallError{Phase: PhaseExtract, Code: "missing_target", Message: "target dir required"})
		return fmt.Errorf("extract: missing target")
	}
	if err := os.MkdirAll(request.TargetDir, 0o755); err != nil {
		result.Errors = append(result.Errors, InstallError{Phase: PhaseExtract, Code: "mkdir", Message: err.Error()})
		return err
	}
	if err := WritePackageToDir(pkg, request.ArchivePath, request.TargetDir); err != nil {
		result.Errors = append(result.Errors, InstallError{Phase: PhaseExtract, Code: "write", Message: err.Error()})
		return err
	}
	for _, f := range pkg.Files {
		if !f.IsDir {
			result.InstalledFiles = append(result.InstalledFiles, filepath.Join(request.TargetDir, filepath.FromSlash(f.Path)))
		}
	}
	return nil
}

func (i *Installer) verify(_ context.Context, pkg *Package, result *InstallResult) error {
	if err := VerifyIntegrity(pkg); err != nil {
		result.Errors = append(result.Errors, InstallError{Phase: PhaseVerify, Code: "integrity", Message: err.Error()})
		return err
	}
	return nil
}

func (i *Installer) verifySignature(_ context.Context, request InstallRequest, pkg *Package, result *InstallResult) error {
	if pkg.Signatures == nil {
		if request.RequireSigned {
			result.Errors = append(result.Errors, InstallError{Phase: PhaseVerifySignature, Code: "unsigned", Message: "package is not signed but signature required"})
			return fmt.Errorf("verify_signature: package not signed")
		}
		return nil
	}
	if len(request.PublicKey) == 0 {
		if request.RequireSigned {
			result.Errors = append(result.Errors, InstallError{Phase: PhaseVerifySignature, Code: "missing_public_key", Message: "signature present but no public key provided"})
			return fmt.Errorf("verify_signature: missing public key")
		}
		return nil
	}
	if err := VerifySignature(pkg, request.PublicKey); err != nil {
		result.Errors = append(result.Errors, InstallError{Phase: PhaseVerifySignature, Code: "signature_invalid", Message: err.Error()})
		return fmt.Errorf("verify_signature: %w", err)
	}
	return nil
}

func (i *Installer) register(_ context.Context, pkg *Package, result *InstallResult) (domain.ExtensionDefinition, error) {
	def, err := pkg.Manifest.ToExtensionDefinition()
	if err != nil {
		result.Errors = append(result.Errors, InstallError{Phase: PhaseRegister, Code: "build_definition", Message: err.Error()})
		return domain.ExtensionDefinition{}, err
	}
	if err := def.Validate(); err != nil {
		result.Errors = append(result.Errors, InstallError{Phase: PhaseRegister, Code: "definition_invalid", Message: err.Error()})
		return domain.ExtensionDefinition{}, err
	}
	return def, nil
}

type rollbackTracker struct {
	mu        sync.Mutex
	result    *InstallResult
	extracted []string
}

func (r *rollbackTracker) markExtract(dir string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.extracted = append(r.extracted, dir)
}

func (i *Installer) rollback(r *rollbackTracker, result *InstallResult) {
	r.mu.Lock()
	dirs := append([]string{}, r.extracted...)
	r.mu.Unlock()
	for _, dir := range dirs {
		if err := os.RemoveAll(dir); err != nil {
			result.RollbackLog = append(result.RollbackLog, fmt.Sprintf("failed to remove %s: %v", dir, err))
		} else {
			result.RollbackLog = append(result.RollbackLog, fmt.Sprintf("removed %s", dir))
		}
	}
	result.Status = InstallRolledback
}

func (i *Installer) GetInstall(_ context.Context, installID string) (InstallResult, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	result, ok := i.installations[installID]
	if !ok {
		return InstallResult{}, fmt.Errorf("amitiax: install %s not found", installID)
	}
	return *result, nil
}

func (i *Installer) ListInstalls() []InstallResult {
	i.mu.Lock()
	defer i.mu.Unlock()
	var out []InstallResult
	for _, r := range i.installations {
		out = append(out, *r)
	}
	return out
}

var _ = strings.Join
