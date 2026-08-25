package package_security

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func pubKeyFingerprint(pub ed25519.PublicKey) string {
	h := sha256.Sum256(pub)
	return "sha256:" + hex.EncodeToString(h[:])
}

func createTestZIP(files map[string][]byte) []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0o444)
		header.SetModTime(time.Unix(0, 0).UTC())
		entry, _ := w.CreateHeader(header)
		entry.Write(content)
	}
	w.Close()
	return buf.Bytes()
}

func createZIPWithSymlink(name string, target string) []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	header := &zip.FileHeader{Name: name, Method: zip.Store}
	header.SetMode(0o444 | os.ModeSymlink)
	entry, _ := w.CreateHeader(header)
	entry.Write([]byte(target))
	w.Close()
	return buf.Bytes()
}

func TestFileTypeDetector(t *testing.T) {
	d := NewFileTypeDetector()

	result := d.Detect([]byte("PK\u0003\u0004\u0000\u0000\u0000\u0000"), ".amitiax")
	if !result.IsArchive {
		t.Error("expected archive detection for ZIP magic")
	}

	result = d.Detect([]byte("MZ\u0090\u0000"), ".exe")
	if !result.IsExecutable {
		t.Error("expected executable detection for PE magic")
	}

	result = d.Detect([]byte(`{"name": "test"}`), ".json")
	if !result.IsText {
		t.Error("expected text detection for JSON")
	}

	result = d.Detect([]byte("not a zip"), ".amitiax")
	found := false
	for _, w := range result.Warnings {
		if w == ".amitiax extension without valid archive magic" {
			found = true
		}
	}
	if !found {
		t.Error("expected warning for .amitiax without archive magic")
	}
}

func TestArchiveInspectorRejectsExecutableMagicAndNestedArchive(t *testing.T) {
	inspector := NewArchiveInspector(DefaultArchivePolicy())
	for name, content := range map[string][]byte{
		"modules/main/picture.png": {'M', 'Z', 0, 0},
		"modules/main/payload.zip": {'P', 'K', 3, 4},
	} {
		raw := createTestZIP(map[string][]byte{name: content})
		result, err := inspector.Inspect(context.Background(), raw)
		if err != nil {
			t.Fatalf("Inspect(%s) returned error: %v", name, err)
		}
		if result.Passed {
			t.Fatalf("Inspect(%s) must reject the entry", name)
		}
	}
}

func TestSecureExtractorRevalidatesEntryContent(t *testing.T) {
	extractor := NewSecureExtractor(DefaultArchivePolicy())
	raw := createTestZIP(map[string][]byte{"modules/main/picture.png": {'M', 'Z', 0, 0}})
	if _, err := extractor.Extract(context.Background(), raw, t.TempDir()); err == nil {
		t.Fatal("Extract must reject executable magic")
	}
}

func TestSafePathResolverNormalize(t *testing.T) {
	r := NewSafePathResolver(512, 32)

	tests := []struct {
		input    string
		expectOk bool
	}{
		{"manifest.json", true},
		{"modules/main/index.js", true},
		{"../escape.sh", false},
		{"/absolute/path", false},
		{"C:\\windows\\malware.exe", false},
		{"normal/../escape", false},
		{"CON", false},
		{"PRN.txt", false},
		{"COM1.exe", false},
		{"trailing-space ", false},
	}

	for _, tt := range tests {
		_, err := r.NormalizeArchivePath(tt.input)
		if tt.expectOk && err != nil {
			t.Errorf("expected %q to pass, got: %v", tt.input, err)
		}
		if !tt.expectOk && err == nil {
			t.Errorf("expected %q to fail, but it passed", tt.input)
		}
	}
}

func TestSafePathResolverResolveWithinRoot(t *testing.T) {
	r := NewSafePathResolver(512, 32)

	normalized, err := r.NormalizeArchivePath("modules/main.js")
	if err != nil {
		t.Fatalf("normalize failed: %v", err)
	}

	resolved, err := r.ResolveWithinRoot("/tmp/staging/123", normalized)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}

	expected := "/tmp/staging/123/modules/main.js"
	if resolved != expected {
		t.Errorf("expected %q, got %q", expected, resolved)
	}
}

func TestSafePathResolverDetectCollision(t *testing.T) {
	r := NewSafePathResolver(512, 32)

	paths := []NormalizedPath{"Modules/main.js", "modules/main.js"}
	collisions := r.DetectCollision(paths, PlatformWindows)
	if len(collisions) != 1 {
		t.Errorf("expected 1 collision on windows, got %d", len(collisions))
	}
}

func TestArchiveInspectorInspect(t *testing.T) {
	policy := DefaultArchivePolicy()
	inspector := NewArchiveInspector(policy)

	files := map[string][]byte{
		"manifest.json":     []byte(`{"name":"test"}`),
		"modules/main.js":   []byte(`console.log("hello")`),
		"schemas/test.json": []byte(`{"type":"object"}`),
	}

	raw := createTestZIP(files)

	result, err := inspector.Inspect(context.Background(), raw)
	if err != nil {
		t.Fatalf("Inspect failed: %v", err)
	}

	if !result.Passed {
		t.Error("expected inspection to pass")
	}
	if result.EntryCount != 3 {
		t.Errorf("expected 3 entries, got %d", result.EntryCount)
	}
}

func TestArchiveInspectorPathTraversal(t *testing.T) {
	policy := DefaultArchivePolicy()
	inspector := NewArchiveInspector(policy)

	files := map[string][]byte{
		"../escape.sh": []byte("malicious"),
	}
	raw := createTestZIP(files)

	result, _ := inspector.Inspect(context.Background(), raw)
	if result.Passed {
		t.Error("expected path traversal to be rejected")
	}
}

func TestArchiveInspectorZipBomb(t *testing.T) {
	policy := DefaultArchivePolicy()
	policy.MaxCompressionRatio = 10
	inspector := NewArchiveInspector(policy)

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	payload := make([]byte, 100000)
	_, _ = rand.Read(payload)
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("file_%d.txt", i)
		f, _ := w.Create(name)
		f.Write(payload)
	}
	w.Close()

	result, _ := inspector.Inspect(context.Background(), buf.Bytes())
	if !result.Passed {
		t.Error("expected zip with reasonable compression to pass")
	}
}

func TestArchiveInspectorMaxSize(t *testing.T) {
	policy := RestrictedArchivePolicy()
	policy.MaxArchiveBytes = 10
	inspector := NewArchiveInspector(policy)

	raw := make([]byte, 20)

	_, err := inspector.Inspect(context.Background(), raw)
	if err != ErrSizeLimitExceeded {
		t.Errorf("expected ErrSizeLimitExceeded, got %v", err)
	}
}

func TestEntryValidatorForbiddenExt(t *testing.T) {
	policy := DefaultArchivePolicy()
	v := NewEntryValidator(policy)

	entry := ArchiveEntryInfo{NormalizedPath: "malware.exe", Kind: EntryKindFile}
	content := []byte("MZ\u0090\u0000")

	result := v.Validate(entry, content)
	if result.Passed {
		t.Error("expected .exe to be rejected")
	}
}

func TestArchiveInspectorAllowsOnlyDeclaredServiceEntrypointExecutable(t *testing.T) {
	manifest := []byte(`{"modules":[{"id":"runtime","runtime":{"type":"service","entryPoint":"bin/game"}}]}`)
	elf := []byte{0x7f, 'E', 'L', 'F', 2, 1, 1, 0}

	inspector := NewArchiveInspector(DefaultArchivePolicy())
	result, err := inspector.Inspect(context.Background(), createTestZIP(map[string][]byte{
		"manifest.json":            manifest,
		"modules/runtime/bin/game": elf,
	}))
	if err != nil {
		t.Fatalf("Inspect returned error: %v", err)
	}
	if !result.Passed {
		t.Fatalf("declared service entrypoint must be accepted, errors=%v", result.Errors)
	}

	result, err = inspector.Inspect(context.Background(), createTestZIP(map[string][]byte{
		"manifest.json":             manifest,
		"modules/runtime/bin/other": elf,
	}))
	if err != nil {
		t.Fatalf("Inspect returned error: %v", err)
	}
	if result.Passed {
		t.Fatal("undeclared executable must remain rejected")
	}
}

func TestRestrictedArchivePolicyRejectsDeclaredServiceEntrypointExecutable(t *testing.T) {
	manifest := []byte(`{"modules":[{"id":"runtime","runtime":{"type":"service","entryPoint":"bin/game"}}]}`)
	elf := []byte{0x7f, 'E', 'L', 'F', 2, 1, 1, 0}
	inspector := NewArchiveInspector(RestrictedArchivePolicy())
	result, err := inspector.Inspect(context.Background(), createTestZIP(map[string][]byte{
		"manifest.json":            manifest,
		"modules/runtime/bin/game": elf,
	}))
	if err != nil {
		t.Fatalf("Inspect returned error: %v", err)
	}
	if result.Passed {
		t.Fatal("restricted policy must reject executable content even when declared")
	}
}

func TestSecureExtractorRestoresExecuteBitOnlyForDeclaredServiceEntrypoint(t *testing.T) {
	manifest := []byte(`{"modules":[{"id":"runtime","runtime":{"type":"service","entryPoint":"bin/game"}}]}`)
	elf := []byte{0x7f, 'E', 'L', 'F', 2, 1, 1, 0}
	raw := createTestZIP(map[string][]byte{
		"manifest.json":            manifest,
		"modules/runtime/bin/game": elf,
		"modules/runtime/config":   []byte("config"),
	})
	root := t.TempDir()
	extractor := NewSecureExtractor(DefaultArchivePolicy())
	if _, err := extractor.Extract(context.Background(), raw, root); err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	execInfo, err := os.Stat(filepath.Join(root, "modules", "runtime", "bin", "game"))
	if err != nil {
		t.Fatalf("stat executable: %v", err)
	}
	if execInfo.Mode().Perm()&0o100 == 0 {
		t.Fatalf("declared executable is missing owner execute bit: %o", execInfo.Mode().Perm())
	}
	configInfo, err := os.Stat(filepath.Join(root, "modules", "runtime", "config"))
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if configInfo.Mode().Perm()&0o111 != 0 {
		t.Fatalf("ordinary package file unexpectedly became executable: %o", configInfo.Mode().Perm())
	}
}

func TestContentHasherHashArchive(t *testing.T) {
	h := NewContentHasher()
	raw := []byte("test data")
	hash1 := h.HashArchive(raw)
	hash2 := h.HashArchive(raw)

	if hash1 != hash2 {
		t.Error("hash should be deterministic")
	}

	if len(hash1) < 10 {
		t.Error("hash should be non-trivial length")
	}
}

func TestContentHasherHashEntry(t *testing.T) {
	h := NewContentHasher()
	hash := h.HashEntry([]byte("content"))
	if len(hash) < 10 {
		t.Error("entry hash should be non-trivial")
	}
}

func TestContentHasherHashContentTree(t *testing.T) {
	h := NewContentHasher()

	entries := []ArchiveEntryInfo{
		{Path: "b.txt", NormalizedPath: "b.txt"},
		{Path: "a.txt", NormalizedPath: "a.txt"},
	}

	contents := map[string][]byte{
		"a.txt": []byte("aaa"),
		"b.txt": []byte("bbb"),
	}

	reader := func(path string) ([]byte, error) {
		return contents[path], nil
	}

	treeHash1, entryHashes1, err := h.HashContentTree(entries, reader)
	if err != nil {
		t.Fatalf("HashContentTree failed: %v", err)
	}

	treeHash2, entryHashes2, err := h.HashContentTree(entries, reader)
	if err != nil {
		t.Fatalf("HashContentTree failed: %v", err)
	}

	if treeHash1 != treeHash2 {
		t.Error("content tree hash should be deterministic with sorted entries")
	}

	if entryHashes1["a.txt"] != entryHashes2["a.txt"] {
		t.Error("entry hashes should be deterministic")
	}
}

func TestContentHasherBuildChecksumsFile(t *testing.T) {
	h := NewContentHasher()

	entryHashes := map[string]string{
		"b.txt": "hashB",
		"a.txt": "hashA",
	}

	checksum := h.BuildChecksumsFile(entryHashes)
	if !bytes.Contains(checksum, []byte("hashA")) || !bytes.Contains(checksum, []byte("hashB")) {
		t.Error("checksums file should contain all hashes")
	}
}

func TestSignatureVerifierValid(t *testing.T) {
	v := NewSignatureVerifier()

	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	fingerprint := pubKeyFingerprint(pub)

	v.AddTrustedKey(fingerprint, pub)

	sig := PackageSignature{
		Algorithm:       "ed25519",
		KeyID:           fingerprint,
		PublisherID:     "test-publisher",
		ContentTreeHash: "sha256:abc123",
		ManifestHash:    "sha256:def456",
	}

	message := sig.PublisherID + ":" + sig.ContentTreeHash + ":" + sig.ManifestHash
	sig.Signature = ed25519.Sign(priv, []byte(message))

	result := v.Verify(context.Background(), SignatureVerificationInput{
		Signature:             sig,
		PublicKey:             pub,
		ActualContentTreeHash: sig.ContentTreeHash,
		ActualManifestHash:    sig.ManifestHash,
	})

	if result.Status != SignatureValid {
		t.Errorf("expected valid signature, got %q", result.Status)
	}
}

func TestSignatureVerifierContentMismatch(t *testing.T) {
	v := NewSignatureVerifier()

	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	fingerprint := pubKeyFingerprint(pub)

	sig := PackageSignature{
		Algorithm:       "ed25519",
		KeyID:           fingerprint,
		PublisherID:     "test-publisher",
		ContentTreeHash: "sha256:abc123",
	}

	message := sig.PublisherID + ":" + sig.ContentTreeHash + ":" + sig.ManifestHash
	sig.Signature = ed25519.Sign(priv, []byte(message))

	result := v.Verify(context.Background(), SignatureVerificationInput{
		Signature:             sig,
		PublicKey:             pub,
		ActualContentTreeHash: "sha256:different",
	})

	if result.Status != SignatureContentMismatch {
		t.Errorf("expected content mismatch, got %q", result.Status)
	}
}

func TestPublisherTrustServiceBasic(t *testing.T) {
	s := NewPublisherTrustService()

	s.RegisterKey(&PublisherKey{
		KeyID:       "key-1",
		PublisherID: "pub-1",
		PublicKey:   []byte("some-key-data"),
		Algorithm:   "ed25519",
	})

	result := s.Evaluate(context.Background(), "pub-1", "key-1", SignatureVerificationResult{Status: SignatureValid})
	if result.Level != TrustUnknown {
		t.Errorf("expected unknown trust level, got %q", result.Level)
	}

	err := s.Trust(context.Background(), "pub-1", TrustTrusted)
	if err != nil {
		t.Fatalf("Trust failed: %v", err)
	}

	result = s.Evaluate(context.Background(), "pub-1", "key-1", SignatureVerificationResult{Status: SignatureValid})
	if result.Level != TrustTrusted {
		t.Errorf("expected trusted level, got %q", result.Level)
	}
}

func TestPublisherTrustServiceRevoke(t *testing.T) {
	s := NewPublisherTrustService()

	s.RegisterKey(&PublisherKey{
		KeyID:       "key-2",
		PublisherID: "pub-2",
		PublicKey:   []byte("some-key-data"),
		Algorithm:   "ed25519",
	})

	err := s.RevokeTrust(context.Background(), "pub-2", "key-2")
	if err != nil {
		t.Fatalf("RevokeTrust failed: %v", err)
	}

	key, _ := s.GetKey(context.Background(), "key-2")
	if !key.IsRevoked() {
		t.Error("expected key to be revoked")
	}

	result := s.Evaluate(context.Background(), "pub-2", "key-2", SignatureVerificationResult{Status: SignatureValid})
	if result.Level != TrustRevoked {
		t.Errorf("expected revoked trust level, got %q", result.Level)
	}
	if !result.Blocked {
		t.Error("revoked publisher should be blocked")
	}
}

func TestSecureExtractor(t *testing.T) {
	policy := DefaultArchivePolicy()
	extractor := NewSecureExtractor(policy)

	files := map[string][]byte{
		"manifest.json": []byte(`{"name":"test"}`),
		"mod/main.js":   []byte(`console.log(1)`),
	}
	raw := createTestZIP(files)

	tmpDir := t.TempDir()
	entries, err := extractor.Extract(context.Background(), raw, tmpDir)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest.json failed: %v", err)
	}
	if string(content) != `{"name":"test"}` {
		t.Errorf("unexpected content: %q", string(content))
	}
}

func TestStagingManagerCreateAndSeal(t *testing.T) {
	mgr := NewStagingManager(t.TempDir())
	ctx := context.Background()

	area, err := mgr.Create(ctx, "test")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if area.Status != StagingCreated {
		t.Errorf("expected created status, got %q", area.Status)
	}

	os.WriteFile(filepath.Join(area.Path, "test.txt"), []byte("hello"), 0o600)
	mgr.MarkPopulated(ctx, area.ID)

	err = mgr.Seal(ctx, area.ID)
	if err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	if area.Status != StagingSealed {
		t.Errorf("expected sealed status, got %q", area.Status)
	}

	err = mgr.Verify(ctx, area.ID)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
}

func TestStagingManagerTamperDetection(t *testing.T) {
	mgr := NewStagingManager(t.TempDir())
	ctx := context.Background()

	area, _ := mgr.Create(ctx, "test")
	os.WriteFile(filepath.Join(area.Path, "test.txt"), []byte("hello"), 0o600)
	mgr.MarkPopulated(ctx, area.ID)
	mgr.Seal(ctx, area.ID)

	os.WriteFile(filepath.Join(area.Path, "test.txt"), []byte("tampered"), 0o600)

	err := mgr.Verify(ctx, area.ID)
	if err != ErrStagingTampered {
		t.Errorf("expected ErrStagingTampered, got %v", err)
	}
}

func TestStagingManagerCleanup(t *testing.T) {
	mgr := NewStagingManager(t.TempDir())
	ctx := context.Background()

	area, _ := mgr.Create(ctx, "test")
	os.WriteFile(filepath.Join(area.Path, "test.txt"), []byte("hello"), 0o600)
	mgr.MarkPopulated(ctx, area.ID)
	mgr.Seal(ctx, area.ID)

	err := mgr.Cleanup(ctx, area.ID)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	if _, err := os.Stat(area.Path); !os.IsNotExist(err) {
		t.Error("expected staging directory to be removed")
	}
}

func TestSnapshotManagerCreateAndRestore(t *testing.T) {
	baseDir := t.TempDir()
	mgr := NewSnapshotManager(filepath.Join(baseDir, "snapshots"))

	srcDir := filepath.Join(baseDir, "src")
	os.MkdirAll(srcDir, 0o700)
	os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("hello"), 0o600)

	snapshot, err := mgr.CreateSnapshot(context.Background(), srcDir, "pkg-1", "1.0.0", "owner-1")
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	if snapshot.PackageID != "pkg-1" {
		t.Errorf("expected package ID 'pkg-1', got %q", snapshot.PackageID)
	}

	got, err := mgr.GetSnapshot(context.Background(), snapshot.SnapshotID)
	if err != nil {
		t.Fatalf("GetSnapshot failed: %v", err)
	}

	if got.SnapshotID != snapshot.SnapshotID {
		t.Error("snapshot ID mismatch")
	}

	err = mgr.Retain(context.Background(), snapshot.SnapshotID)
	if err != nil {
		t.Fatalf("Retain failed: %v", err)
	}

	err = mgr.Delete(context.Background(), snapshot.SnapshotID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = mgr.GetSnapshot(context.Background(), snapshot.SnapshotID)
	if err != ErrSnapshotNotFound {
		t.Errorf("expected ErrSnapshotNotFound, got %v", err)
	}
}

func TestRollbackCoordinator(t *testing.T) {
	baseDir := t.TempDir()
	snapMgr := NewSnapshotManager(filepath.Join(baseDir, "snapshots"))
	coordinator := NewRollbackCoordinator(snapMgr)

	targetDir := filepath.Join(baseDir, "target")
	os.MkdirAll(targetDir, 0o700)
	os.WriteFile(filepath.Join(targetDir, "a.txt"), []byte("v1"), 0o600)

	snapshot, err := coordinator.Prepare(context.Background(), RollbackPrepareRequest{
		PackageID:  "pkg-1",
		Version:    "1.0.0",
		TargetPath: targetDir,
		OwnerID:    "owner-1",
	})
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}

	os.WriteFile(filepath.Join(targetDir, "a.txt"), []byte("v2"), 0o600)

	result := coordinator.Restore(context.Background(), snapshot.SnapshotID, targetDir)
	if !result.Success {
		t.Fatalf("Restore failed: %v", result.Errors)
	}

	content, _ := os.ReadFile(filepath.Join(targetDir, "a.txt"))
	if string(content) != "v1" {
		t.Errorf("expected 'v1', got %q", string(content))
	}
}

func TestRecoveryJournalBasic(t *testing.T) {
	j := NewRecoveryJournal()
	ctx := context.Background()

	entry := RecoveryJournalEntry{
		OperationID: "op-1",
		PackageID:   "pkg-1",
		Version:     "1.0.0",
		Step:        "staging_verified",
		State:       "in_progress",
	}

	j.Record(ctx, entry)

	entries := j.GetEntries(ctx, "op-1")
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}

	pending := j.ListPending(ctx)
	if len(pending) != 1 {
		t.Errorf("expected 1 pending, got %d", len(pending))
	}

	j.DeleteOperation(ctx, "op-1")
	entries = j.GetEntries(ctx, "op-1")
	if len(entries) != 0 {
		t.Error("expected 0 entries after delete")
	}
}

func TestCleanupManager(t *testing.T) {
	stagingBase := filepath.Join(t.TempDir(), "staging")
	snapshotBase := filepath.Join(t.TempDir(), "snapshots")

	stagingMgr := NewStagingManager(stagingBase)
	snapshotMgr := NewSnapshotManager(snapshotBase)
	cleanupMgr := NewCleanupManager(stagingMgr, snapshotMgr)

	area, _ := stagingMgr.Create(context.Background(), "test")
	area.ExpiresAt = time.Now().Add(-1 * time.Hour)

	result := cleanupMgr.CleanupAll(context.Background())
	if result.StagingCleaned != 1 {
		t.Errorf("expected 1 staging cleaned, got %d", result.StagingCleaned)
	}
}

func TestPackageSecurityReport(t *testing.T) {
	r := &PackageSecurityReport{Passed: true}

	r.AddPathIssue("test.exe", "forbidden extension", SeverityCritical, true)

	if !r.IsBlocked() {
		t.Error("expected report to be blocked")
	}
	if !r.HasHighRisk() {
		t.Error("expected report to have high risk")
	}
	if r.Passed {
		t.Error("expected report to not pass")
	}
}

func TestSecurityServiceInspect(t *testing.T) {
	policy := DefaultArchivePolicy()
	audit := NewMemoryAuditWriter()
	svc := NewPackageSecurityService(policy, audit)

	files := map[string][]byte{
		"manifest.json": []byte(`{"name":"test","version":"1.0.0"}`),
	}
	raw := createTestZIP(files)

	report, err := svc.Inspect(context.Background(), raw, PackageSource{
		SourceType: SourceLocalFile,
	})
	if err != nil {
		t.Fatalf("Inspect failed: %v", err)
	}

	if !report.Passed {
		t.Error("expected inspection to pass for valid zip")
	}

	events := audit.GetEvents(context.Background())
	if len(events) < 1 {
		t.Error("expected audit events to be recorded")
	}
}

func TestSecurityServiceInspectInvalidArchive(t *testing.T) {
	policy := DefaultArchivePolicy()
	audit := NewMemoryAuditWriter()
	svc := NewPackageSecurityService(policy, audit)

	report, err := svc.Inspect(context.Background(), []byte("not a zip"), PackageSource{
		SourceType: SourceLocalFile,
	})
	if err != nil {
		t.Fatalf("Inspect returned unexpected error: %v", err)
	}

	if report.Passed {
		t.Error("expected inspection to fail for invalid archive")
	}
}

func TestSecurityServiceExtractAndCommit(t *testing.T) {
	policy := DefaultArchivePolicy()
	audit := NewMemoryAuditWriter()
	svc := NewPackageSecurityService(policy, audit)

	files := map[string][]byte{
		"manifest.json": []byte(`{"name":"test"}`),
		"main.js":       []byte(`console.log(1)`),
	}
	raw := createTestZIP(files)

	report, _ := svc.Inspect(context.Background(), raw, PackageSource{SourceType: SourceLocalFile})
	if !report.Passed {
		t.Fatal("inspection should pass")
	}

	tmpDir := t.TempDir()

	staging, err := svc.ExtractToStaging(context.Background(), raw, report, "install-test")
	if err != nil {
		t.Fatalf("ExtractToStaging failed: %v", err)
	}

	targetPath := filepath.Join(tmpDir, "installed")
	_, err = svc.Commit(context.Background(), staging, targetPath, "test-pkg", "1.0.0")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(targetPath, "manifest.json"))
	if err != nil {
		t.Fatalf("read installed file failed: %v", err)
	}
	if string(content) != `{"name":"test"}` {
		t.Errorf("unexpected installed content: %q", string(content))
	}
}

func TestManifestBindingVerifier(t *testing.T) {
	v := NewManifestBindingVerifier()

	entries := []ArchiveEntryInfo{
		{Path: "a.txt", NormalizedPath: "a.txt"},
	}

	contents := map[string][]byte{"a.txt": []byte("hello")}
	reader := func(path string) ([]byte, error) { return contents[path], nil }

	integrity, err := v.BuildIntegrity(entries, reader)
	if err != nil {
		t.Fatalf("BuildIntegrity failed: %v", err)
	}

	manifestData, _ := json.Marshal(map[string]interface{}{
		"integrity": integrity,
	})

	result, err := v.Verify(context.Background(), manifestData, entries, reader)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	if !result.Passed {
		t.Error("expected integrity verification to pass")
	}
}

func TestAuditWriter(t *testing.T) {
	w := NewMemoryAuditWriter()
	ctx := context.Background()

	w.WriteAuditEvent(ctx, ResourceAuditEvent{
		EventType: AuditPackageInspect,
		PackageID: "pkg-1",
		Success:   true,
	})

	w.WriteAuditEvent(ctx, ResourceAuditEvent{
		EventType: AuditPackageReject,
		PackageID: "pkg-2",
		Success:   false,
	})

	events := w.GetEvents(ctx)
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}

	inspects := w.GetEventsByType(ctx, AuditPackageInspect)
	if len(inspects) != 1 {
		t.Errorf("expected 1 inspect event, got %d", len(inspects))
	}
}

func TestPackageSourceTypes(t *testing.T) {
	types := []PackageSourceType{
		SourceLocalFile, SourceUploadedFile, SourceWorkshopArtifact,
		SourceMigrationArtifact, SourceSystemBundle,
	}

	for _, st := range types {
		if !st.IsValid() {
			t.Errorf("expected %q to be valid", st)
		}
	}

	if PackageSourceType("invalid").IsValid() {
		t.Error("expected invalid type to be invalid")
	}
}

func TestErrorDefinitions(t *testing.T) {
	errors := []error{
		ErrInvalidArchive, ErrPathTraversal, ErrAbsolutePath,
		ErrSizeLimitExceeded, ErrSymlinkNotAllowed, ErrSignatureInvalid,
		ErrUnknownPublisher, ErrStagingTampered, ErrCommitFailed,
		ErrSnapshotNotFound, ErrRollbackFailed,
	}

	for _, e := range errors {
		if e.Error() == "" {
			t.Error("error should have a message")
		}
	}
}
