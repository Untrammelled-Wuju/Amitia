package packageformat

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type testActionConfig struct {
	SchemaVersion          int              `json:"schemaVersion"`
	ActionKey              string           `json:"actionKey"`
	DisplayName            string           `json:"displayName"`
	Version                int              `json:"version"`
	Fps                    int              `json:"fps"`
	PlaybackMode           string           `json:"playbackMode"`
	Interruptible          bool             `json:"interruptible"`
	Priority               int              `json:"priority"`
	CooldownMs             int              `json:"cooldownMs"`
	MinimumPlayMs          int              `json:"minimumPlayMs"`
	MaximumPlayMs          *int             `json:"maximumPlayMs"`
	MutexGroup             *string          `json:"mutexGroup"`
	SupportsDefaultIdle    bool             `json:"supportsDefaultIdle"`
	IsStableStateCandidate bool             `json:"isStableStateCandidate"`
	IsTransitionOnly       bool             `json:"isTransitionOnly"`
	Frames                 []testFrameEntry `json:"frames"`
	ReturnTo               testReturnTo     `json:"returnTo"`
	Anchor                 testAnchor       `json:"anchor"`
}

type testFrameEntry struct {
	Index       int    `json:"index"`
	FrameID     string `json:"frameId"`
	File        string `json:"file"`
	DurationMs  int    `json:"durationMs"`
	AssetID     string `json:"assetId"`
	ContentHash string `json:"contentHash"`
}

type testReturnTo struct {
	Type      string `json:"type"`
	ActionKey string `json:"actionKey"`
}

type testAnchor struct {
	X               float64 `json:"x"`
	Y               float64 `json:"y"`
	CoordinateSpace string  `json:"coordinateSpace"`
}

func pngFrameData() []byte {
	return []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func slashJoin(paths ...string) string {
	return filepath.ToSlash(filepath.Join(paths...))
}

func writeTestFile(t *testing.T, dir, relPath string, data []byte) {
	t.Helper()
	abs := filepath.Join(dir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("MkdirAll for %s: %v", abs, err)
	}
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", abs, err)
	}
}

func hasFinding(report *ValidationReport, code ErrorCode) bool {
	for _, f := range report.Findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

func createValidTestPackage(t *testing.T) (string, *Manifest) {
	t.Helper()
	dir := t.TempDir()
	frameData := pngFrameData()
	frameHash := sha256Hex(frameData)

	cfg := testActionConfig{
		SchemaVersion:          2,
		ActionKey:              "idle",
		DisplayName:            "Idle",
		Version:                1,
		Interruptible:          true,
		Priority:               50,
		CooldownMs:             0,
		MinimumPlayMs:          0,
		MaximumPlayMs:          nil,
		MutexGroup:             nil,
		SupportsDefaultIdle:    true,
		IsStableStateCandidate: false,
		IsTransitionOnly:       false,
		Fps:                    10,
		PlaybackMode:           "loop",
		Frames: []testFrameEntry{
			{Index: 0, FrameID: "idle_0", File: "frames/0.png", DurationMs: 100, AssetID: "asset_idle_0", ContentHash: frameHash},
			{Index: 1, FrameID: "idle_1", File: "frames/1.png", DurationMs: 100, AssetID: "asset_idle_1", ContentHash: frameHash},
		},
		ReturnTo: testReturnTo{Type: "none"},
		Anchor:   testAnchor{X: 0.5, Y: 1.0, CoordinateSpace: "normalized_canvas"},
	}

	cfgData, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal action config: %v", err)
	}
	writeTestFile(t, dir, "actions/idle/action.json", cfgData)
	writeTestFile(t, dir, "actions/idle/frames/0.png", frameData)
	writeTestFile(t, dir, "actions/idle/frames/1.png", frameData)

	manifest := NewManifest()
	manifest.PetID = "pet-001"
	manifest.ReleaseID = "release-001"
	manifest.Version = "1.0.0"
	manifest.Name = "Test Pet"
	manifest.Canvas.Width = 512
	manifest.Canvas.Height = 512
	manifest.DefaultAction = "idle"
	manifest.Actions = []ManifestActionEntry{
		{
			Key:                 "idle",
			Name:                "Idle",
			Config:              "actions/idle/action.json",
			PlaybackMode:        "loop",
			FPS:                 10,
			FrameCount:          2,
			SupportsDefaultIdle: true,
		},
	}

	aw := &ArchiveWriter{}
	m, err := aw.BuildManifestForArchive(dir, manifest)
	if err != nil {
		t.Fatalf("BuildManifestForArchive: %v", err)
	}
	return dir, m
}

func createTestPackageWithConfig(t *testing.T, cfg *testActionConfig) (string, *Manifest) {
	t.Helper()
	dir := t.TempDir()
	frameData := pngFrameData()

	for _, frame := range cfg.Frames {
		frameRelPath := slashJoin("actions", cfg.ActionKey, frame.File)
		writeTestFile(t, dir, frameRelPath, frameData)
	}

	cfgData, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal action config: %v", err)
	}
	configRelPath := slashJoin("actions", cfg.ActionKey, "action.json")
	writeTestFile(t, dir, configRelPath, cfgData)

	manifest := NewManifest()
	manifest.PetID = "pet-001"
	manifest.ReleaseID = "release-001"
	manifest.Version = "1.0.0"
	manifest.Name = "Test Pet"
	manifest.Canvas.Width = 512
	manifest.Canvas.Height = 512
	manifest.DefaultAction = cfg.ActionKey
	manifest.Actions = []ManifestActionEntry{
		{
			Key:                 cfg.ActionKey,
			Name:                cfg.DisplayName,
			Config:              configRelPath,
			PlaybackMode:        cfg.PlaybackMode,
			FPS:                 cfg.Fps,
			FrameCount:          len(cfg.Frames),
			SupportsDefaultIdle: true,
		},
	}

	aw := &ArchiveWriter{}
	m, err := aw.BuildManifestForArchive(dir, manifest)
	if err != nil {
		t.Fatalf("BuildManifestForArchive: %v", err)
	}
	return dir, m
}

func TestNormalizePlaybackMode(t *testing.T) {
	cases := []struct {
		input  string
		expect string
	}{
		{"ping-pong", "ping_pong"},
		{"loop", "loop"},
		{"once", "once"},
		{"hold", "hold"},
		{"ping_pong", "ping_pong"},
		{"", ""},
	}
	for _, c := range cases {
		got := NormalizePlaybackMode(c.input)
		if got != c.expect {
			t.Errorf("NormalizePlaybackMode(%q) = %q, want %q", c.input, got, c.expect)
		}
	}
}

func TestIsValidPlaybackMode(t *testing.T) {
	validModes := []string{"loop", "once", "hold", "ping_pong"}
	for _, mode := range validModes {
		if !IsValidPlaybackMode(mode) {
			t.Errorf("IsValidPlaybackMode(%q) = false, want true", mode)
		}
	}
	invalidModes := []string{"ping-pong", "pingpong", "unknown", ""}
	for _, mode := range invalidModes {
		if IsValidPlaybackMode(mode) {
			t.Errorf("IsValidPlaybackMode(%q) = true, want false", mode)
		}
	}
}

func TestComputeTreeHash_SameInput(t *testing.T) {
	entries := []FileEntry{
		{Path: "actions/idle/action.json", SHA256: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2", Bytes: 100},
		{Path: "actions/idle/frames/0.png", SHA256: "1112131415161718192021222324252627282930313233343536373839404142", Bytes: 8},
	}
	hash1 := ComputeTreeHash(entries)
	hash2 := ComputeTreeHash(entries)
	if hash1 != hash2 {
		t.Errorf("same input produced different hashes: %s vs %s", hash1, hash2)
	}
	if hash1 == "" {
		t.Error("hash is empty for non-empty input")
	}
}

func TestComputeTreeHash_DifferentOrder(t *testing.T) {
	entries1 := []FileEntry{
		{Path: "actions/idle/action.json", SHA256: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2", Bytes: 100},
		{Path: "actions/idle/frames/0.png", SHA256: "1112131415161718192021222324252627282930313233343536373839404142", Bytes: 8},
		{Path: "preview.png", SHA256: "9999999999999999999999999999999999999999999999999999999999999999", Bytes: 8},
	}
	entries2 := []FileEntry{
		{Path: "preview.png", SHA256: "9999999999999999999999999999999999999999999999999999999999999999", Bytes: 8},
		{Path: "actions/idle/frames/0.png", SHA256: "1112131415161718192021222324252627282930313233343536373839404142", Bytes: 8},
		{Path: "actions/idle/action.json", SHA256: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2", Bytes: 100},
	}
	hash1 := ComputeTreeHash(entries1)
	hash2 := ComputeTreeHash(entries2)
	if hash1 != hash2 {
		t.Errorf("different order produced different hashes: %s vs %s", hash1, hash2)
	}
}

func TestComputeTreeHash_EmptyList(t *testing.T) {
	hash1 := ComputeTreeHash(nil)
	hash2 := ComputeTreeHash([]FileEntry{})
	if hash1 != hash2 {
		t.Errorf("nil and empty slice produced different hashes: %s vs %s", hash1, hash2)
	}
	if hash1 == "" {
		t.Error("empty list hash is empty")
	}
	hash3 := ComputeTreeHash(nil)
	if hash1 != hash3 {
		t.Errorf("empty list hash not deterministic: %s vs %s", hash1, hash3)
	}
}

func TestValidateDirectory_Valid(t *testing.T) {
	dir, manifest := createValidTestPackage(t)
	v := NewValidator()
	report := v.ValidateDirectory(dir, manifest)
	if report.Verdict != "valid" && report.Verdict != "valid_with_warnings" {
		t.Errorf("Verdict = %q, want valid or valid_with_warnings", report.Verdict)
		for _, f := range report.Findings {
			t.Logf("Finding: %s %s %s", f.Severity, f.Code, f.Message)
		}
	}
}

func TestValidateDirectory_ActionConfigMissing(t *testing.T) {
	dir, manifest := createValidTestPackage(t)
	actionConfigPath := filepath.Join(dir, "actions", "idle", "action.json")
	if err := os.Remove(actionConfigPath); err != nil {
		t.Fatalf("remove action.json: %v", err)
	}
	v := NewValidator()
	report := v.ValidateDirectory(dir, manifest)
	if !hasFinding(report, ErrCodeActionConfigMissing) {
		t.Errorf("expected finding %s, not found", ErrCodeActionConfigMissing)
		for _, f := range report.Findings {
			t.Logf("Finding: %s %s %s", f.Severity, f.Code, f.Message)
		}
	}
}

func TestValidateDirectory_InvalidPlaybackMode(t *testing.T) {
	frameData := pngFrameData()
	frameHash := sha256Hex(frameData)
	cfg := &testActionConfig{
		SchemaVersion:          2,
		ActionKey:              "idle",
		DisplayName:            "Idle",
		Version:                1,
		Interruptible:          true,
		Priority:               50,
		CooldownMs:             0,
		MinimumPlayMs:          0,
		MaximumPlayMs:          nil,
		MutexGroup:             nil,
		SupportsDefaultIdle:    true,
		IsStableStateCandidate: false,
		IsTransitionOnly:       false,
		Fps:                    10,
		PlaybackMode:           "unknown",
		Frames: []testFrameEntry{
			{Index: 0, FrameID: "idle_0", File: "frames/0.png", DurationMs: 100, AssetID: "asset_idle_0", ContentHash: frameHash},
			{Index: 1, FrameID: "idle_1", File: "frames/1.png", DurationMs: 100, AssetID: "asset_idle_1", ContentHash: frameHash},
		},
		ReturnTo: testReturnTo{Type: "none"},
		Anchor:   testAnchor{X: 0.5, Y: 1.0, CoordinateSpace: "normalized_canvas"},
	}
	dir, manifest := createTestPackageWithConfig(t, cfg)
	v := NewValidator()
	report := v.ValidateDirectory(dir, manifest)
	if !hasFinding(report, ErrCodeActionConfigInvalid) {
		t.Errorf("expected finding %s, not found", ErrCodeActionConfigInvalid)
		for _, f := range report.Findings {
			t.Logf("Finding: %s %s %s", f.Severity, f.Code, f.Message)
		}
	}
}

func TestValidateDirectory_EmptyFrames(t *testing.T) {
	cfg := &testActionConfig{
		SchemaVersion:          2,
		ActionKey:              "idle",
		DisplayName:            "Idle",
		Version:                1,
		Interruptible:          true,
		Priority:               50,
		CooldownMs:             0,
		MinimumPlayMs:          0,
		MaximumPlayMs:          nil,
		MutexGroup:             nil,
		SupportsDefaultIdle:    true,
		IsStableStateCandidate: false,
		IsTransitionOnly:       false,
		Fps:                    10,
		PlaybackMode:           "loop",
		Frames:                 []testFrameEntry{},
		ReturnTo:               testReturnTo{Type: "none"},
		Anchor:                 testAnchor{X: 0.5, Y: 1.0, CoordinateSpace: "normalized_canvas"},
	}
	dir, manifest := createTestPackageWithConfig(t, cfg)
	v := NewValidator()
	report := v.ValidateDirectory(dir, manifest)
	if !hasFinding(report, ErrCodeFrameMissing) {
		t.Errorf("expected finding %s, not found", ErrCodeFrameMissing)
		for _, f := range report.Findings {
			t.Logf("Finding: %s %s %s", f.Severity, f.Code, f.Message)
		}
	}
}

func TestValidateDirectory_ReturnToActionInvalid(t *testing.T) {
	frameData := pngFrameData()
	frameHash := sha256Hex(frameData)
	cfg := &testActionConfig{
		SchemaVersion:          2,
		ActionKey:              "idle",
		DisplayName:            "Idle",
		Version:                1,
		Interruptible:          true,
		Priority:               50,
		CooldownMs:             0,
		MinimumPlayMs:          0,
		MaximumPlayMs:          nil,
		MutexGroup:             nil,
		SupportsDefaultIdle:    true,
		IsStableStateCandidate: false,
		IsTransitionOnly:       false,
		Fps:                    10,
		PlaybackMode:           "loop",
		Frames: []testFrameEntry{
			{Index: 0, FrameID: "idle_0", File: "frames/0.png", DurationMs: 100, AssetID: "asset_idle_0", ContentHash: frameHash},
			{Index: 1, FrameID: "idle_1", File: "frames/1.png", DurationMs: 100, AssetID: "asset_idle_1", ContentHash: frameHash},
		},
		ReturnTo: testReturnTo{Type: "action", ActionKey: "nonexistent"},
		Anchor:   testAnchor{X: 0.5, Y: 1.0, CoordinateSpace: "normalized_canvas"},
	}
	dir, manifest := createTestPackageWithConfig(t, cfg)
	v := NewValidator()
	report := v.ValidateDirectory(dir, manifest)
	if !hasFinding(report, ErrCodeActionReferenceInvalid) {
		t.Errorf("expected finding %s, not found", ErrCodeActionReferenceInvalid)
		for _, f := range report.Findings {
			t.Logf("Finding: %s %s %s", f.Severity, f.Code, f.Message)
		}
	}
}

func TestDirectoryPackageFS_OpenStatList(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "file1.txt", []byte("hello"))
	writeTestFile(t, dir, "subdir/file2.txt", []byte("world"))

	fs := NewDirectoryPackageFS(dir)

	rc, err := fs.Open("file1.txt")
	if err != nil {
		t.Fatalf("Open file1.txt: %v", err)
	}
	data, readErr := io.ReadAll(rc)
	rc.Close()
	if readErr != nil {
		t.Fatalf("ReadAll: %v", readErr)
	}
	if string(data) != "hello" {
		t.Errorf("Open content = %q, want %q", string(data), "hello")
	}

	fi, err := fs.Stat("file1.txt")
	if err != nil {
		t.Fatalf("Stat file1.txt: %v", err)
	}
	if fi.Size() != 5 {
		t.Errorf("Stat size = %d, want 5", fi.Size())
	}

	paths, err := fs.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(paths) != 2 {
		t.Errorf("List count = %d, want 2", len(paths))
	}

	pathSet := make(map[string]bool, len(paths))
	for _, p := range paths {
		pathSet[p] = true
	}
	if !pathSet["file1.txt"] {
		t.Errorf("List missing file1.txt, got %v", paths)
	}
	if !pathSet["subdir/file2.txt"] {
		t.Errorf("List missing subdir/file2.txt, got %v", paths)
	}
}

func TestDirectoryPackageFS_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "safe.txt", []byte("hello"))

	fs := NewDirectoryPackageFS(dir)

	_, err := fs.Open("safe.txt")
	if err != nil {
		t.Errorf("Open safe.txt failed: %v", err)
	}

	_, err = fs.Open("../escape.txt")
	if err == nil {
		t.Error("expected error for path traversal Open ../escape.txt, got nil")
	}

	_, err = fs.Stat("../escape.txt")
	if err == nil {
		t.Error("expected error for path traversal Stat ../escape.txt, got nil")
	}

	_, err = fs.Open("../../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal Open ../../etc/passwd, got nil")
	}

	_, err = fs.Open("/etc/passwd")
	if err == nil {
		t.Error("expected error for absolute path Open /etc/passwd, got nil")
	}
}

func TestIsForbiddenExecutable(t *testing.T) {
	for _, relPath := range []string{
		"payload.exe",
		"actions/idle/helper.DLL",
		"scripts/install.ps1",
		"scripts/runtime.sh",
		"payload.wasm",
		"mac/Companion.app/Contents/MacOS/Companion",
	} {
		if !isForbiddenExecutable(relPath) {
			t.Errorf("isForbiddenExecutable(%q) = false, want true", relPath)
		}
	}
	for _, relPath := range []string{
		"manifest.json",
		"preview.png",
		"actions/idle/action.json",
		"actions/idle/frames/frame-0001.webp",
		"NOTICE.txt",
	} {
		if isForbiddenExecutable(relPath) {
			t.Errorf("isForbiddenExecutable(%q) = true, want false", relPath)
		}
	}
}

func TestV2ReaderRejectsMissingNestedRequiredField(t *testing.T) {
	_, manifest := createValidTestPackage(t)
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	actions := raw["actions"].([]any)
	first := actions[0].(map[string]any)
	delete(first, "supportsDefaultIdle")
	data, err = json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal mutated manifest: %v", err)
	}
	if _, err := (&V2Reader{}).ReadManifest(data); err == nil {
		t.Fatal("expected V2Reader to reject an action missing supportsDefaultIdle")
	}
}

func TestValidateDirectoryRejectsMissingRequiredFrameField(t *testing.T) {
	dir, manifest := createValidTestPackage(t)
	configPath := filepath.Join(dir, "actions", "idle", "action.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read action config: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal action config: %v", err)
	}
	frames := raw["frames"].([]any)
	delete(frames[0].(map[string]any), "index")
	data, err = json.MarshalIndent(raw, "", "  ")
	if err != nil {
		t.Fatalf("marshal mutated action config: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("write action config: %v", err)
	}
	manifest, err = (&ArchiveWriter{}).BuildManifestForArchive(dir, manifest)
	if err != nil {
		t.Fatalf("rebuild manifest: %v", err)
	}
	report := NewValidator().ValidateDirectory(dir, manifest)
	if !hasFinding(report, ErrCodeActionConfigInvalid) {
		t.Fatalf("expected %s for missing frame.index, findings=%+v", ErrCodeActionConfigInvalid, report.Findings)
	}
}

func TestValidateDirectoryRejectsFrameHashNotMatchingIntegrity(t *testing.T) {
	dir, manifest := createValidTestPackage(t)
	configPath := filepath.Join(dir, "actions", "idle", "action.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read action config: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal action config: %v", err)
	}
	frames := raw["frames"].([]any)
	frames[0].(map[string]any)["contentHash"] = string(make([]byte, 0)) + "0000000000000000000000000000000000000000000000000000000000000000"
	data, err = json.MarshalIndent(raw, "", "  ")
	if err != nil {
		t.Fatalf("marshal mutated action config: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("write action config: %v", err)
	}
	manifest, err = (&ArchiveWriter{}).BuildManifestForArchive(dir, manifest)
	if err != nil {
		t.Fatalf("rebuild manifest: %v", err)
	}
	report := NewValidator().ValidateDirectory(dir, manifest)
	if !hasFinding(report, ErrCodePackageResourceHashMismatch) {
		t.Fatalf("expected %s, findings=%+v", ErrCodePackageResourceHashMismatch, report.Findings)
	}
}

func TestValidateDirectoryLegacyV1Compatibility(t *testing.T) {
	dir := t.TempDir()
	legacyManifest := map[string]any{
		"schemaVersion":     1,
		"packageId":         "legacy-release-001",
		"name":              "Legacy Pet",
		"characterId":       "legacy-pet-001",
		"generationTaskId":  "legacy-task-001",
		"processingVersion": 1,
		"createdAt":         "2026-08-29T00:00:00Z",
		"canvas":            map[string]any{"width": 512, "height": 512},
		"defaultAction":     "idle",
		"preview":           "",
		"actions":           []map[string]any{{"key": "idle", "name": "Idle", "config": "actions/idle/action.json", "loopType": "loop"}},
		"capabilities":      map[string]any{"hasTransparentBackground": true, "supportsFrameSequence": true},
	}
	manifestData, err := json.MarshalIndent(legacyManifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal legacy manifest: %v", err)
	}
	writeTestFile(t, dir, "manifest.json", manifestData)
	frameData := pngFrameData()
	writeTestFile(t, dir, "actions/idle/frames/0.png", frameData)
	legacyAction := map[string]any{
		"key":           "idle",
		"name":          "Idle",
		"loopType":      "loop",
		"fps":           10,
		"frames":        []map[string]any{{"index": 0, "file": "frames/0.png", "durationMs": 100}},
		"anchor":        map[string]any{"type": "feet_center", "x": 0.5, "y": 0.92},
		"interruptible": true,
	}
	actionData, err := json.MarshalIndent(legacyAction, "", "  ")
	if err != nil {
		t.Fatalf("marshal legacy action: %v", err)
	}
	writeTestFile(t, dir, "actions/idle/action.json", actionData)

	manifest, err := NewSchemaRegistry().ReadManifest(manifestData)
	if err != nil {
		t.Fatalf("read legacy manifest: %v", err)
	}
	report := NewValidator().ValidateDirectory(dir, manifest)
	if report.Verdict != "valid" && report.Verdict != "valid_with_warnings" {
		t.Fatalf("legacy V1 package should remain importable, verdict=%s findings=%+v", report.Verdict, report.Findings)
	}
}

func TestValidateDirectoryLegacyV1MissingFrameRejected(t *testing.T) {
	dir := t.TempDir()
	legacyManifest := map[string]any{
		"schemaVersion":     1,
		"packageId":         "legacy-release-002",
		"name":              "Legacy Pet",
		"characterId":       "legacy-pet-002",
		"processingVersion": 1,
		"canvas":            map[string]any{"width": 512, "height": 512},
		"defaultAction":     "idle",
		"actions":           []map[string]any{{"key": "idle", "name": "Idle", "config": "actions/idle/action.json", "loopType": "loop"}},
		"capabilities":      map[string]any{},
	}
	manifestData, _ := json.Marshal(legacyManifest)
	writeTestFile(t, dir, "manifest.json", manifestData)
	legacyAction := map[string]any{
		"key":      "idle",
		"fps":      10,
		"loopType": "loop",
		"frames":   []string{"frames/missing.png"},
	}
	actionData, _ := json.Marshal(legacyAction)
	writeTestFile(t, dir, "actions/idle/action.json", actionData)
	manifest, err := NewSchemaRegistry().ReadManifest(manifestData)
	if err != nil {
		t.Fatalf("read legacy manifest: %v", err)
	}
	report := NewValidator().ValidateDirectory(dir, manifest)
	if !hasFinding(report, ErrCodeFrameMissing) {
		t.Fatalf("expected missing legacy frame to be rejected, findings=%+v", report.Findings)
	}
}

func TestV2ReaderRejectsNullForNonNullableNestedField(t *testing.T) {
	_, manifest := createValidTestPackage(t)
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	actions := raw["actions"].([]any)
	actions[0].(map[string]any)["supportsDefaultIdle"] = nil
	data, err = json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal mutated manifest: %v", err)
	}
	if _, err := (&V2Reader{}).ReadManifest(data); err == nil {
		t.Fatal("expected V2Reader to reject supportsDefaultIdle:null")
	}
}

func TestValidateDirectoryRejectsNullForNonNullableActionField(t *testing.T) {
	dir, manifest := createValidTestPackage(t)
	configPath := filepath.Join(dir, "actions", "idle", "action.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read action config: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal action config: %v", err)
	}
	raw["interruptible"] = nil
	frames := raw["frames"].([]any)
	frames[0].(map[string]any)["index"] = nil
	data, err = json.MarshalIndent(raw, "", "  ")
	if err != nil {
		t.Fatalf("marshal mutated action config: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("write action config: %v", err)
	}
	manifest, err = (&ArchiveWriter{}).BuildManifestForArchive(dir, manifest)
	if err != nil {
		t.Fatalf("rebuild manifest: %v", err)
	}
	report := NewValidator().ValidateDirectory(dir, manifest)
	if !hasFinding(report, ErrCodeActionConfigInvalid) {
		t.Fatalf("expected null non-nullable V2 fields to be rejected, findings=%+v", report.Findings)
	}
}
