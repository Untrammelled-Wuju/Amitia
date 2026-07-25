// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package installation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/desktoppet/processing"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

const (
	testUserID       = "user_test"
	testCharacterID  = "char_test"
	testTaskID       = "gt_test"
	testPackageID    = "pkg_test"
	testCanvasWidth  = 8
	testCanvasHeight = 8
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "installation_test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlDB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&Installation{}, &RuntimeSettings{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func newTestContext(db *gorm.DB) *app.AppContext {
	return &app.AppContext{DB: db, Context: context.Background()}
}

func newTestService(t *testing.T, db *gorm.DB, dataDir string, pkgRepo processing.Repository, charRepo character.Repository) Service {
	t.Helper()
	ctx := newTestContext(db)
	repo := NewRepository(db, ctx)
	inst := NewInstaller(repo, pkgRepo, charRepo, dataDir)
	un := NewUninstaller(repo, dataDir)
	return NewService(repo, inst, un, pkgRepo, charRepo, dataDir)
}

func newTestInstaller(t *testing.T, db *gorm.DB, dataDir string, pkgRepo processing.Repository, charRepo character.Repository) (Installer, Repository) {
	t.Helper()
	ctx := newTestContext(db)
	repo := NewRepository(db, ctx)
	return NewInstaller(repo, pkgRepo, charRepo, dataDir), repo
}

func newTestUninstaller(t *testing.T, db *gorm.DB, dataDir string) Uninstaller {
	t.Helper()
	ctx := newTestContext(db)
	repo := NewRepository(db, ctx)
	return NewUninstaller(repo, dataDir)
}

type stubPackageRepo struct {
	processing.Repository
	pkgs map[string]*processing.Package
	err  error
}

func (s *stubPackageRepo) GetPackage(id string) (*processing.Package, error) {
	if s.err != nil {
		return nil, s.err
	}
	if pkg, ok := s.pkgs[id]; ok {
		return pkg, nil
	}
	return nil, gorm.ErrRecordNotFound
}

type stubCharRepo struct {
	character.Repository
	chars map[string]*character.Character
	err   error
}

func (s *stubCharRepo) FindByID(id string) (*character.Character, error) {
	if s.err != nil {
		return nil, s.err
	}
	if c, ok := s.chars[id]; ok {
		return c, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func newDefaultStubRepos() (*stubPackageRepo, *stubCharRepo) {
	pkgRepo := &stubPackageRepo{pkgs: map[string]*processing.Package{}}
	charRepo := &stubCharRepo{chars: map[string]*character.Character{}}
	charRepo.chars[testCharacterID] = &character.Character{ID: testCharacterID, Name: "测试角色", Status: "enabled"}
	return pkgRepo, charRepo
}

type mockNotifier struct {
	enabledCalls    []mockEnabledCall
	disabledCalls   []mockDisabledCall
	actionPlayCalls []mockActionPlayCall
	recenterCalls   []string
	defaultChanged  []mockDefaultChangedCall
	settingsUpdated []mockSettingsUpdatedCall
	failOnAction    bool
}

type mockEnabledCall struct {
	UserID         string
	InstallationID string
	Settings       *RuntimeSettings
}

type mockDisabledCall struct {
	UserID         string
	InstallationID string
}

type mockActionPlayCall struct {
	UserID         string
	InstallationID string
	ActionKey      string
}

type mockDefaultChangedCall struct {
	InstallationID string
	ActionKey      string
}

type mockSettingsUpdatedCall struct {
	InstallationID string
	Settings       map[string]interface{}
}

func (m *mockNotifier) NotifyInstallationEnabled(userId, installationId string, settings *RuntimeSettings) error {
	m.enabledCalls = append(m.enabledCalls, mockEnabledCall{UserID: userId, InstallationID: installationId, Settings: settings})
	return nil
}

func (m *mockNotifier) NotifyInstallationDisabled(userId, installationId string) error {
	m.disabledCalls = append(m.disabledCalls, mockDisabledCall{UserID: userId, InstallationID: installationId})
	return nil
}

func (m *mockNotifier) NotifyActionPlayed(userId, installationId, actionKey string) error {
	if m.failOnAction {
		return fmt.Errorf("模拟调度器失败")
	}
	m.actionPlayCalls = append(m.actionPlayCalls, mockActionPlayCall{UserID: userId, InstallationID: installationId, ActionKey: actionKey})
	return nil
}

func (m *mockNotifier) NotifyRecenter(installationId string) error {
	m.recenterCalls = append(m.recenterCalls, installationId)
	return nil
}

func (m *mockNotifier) NotifyDefaultActionChanged(installationId, actionKey string) error {
	m.defaultChanged = append(m.defaultChanged, mockDefaultChangedCall{InstallationID: installationId, ActionKey: actionKey})
	return nil
}

func (m *mockNotifier) NotifyRuntimeSettingsUpdated(installationId string, settings map[string]interface{}) error {
	m.settingsUpdated = append(m.settingsUpdated, mockSettingsUpdatedCall{InstallationID: installationId, Settings: settings})
	return nil
}

func assertInstallationError(t *testing.T, err error, expectedCode string) {
	t.Helper()
	if err == nil {
		t.Fatalf("期望错误码 %s 但得到 nil", expectedCode)
	}
	var ie *InstallationError
	if !errors.As(err, &ie) {
		t.Fatalf("期望 *InstallationError，实际类型 %T: %v", err, err)
	}
	if ie.Code != expectedCode {
		t.Fatalf("错误码 = %s, 期望 %s (消息=%s)", ie.Code, expectedCode, ie.Message)
	}
}

func makePNGBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func writeFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func resolveDataDir(t *testing.T, dataDir string) string {
	t.Helper()
	if resolved, err := filepath.EvalSymlinks(dataDir); err == nil {
		return resolved
	}
	return dataDir
}

func removeAllDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("os.RemoveAll %s: %v", dir, err)
	}
	if _, err := os.Stat(dir); err == nil {
		if runtime.GOOS == "windows" {
			cmd := exec.Command("cmd", "/c", "rmdir", "/s", "/q", dir)
			if err := cmd.Run(); err != nil {
				t.Fatalf("rmdir %s: %v", dir, err)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
}

func waitForDirDeleted(t *testing.T, dir string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		f, err := os.Open(dir)
		if err != nil {
			return
		}
		f.Close()
		time.Sleep(10 * time.Millisecond)
	}
	if f, err := os.Open(dir); err == nil {
		f.Close()
		if runtime.GOOS == "windows" {
			cmd := exec.Command("cmd", "/c", "rmdir", "/s", "/q", dir)
			_ = cmd.Run()
			time.Sleep(50 * time.Millisecond)
			if f2, err2 := os.Open(dir); err2 != nil {
				return
			} else {
				f2.Close()
			}
		}
		t.Fatalf("目录应已删除: %s", dir)
	}
}

func packageSourceDir(dataDir, taskID, pkgID string) string {
	return filepath.Join(dataDir, "desktop-pets", "generation-tasks", taskID, "packages", pkgID)
}

type actionSpec struct {
	Key                  string
	Name                 string
	FrameCount           int
	SupportsDefaultIdle  *bool
}

func boolPtr(v bool) *bool { return &v }

func createPackageOnDisk(t *testing.T, dataDir, taskID, pkgID string, canvasW, canvasH int, defaultAction string, actions []actionSpec) string {
	t.Helper()
	srcDir := packageSourceDir(dataDir, taskID, pkgID)

	manifestActions := make([]processing.ManifestAction, 0, len(actions))
	for _, a := range actions {
		manifestActions = append(manifestActions, processing.ManifestAction{
			Key:    a.Key,
			Name:   a.Name,
			Config: fmt.Sprintf("actions/%s/action.json", a.Key),
		})
	}

	manifest := &processing.Manifest{
		SchemaVersion:    processing.ManifestSchemaVersion,
		PackageID:        pkgID,
		Name:             "测试包",
		CharacterID:      testCharacterID,
		GenerationTaskID: taskID,
		ProcessingVersion: 1,
		Canvas:            processing.ManifestCanvas{Width: canvasW, Height: canvasH},
		DefaultAction:     defaultAction,
		Preview:           "preview.png",
		Actions:           manifestActions,
		Capabilities: processing.ManifestCapabilities{
			HasTransparentBackground: true,
			SupportsFrameSequence:    true,
		},
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	writeFile(t, filepath.Join(srcDir, "manifest.json"), manifestData)
	writeFile(t, filepath.Join(srcDir, "preview.png"), makePNGBytes(t, canvasW, canvasH))

	for _, a := range actions {
		actionJSON := buildTestActionJSON(a.Key, a.Name, a.FrameCount, a.SupportsDefaultIdle)
		actionData, err := json.MarshalIndent(actionJSON, "", "  ")
		if err != nil {
			t.Fatalf("marshal action.json for %s: %v", a.Key, err)
		}
		writeFile(t, filepath.Join(srcDir, "actions", a.Key, "action.json"), actionData)

		framesDir := filepath.Join(srcDir, "actions", a.Key, "frames")
		if err := os.MkdirAll(framesDir, 0o755); err != nil {
			t.Fatalf("mkdir frames: %v", err)
		}
		for i := 0; i < a.FrameCount; i++ {
			frameFile := fmt.Sprintf("frame-%04d.png", i+1)
			writeFile(t, filepath.Join(framesDir, frameFile), makePNGBytes(t, canvasW, canvasH))
		}
	}

	return srcDir
}

type testActionJSON struct {
	Key                 string                 `json:"key"`
	Name                string                 `json:"name"`
	Version             int                    `json:"version"`
	LoopType            string                 `json:"loopType"`
	Fps                 int                    `json:"fps"`
	FrameDurationMs     int                    `json:"frameDurationMs"`
	FrameCount          int                    `json:"frameCount"`
	Frames              []processing.FrameInfo `json:"frames"`
	Anchor              processing.AnchorJSON  `json:"anchor"`
	Interruptible       bool                   `json:"interruptible"`
	ReturnAction        string                 `json:"returnAction"`
	SupportsDefaultIdle *bool                  `json:"supportsDefaultIdle,omitempty"`
}

func buildTestActionJSON(key, name string, frameCount int, supportsIdle *bool) *testActionJSON {
	fps := 10
	durationMs := 100
	frames := make([]processing.FrameInfo, frameCount)
	for i := 0; i < frameCount; i++ {
		frames[i] = processing.FrameInfo{
			Index:      i,
			File:       fmt.Sprintf("frames/frame-%04d.png", i+1),
			DurationMs: durationMs,
		}
	}
	returnAction := ""
	if processing.IsLoopAction(key) {
		returnAction = "idle_normal"
	}
	return &testActionJSON{
		Key:                 key,
		Name:                name,
		Version:             1,
		LoopType:            "loop",
		Fps:                 fps,
		FrameDurationMs:     durationMs,
		FrameCount:          frameCount,
		Frames:              frames,
		Anchor:              processing.AnchorJSON{Type: "feet_center", X: 0.5, Y: 0.92},
		Interruptible:       true,
		ReturnAction:        returnAction,
		SupportsDefaultIdle: supportsIdle,
	}
}

func computePackageHash(t *testing.T, dir string) string {
	t.Helper()
	files, err := listInstallationFiles(dir)
	if err != nil {
		t.Fatalf("list files: %v", err)
	}
	sort.Strings(files)
	hasher := sha256.New()
	for _, relPath := range files {
		hasher.Write([]byte(relPath))
		hasher.Write([]byte{0})
		absPath := filepath.Join(dir, filepath.FromSlash(relPath))
		f, err := os.Open(absPath)
		if err != nil {
			t.Fatalf("open file %s: %v", absPath, err)
		}
		if _, err := io.Copy(hasher, f); err != nil {
			f.Close()
			t.Fatalf("copy file %s: %v", absPath, err)
		}
		f.Close()
		hasher.Write([]byte{0})
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func createReadyPackage(t *testing.T, dataDir, pkgID string, actions []actionSpec) *processing.Package {
	t.Helper()
	srcDir := createPackageOnDisk(t, dataDir, testTaskID, pkgID, testCanvasWidth, testCanvasHeight, "idle_normal", actions)
	hash := computePackageHash(t, srcDir)
	return &processing.Package{
		ID:               pkgID,
		UserID:           testUserID,
		CharacterID:      testCharacterID,
		GenerationTaskID: testTaskID,
		Name:             "测试包",
		Version:          1,
		Status:           "ready",
		DefaultActionKey: "idle_normal",
		CanvasWidth:      testCanvasWidth,
		CanvasHeight:     testCanvasHeight,
		PackageHash:      hash,
		ActionCount:      len(actions),
	}
}

func defaultTestActions() []actionSpec {
	return []actionSpec{
		{Key: "idle_normal", Name: "待机", FrameCount: 2, SupportsDefaultIdle: boolPtr(true)},
		{Key: "wave", Name: "招手", FrameCount: 1, SupportsDefaultIdle: boolPtr(false)},
	}
}

func seedInstallation(t *testing.T, db *gorm.DB, inst *Installation) {
	t.Helper()
	if err := db.Create(inst).Error; err != nil {
		t.Fatalf("seed installation: %v", err)
	}
}

func seedRuntimeSettings(t *testing.T, db *gorm.DB, settings *RuntimeSettings) {
	t.Helper()
	if err := db.Create(settings).Error; err != nil {
		t.Fatalf("seed runtime settings: %v", err)
	}
}

func getInstallationFromDB(t *testing.T, db *gorm.DB, id string) *Installation {
	t.Helper()
	var inst Installation
	if err := db.Where("id = ?", id).First(&inst).Error; err != nil {
		t.Fatalf("query installation %s: %v", id, err)
	}
	return &inst
}

func getRuntimeSettingsFromDB(t *testing.T, db *gorm.DB, installationID string) *RuntimeSettings {
	t.Helper()
	var rs RuntimeSettings
	if err := db.Where("installation_id = ?", installationID).First(&rs).Error; err != nil {
		t.Fatalf("query runtime settings for %s: %v", installationID, err)
	}
	return &rs
}

func createInstalledPackageOnDisk(t *testing.T, dataDir, installID string, actions []actionSpec) (*Installation, string) {
	t.Helper()
	installDir := filepath.Join(dataDir, "desktop-pets", "installed", installID)
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatalf("mkdir install dir: %v", err)
	}

	manifestActions := make([]processing.ManifestAction, 0, len(actions))
	for _, a := range actions {
		manifestActions = append(manifestActions, processing.ManifestAction{
			Key:    a.Key,
			Name:   a.Name,
			Config: fmt.Sprintf("actions/%s/action.json", a.Key),
		})
	}

	manifest := &processing.Manifest{
		SchemaVersion:    processing.ManifestSchemaVersion,
		PackageID:        testPackageID,
		Name:             "测试包",
		CharacterID:      testCharacterID,
		GenerationTaskID: testTaskID,
		ProcessingVersion: 1,
		Canvas:            processing.ManifestCanvas{Width: testCanvasWidth, Height: testCanvasHeight},
		DefaultAction:     "idle_normal",
		Preview:           "preview.png",
		Actions:           manifestActions,
		Capabilities: processing.ManifestCapabilities{
			HasTransparentBackground: true,
			SupportsFrameSequence:    true,
		},
	}
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	writeFile(t, filepath.Join(installDir, "manifest.json"), manifestData)
	writeFile(t, filepath.Join(installDir, "preview.png"), makePNGBytes(t, testCanvasWidth, testCanvasHeight))

	for _, a := range actions {
		actionJSON := buildTestActionJSON(a.Key, a.Name, a.FrameCount, a.SupportsDefaultIdle)
		actionData, _ := json.MarshalIndent(actionJSON, "", "  ")
		writeFile(t, filepath.Join(installDir, "actions", a.Key, "action.json"), actionData)
		framesDir := filepath.Join(installDir, "actions", a.Key, "frames")
		if err := os.MkdirAll(framesDir, 0o755); err != nil {
			t.Fatalf("mkdir frames: %v", err)
		}
		for i := 0; i < a.FrameCount; i++ {
			frameFile := fmt.Sprintf("frame-%04d.png", i+1)
			writeFile(t, filepath.Join(framesDir, frameFile), makePNGBytes(t, testCanvasWidth, testCanvasHeight))
		}
	}

	hash := computePackageHash(t, installDir)
	relPath := filepath.ToSlash(filepath.Join("desktop-pets", "installed", installID)) + "/"
	manifestRel := filepath.ToSlash(filepath.Join(relPath, "manifest.json"))
	previewRel := filepath.ToSlash(filepath.Join(relPath, "preview.png"))

	inst := &Installation{
		ID:               installID,
		UserID:           testUserID,
		CharacterID:      testCharacterID,
		PackageID:        testPackageID,
		PackageVersion:   "1",
		Name:             "测试包",
		Status:           StatusInstalled,
		IsActive:         0,
		InstallPath:      relPath,
		ManifestPath:     manifestRel,
		PreviewPath:      previewRel,
		DefaultActionKey: "idle_normal",
		CanvasWidth:      testCanvasWidth,
		CanvasHeight:     testCanvasHeight,
		PackageHash:      hash,
		CreatedAt:        "2026-07-25 10:00:00",
		UpdatedAt:        "2026-07-25 10:00:00",
	}
	return inst, installDir
}

func setupInstalledService(t *testing.T) (Service, *gorm.DB, string, *Installation, *mockNotifier) {
	t.Helper()
	db := setupTestDB(t)
	dataDir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dataDir); err == nil {
		dataDir = resolved
	}

	pkgRepo, charRepo := newDefaultStubRepos()
	pkg := createReadyPackage(t, dataDir, testPackageID, defaultTestActions())
	pkgRepo.pkgs[testPackageID] = pkg

	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)
	notifier := &mockNotifier{}
	SetRuntimeNotifier(svc, notifier)

	inst, err := svc.InstallPackage(testPackageID, testUserID, testCharacterID)
	if err != nil {
		t.Fatalf("InstallPackage: %v", err)
	}
	return svc, db, dataDir, inst, notifier
}
