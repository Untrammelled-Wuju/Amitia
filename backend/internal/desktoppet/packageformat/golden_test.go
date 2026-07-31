package packageformat

import (
	"encoding/json"
	"testing"
)

func buildGoldenPackage(t *testing.T) (string, *Manifest) {
	t.Helper()
	dir := t.TempDir()
	frameData := pngFrameData()
	frameHash := sha256Hex(frameData)

	idleCfg := testActionConfig{
		SchemaVersion: 2,
		ActionKey:     "idle",
		DisplayName:   "Idle",
		Fps:           12,
		PlaybackMode:  "loop",
		Frames: []testFrameEntry{
			{Index: 0, FrameID: "idle_0", File: "frames/0.png", DurationMs: 83, AssetID: "asset_idle_0", ContentHash: frameHash},
			{Index: 1, FrameID: "idle_1", File: "frames/1.png", DurationMs: 83, AssetID: "asset_idle_1", ContentHash: frameHash},
			{Index: 2, FrameID: "idle_2", File: "frames/2.png", DurationMs: 83, AssetID: "asset_idle_2", ContentHash: frameHash},
		},
		ReturnTo: testReturnTo{Type: "none"},
		Anchor:   testAnchor{X: 0.5, Y: 1.0, CoordinateSpace: "normalized_canvas"},
	}

	waveCfg := testActionConfig{
		SchemaVersion: 2,
		ActionKey:     "wave",
		DisplayName:   "Wave",
		Fps:           15,
		PlaybackMode:  "ping_pong",
		Frames: []testFrameEntry{
			{Index: 0, FrameID: "wave_0", File: "frames/0.png", DurationMs: 67, AssetID: "asset_wave_0", ContentHash: frameHash},
			{Index: 1, FrameID: "wave_1", File: "frames/1.png", DurationMs: 67, AssetID: "asset_wave_1", ContentHash: frameHash},
			{Index: 2, FrameID: "wave_2", File: "frames/2.png", DurationMs: 67, AssetID: "asset_wave_2", ContentHash: frameHash},
			{Index: 3, FrameID: "wave_3", File: "frames/3.png", DurationMs: 67, AssetID: "asset_wave_3", ContentHash: frameHash},
		},
		ReturnTo: testReturnTo{Type: "action", ActionKey: "idle"},
		Anchor:   testAnchor{X: 0.5, Y: 1.0, CoordinateSpace: "normalized_canvas"},
	}

	idleCfgData, err := json.MarshalIndent(idleCfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal idle config: %v", err)
	}
	waveCfgData, err := json.MarshalIndent(waveCfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal wave config: %v", err)
	}

	writeTestFile(t, dir, "actions/idle/action.json", idleCfgData)
	writeTestFile(t, dir, "actions/idle/frames/0.png", frameData)
	writeTestFile(t, dir, "actions/idle/frames/1.png", frameData)
	writeTestFile(t, dir, "actions/idle/frames/2.png", frameData)

	writeTestFile(t, dir, "actions/wave/action.json", waveCfgData)
	writeTestFile(t, dir, "actions/wave/frames/0.png", frameData)
	writeTestFile(t, dir, "actions/wave/frames/1.png", frameData)
	writeTestFile(t, dir, "actions/wave/frames/2.png", frameData)
	writeTestFile(t, dir, "actions/wave/frames/3.png", frameData)

	writeTestFile(t, dir, "preview.png", frameData)

	manifest := NewManifest()
	manifest.PetID = "golden-pet-001"
	manifest.ReleaseID = "golden-release-001"
	manifest.Version = "1.0.0"
	manifest.Name = "Golden Test Pet"
	manifest.Description = "Golden package for validation testing"
	manifest.Preview = "preview.png"
	manifest.Canvas.Width = 512
	manifest.Canvas.Height = 512
	manifest.DefaultAction = "idle"
	manifest.Actions = []ManifestActionEntry{
		{
			Key:                 "idle",
			Name:                "Idle",
			Config:              "actions/idle/action.json",
			PlaybackMode:        "loop",
			FPS:                 12,
			FrameCount:          3,
			SupportsDefaultIdle: true,
		},
		{
			Key:                 "wave",
			Name:                "Wave",
			Config:              "actions/wave/action.json",
			PlaybackMode:        "ping_pong",
			FPS:                 15,
			FrameCount:          4,
			SupportsDefaultIdle: false,
		},
	}

	aw := &ArchiveWriter{}
	m, err := aw.BuildManifestForArchive(dir, manifest)
	if err != nil {
		t.Fatalf("BuildManifestForArchive: %v", err)
	}
	return dir, m
}

func TestGoldenPackage_Valid(t *testing.T) {
	dir, manifest := buildGoldenPackage(t)

	v := NewValidator()
	report := v.ValidateDirectory(dir, manifest)

	if report.Verdict != "valid" {
		t.Errorf("Verdict = %q, want valid", report.Verdict)
		for _, f := range report.Findings {
			t.Logf("Finding: %s %s %s (path=%s actionKey=%s)", f.Severity, f.Code, f.Message, f.Path, f.ActionKey)
		}
	}

	if report.ErrorCount != 0 {
		t.Errorf("ErrorCount = %d, want 0", report.ErrorCount)
	}

	if report.WarningCount != 0 {
		t.Errorf("WarningCount = %d, want 0", report.WarningCount)
	}

	if report.FileCount != 10 {
		t.Errorf("FileCount = %d, want 10", report.FileCount)
	}
}

func TestGoldenPackage_TreeHashDeterministic(t *testing.T) {
	_, manifest := buildGoldenPackage(t)

	var entries []FileEntry
	for _, f := range manifest.Integrity.Files {
		entries = append(entries, FileEntry{
			Path:   f.Path,
			SHA256: f.SHA256,
			Bytes:  f.Bytes,
		})
	}

	hash1 := ComputeTreeHash(entries)
	hash2 := ComputeTreeHash(entries)
	if hash1 != hash2 {
		t.Errorf("tree hash not deterministic: %s vs %s", hash1, hash2)
	}

	shuffled := make([]FileEntry, len(entries))
	copy(shuffled, entries)
	for i, j := 0, len(shuffled)-1; i < j; i, j = i+1, j-1 {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}
	hash3 := ComputeTreeHash(shuffled)
	if hash1 != hash3 {
		t.Errorf("tree hash differs with reversed order: %s vs %s", hash1, hash3)
	}

	manifestHash, hashErr := CanonicalManifestHash(manifest)
	if hashErr != nil {
		t.Fatalf("CanonicalManifestHash: %v", hashErr)
	}
	canonicalData, dataErr := CanonicalManifestData(manifest)
	if dataErr != nil {
		t.Fatalf("CanonicalManifestData: %v", dataErr)
	}
	manifestBytes := int64(len(canonicalData))
	contentRootHash := ComputeContentRootHash(entries, manifestHash, manifestBytes)
	if contentRootHash != manifest.Integrity.ContentRootHash {
		t.Errorf("content root hash %s != manifest ContentRootHash %s", contentRootHash, manifest.Integrity.ContentRootHash)
	}

	if len(hash1) != 64 {
		t.Errorf("tree hash length = %d, want 64", len(hash1))
	}
}

func TestGoldenPackage_PingPongAccepted(t *testing.T) {
	dir, manifest := buildGoldenPackage(t)

	v := NewValidator()
	report := v.ValidateDirectory(dir, manifest)

	for _, f := range report.Findings {
		if f.Code == ErrCodeActionConfigInvalid && f.ActionKey == "wave" {
			t.Errorf("wave action with ping_pong mode should be valid, but got finding: %s", f.Message)
		}
	}

	if report.Verdict != "valid" {
		t.Errorf("Verdict = %q, want valid (ping_pong should be accepted)", report.Verdict)
	}
}

func TestGoldenPackage_ReturnToActionValid(t *testing.T) {
	dir, manifest := buildGoldenPackage(t)

	v := NewValidator()
	report := v.ValidateDirectory(dir, manifest)

	for _, f := range report.Findings {
		if f.Code == ErrCodeActionReferenceInvalid && f.ActionKey == "wave" {
			t.Errorf("wave action returnTo action:idle should be valid, but got finding: %s", f.Message)
		}
	}

	if report.Verdict != "valid" {
		t.Errorf("Verdict = %q, want valid (returnTo action:idle should be accepted)", report.Verdict)
	}
}

func TestGoldenPackage_ReturnToActionInvalid(t *testing.T) {
	frameData := pngFrameData()
	frameHash := sha256Hex(frameData)

	idleCfg := testActionConfig{
		SchemaVersion: 2,
		ActionKey:     "idle",
		DisplayName:   "Idle",
		Fps:           12,
		PlaybackMode:  "loop",
		Frames: []testFrameEntry{
			{Index: 0, FrameID: "idle_0", File: "frames/0.png", DurationMs: 83, AssetID: "asset_idle_0", ContentHash: frameHash},
			{Index: 1, FrameID: "idle_1", File: "frames/1.png", DurationMs: 83, AssetID: "asset_idle_1", ContentHash: frameHash},
		},
		ReturnTo: testReturnTo{Type: "none"},
		Anchor:   testAnchor{X: 0.5, Y: 1.0, CoordinateSpace: "normalized_canvas"},
	}

	waveCfg := testActionConfig{
		SchemaVersion: 2,
		ActionKey:     "wave",
		DisplayName:   "Wave",
		Fps:           15,
		PlaybackMode:  "ping_pong",
		Frames: []testFrameEntry{
			{Index: 0, FrameID: "wave_0", File: "frames/0.png", DurationMs: 67, AssetID: "asset_wave_0", ContentHash: frameHash},
			{Index: 1, FrameID: "wave_1", File: "frames/1.png", DurationMs: 67, AssetID: "asset_wave_1", ContentHash: frameHash},
		},
		ReturnTo: testReturnTo{Type: "action", ActionKey: "nonexistent"},
		Anchor:   testAnchor{X: 0.5, Y: 1.0, CoordinateSpace: "normalized_canvas"},
	}

	dir := t.TempDir()

	idleCfgData, err := json.MarshalIndent(idleCfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal idle config: %v", err)
	}
	waveCfgData, err := json.MarshalIndent(waveCfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal wave config: %v", err)
	}

	writeTestFile(t, dir, "actions/idle/action.json", idleCfgData)
	writeTestFile(t, dir, "actions/idle/frames/0.png", frameData)
	writeTestFile(t, dir, "actions/idle/frames/1.png", frameData)

	writeTestFile(t, dir, "actions/wave/action.json", waveCfgData)
	writeTestFile(t, dir, "actions/wave/frames/0.png", frameData)
	writeTestFile(t, dir, "actions/wave/frames/1.png", frameData)

	manifest := NewManifest()
	manifest.PetID = "golden-pet-001"
	manifest.ReleaseID = "golden-release-001"
	manifest.Version = "1.0.0"
	manifest.Name = "Golden Test Pet"
	manifest.Canvas.Width = 512
	manifest.Canvas.Height = 512
	manifest.DefaultAction = "idle"
	manifest.Actions = []ManifestActionEntry{
		{
			Key:                 "idle",
			Name:                "Idle",
			Config:              "actions/idle/action.json",
			PlaybackMode:        "loop",
			FPS:                 12,
			FrameCount:          2,
			SupportsDefaultIdle: true,
		},
		{
			Key:                 "wave",
			Name:                "Wave",
			Config:              "actions/wave/action.json",
			PlaybackMode:        "ping_pong",
			FPS:                 15,
			FrameCount:          2,
			SupportsDefaultIdle: false,
		},
	}

	aw := &ArchiveWriter{}
	m, err := aw.BuildManifestForArchive(dir, manifest)
	if err != nil {
		t.Fatalf("BuildManifestForArchive: %v", err)
	}

	v := NewValidator()
	report := v.ValidateDirectory(dir, m)

	if !hasFinding(report, ErrCodeActionReferenceInvalid) {
		t.Errorf("expected finding %s for wave action returnTo nonexistent, not found", ErrCodeActionReferenceInvalid)
		for _, f := range report.Findings {
			t.Logf("Finding: %s %s %s (actionKey=%s)", f.Severity, f.Code, f.Message, f.ActionKey)
		}
	}

	foundForWave := false
	for _, f := range report.Findings {
		if f.Code == ErrCodeActionReferenceInvalid && f.ActionKey == "wave" {
			foundForWave = true
		}
	}
	if !foundForWave {
		t.Errorf("expected ACTION_REFERENCE_INVALID finding specifically for wave action")
	}
}

func TestGoldenPackage_AllFieldsValidated(t *testing.T) {
	dir, manifest := buildGoldenPackage(t)

	v := NewValidator()
	report := v.ValidateDirectory(dir, manifest)

	if report.Verdict != "valid" {
		t.Fatalf("Verdict = %q, want valid", report.Verdict)
	}

	if len(report.Findings) != 0 {
		t.Errorf("expected 0 findings for valid golden package, got %d:", len(report.Findings))
		for _, f := range report.Findings {
			t.Logf("  Finding: %s %s %s", f.Severity, f.Code, f.Message)
		}
	}

	if len(manifest.Actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(manifest.Actions))
	}

	idleAction := manifest.Actions[0]
	if idleAction.Key != "idle" {
		t.Errorf("action[0] Key = %q, want idle", idleAction.Key)
	}
	if idleAction.Config != "actions/idle/action.json" {
		t.Errorf("action[0] Config = %q, want actions/idle/action.json", idleAction.Config)
	}
	if idleAction.FPS != 12 {
		t.Errorf("action[0] FPS = %d, want 12", idleAction.FPS)
	}
	if idleAction.FrameCount != 3 {
		t.Errorf("action[0] FrameCount = %d, want 3", idleAction.FrameCount)
	}
	if !idleAction.SupportsDefaultIdle {
		t.Errorf("action[0] SupportsDefaultIdle = false, want true")
	}

	waveAction := manifest.Actions[1]
	if waveAction.Key != "wave" {
		t.Errorf("action[1] Key = %q, want wave", waveAction.Key)
	}
	if waveAction.Config != "actions/wave/action.json" {
		t.Errorf("action[1] Config = %q, want actions/wave/action.json", waveAction.Config)
	}
	if waveAction.FPS != 15 {
		t.Errorf("action[1] FPS = %d, want 15", waveAction.FPS)
	}
	if waveAction.FrameCount != 4 {
		t.Errorf("action[1] FrameCount = %d, want 4", waveAction.FrameCount)
	}

	if manifest.DefaultAction != "idle" {
		t.Errorf("DefaultAction = %q, want idle", manifest.DefaultAction)
	}

	if manifest.Preview != "preview.png" {
		t.Errorf("Preview = %q, want preview.png", manifest.Preview)
	}

	if manifest.Integrity.FileCount != len(manifest.Integrity.Files) {
		t.Errorf("Integrity.FileCount = %d, want %d", manifest.Integrity.FileCount, len(manifest.Integrity.Files))
	}

	if manifest.Integrity.ContentRootHash == "" {
		t.Error("Integrity.ContentRootHash is empty")
	}

	if manifest.Integrity.Algorithm != IntegrityAlgorithmV2 {
		t.Errorf("Integrity.Algorithm = %q, want %q", manifest.Integrity.Algorithm, IntegrityAlgorithmV2)
	}

	fs := NewDirectoryPackageFS(dir)
	rc, err := fs.Open("actions/wave/action.json")
	if err != nil {
		t.Fatalf("open wave action.json: %v", err)
	}
	defer rc.Close()

	var cfg validatedActionConfig
	if err := json.NewDecoder(rc).Decode(&cfg); err != nil {
		t.Fatalf("decode wave action config: %v", err)
	}

	if cfg.PlaybackMode != "ping_pong" {
		t.Errorf("wave playbackMode = %q, want ping_pong", cfg.PlaybackMode)
	}
	if cfg.ReturnTo.Type != "action" {
		t.Errorf("wave returnTo.type = %q, want action", cfg.ReturnTo.Type)
	}
	if cfg.ReturnTo.ActionKey != "idle" {
		t.Errorf("wave returnTo.actionKey = %q, want idle", cfg.ReturnTo.ActionKey)
	}
	if len(cfg.Frames) != 4 {
		t.Errorf("wave frames count = %d, want 4", len(cfg.Frames))
	}
	for i, frame := range cfg.Frames {
		if frame.Index != i {
			t.Errorf("wave frame[%d].Index = %d, want %d", i, frame.Index, i)
		}
		if frame.File == "" {
			t.Errorf("wave frame[%d].File is empty", i)
		}
		if frame.ContentHash == "" {
			t.Errorf("wave frame[%d].ContentHash is empty", i)
		}
	}
}
