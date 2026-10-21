package extension

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/auth"
	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/migration"
	"github.com/u-ai/backend/internal/extension/kernel/package_distribution"
	"github.com/u-ai/backend/internal/extension/kernel/script_host"
	"github.com/u-ai/backend/internal/runtimeprofile"
	"github.com/u-ai/backend/internal/scriptruntime/nodeenv"
	"github.com/u-ai/backend/pkg/app"
	platformruntime "github.com/u-ai/backend/pkg/platform"
)

func TestPackageDistributionProductionFlowAcrossRuntimeProfiles(t *testing.T) {
	for _, profile := range []runtimeprofile.Profile{
		runtimeprofile.ProfileLocal,
		runtimeprofile.ProfileCloudCore,
		runtimeprofile.ProfileDeviceAgent,
	} {
		t.Run(profile.String(), func(t *testing.T) {
			runPackageDistributionProductionFlow(t, profile)
		})
	}
}

func runPackageDistributionProductionFlow(t *testing.T, profile runtimeprofile.Profile) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	originalConfig := config.AppCfg
	if config.AppCfg == nil {
		config.AppCfg = &config.Config{}
	}
	t.Cleanup(func() { config.AppCfg = originalConfig })

	extensionID := "com.example/update-flow"
	v1Path := createDistributionTestArchive(t, extensionID, "1.0.0", nil)
	v2Path := createDistributionTestArchive(t, extensionID, "2.0.0", distributionMigrationDefinitions(extensionID))
	registry, publicKey := startDistributionRegistry(t, extensionID, v2Path)
	t.Setenv("AMITIA_EXTENSION_UPDATE_REGISTRY_URL", registry.URL)
	t.Setenv("AMITIA_REGISTRY_PUBLIC_KEY", base64.StdEncoding.EncodeToString(publicKey))
	t.Setenv("AMITIA_EXTENSION_UPDATE_ALLOW_PRIVATE", "true")

	root := t.TempDir()
	extRoot := filepath.Join(root, "extensions")
	nodeBinary := distributionNodeBinary(t)
	projectRoot := distributionProjectRoot(t)
	startServer := func() (*kernelruntime.Container, *httptest.Server) {
		container, err := kernelruntime.NewContainerBuilder().
			WithDBPath(filepath.Join(root, "kernel.db")).
			WithExtensionRoot(extRoot).
			WithNodeEnvironmentResolver(distributionNodeResolver{nodeBinary: nodeBinary, workDir: root}).
			WithHostArtifactResolver(distributionArtifactResolver{entryPath: filepath.Join(projectRoot, "runtime", "plugin-host", "dist", "index.js")}).
			WithRuntimeProfile(profile).
			Build(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = container.Close() })
		kernel, err := kernelruntime.NewRuntime(extRoot)
		if err != nil {
			t.Fatal(err)
		}
		kernel.SetContainer(container)
		extensionRuntime := &Runtime{Kernel: kernel}
		engine := gin.New()
		engine.Use(func(c *gin.Context) {
			c.Set("actorContext", &auth.ActorContext{
				PrincipalType:  auth.PrincipalLocalUI,
				SpaceID:        "user-1",
				DeviceID:       "device-1",
				AuthMethod:     "local_token",
				IsLocalTrusted: true,
				Permissions:    auth.OwnerDevicePermissions(),
			})
			c.Next()
		})
		RegisterRouter(engine.Group("/api"), &app.AppContext{}, extensionRuntime)
		server := httptest.NewServer(engine)
		t.Cleanup(server.Close)
		return container, server
	}
	container, server := startServer()

	installDistributionTestPackage(t, server.URL, v1Path)
	assertDistributionVersion(t, container, extensionID, "1.0.0")

	check := distributionJSON(t, server.URL, http.MethodPost, "/api/extensions/updates/check", map[string]any{"extensionId": extensionID})
	if check["available"] != true {
		t.Fatalf("update was not available: %v", check)
	}
	update, ok := check["update"].(map[string]any)
	if !ok || update["version"] != "2.0.0" {
		t.Fatalf("unexpected update response: %v", check)
	}

	download := distributionJSONWithStatus(t, server.URL, http.MethodPost, "/api/extensions/updates/downloads", map[string]any{"extensionId": extensionID, "version": "2.0.0"}, http.StatusAccepted)
	downloadID, _ := download["downloadId"].(string)
	if downloadID == "" {
		t.Fatalf("missing download id: %v", download)
	}
	waitForDistributionHTTPDownload(t, server.URL, downloadID)

	previewEnvelope := distributionJSONWithStatus(t, server.URL, http.MethodPost, "/api/extensions/updates/previews", map[string]any{
		"extensionId": extensionID,
		"downloadId":  downloadID,
		"scopeType":   "global",
		"scopeId":     "",
	}, http.StatusCreated)
	preview, ok := previewEnvelope["preview"].(map[string]any)
	if !ok {
		t.Fatalf("missing package preview: %v", previewEnvelope)
	}
	if preview["compatible"] != true {
		t.Fatalf("preview is not compatible: %v", preview)
	}
	sessionID, _ := preview["sessionId"].(string)
	confirmations := distributionConfirmations(preview)
	confirmation := distributionJSON(t, server.URL, http.MethodPost, "/api/extensions/packages/previews/"+sessionID+"/confirm", map[string]any{
		"scopeType":     "global",
		"scopeId":       "",
		"confirmations": confirmations,
	})
	token, _ := confirmation["confirmationToken"].(string)
	if token == "" {
		t.Fatalf("missing confirmation token: %v", confirmation)
	}
	updateResult := distributionJSONWithStatus(t, server.URL, http.MethodPost, "/api/extensions/packages/operations/update", map[string]any{
		"sessionId":           sessionID,
		"scopeType":           "global",
		"scopeId":             "",
		"confirmationToken":   token,
		"expectedExtensionId": extensionID,
		"idempotencyKey":      "distribution-update-flow",
	}, http.StatusAccepted)
	operationID, _ := updateResult["operationId"].(string)
	if operationID == "" {
		t.Fatalf("missing update operation: %v", updateResult)
	}
	operation := distributionJSON(t, server.URL, http.MethodGet, "/api/extensions/packages/operations/"+operationID, nil)
	if operation["status"] != "completed" {
		t.Fatalf("update operation did not complete: %v", operation)
	}
	assertDistributionVersion(t, container, extensionID, "2.0.0")
	assertDistributionMigrationStep(t, operation)
	_ = container.Close()
	container, server = startServer()
	assertDistributionVersion(t, container, extensionID, "2.0.0")

	versions := distributionJSON(t, server.URL, http.MethodGet, "/api/extensions/versions?extensionId="+url.QueryEscape(extensionID), nil)
	versionItems, _ := versions["items"].([]any)
	if len(versionItems) != 2 {
		t.Fatalf("expected two immutable versions, got %v", versions)
	}

	rollbackPreview := distributionJSON(t, server.URL, http.MethodPost, "/api/extensions/versions/rollback/preview", map[string]any{
		"extensionId": extensionID, "targetVersion": "1.0.0", "scopeType": "global", "scopeId": "",
	})
	if rollbackPreview["installable"] != true {
		t.Fatalf("rollback preview not installable: %v", rollbackPreview)
	}
	rollbackConfirmations := stringSetMap(rollbackPreview["requiredConfirmations"])
	rollbackConfirmation := distributionJSON(t, server.URL, http.MethodPost, "/api/extensions/versions/rollback/confirm", map[string]any{
		"extensionId":      extensionID,
		"targetVersion":    "1.0.0",
		"scopeType":        "global",
		"scopeId":          "",
		"previewSessionId": rollbackPreview["previewSessionId"],
		"confirmations":    rollbackConfirmations,
	})
	rollbackToken, _ := rollbackConfirmation["confirmationToken"].(string)
	if rollbackToken == "" {
		t.Fatalf("missing rollback token: %v", rollbackConfirmation)
	}
	rollbackResult := distributionJSON(t, server.URL, http.MethodPost, "/api/extensions/versions/rollback", map[string]any{
		"extensionId":       extensionID,
		"targetVersion":     "1.0.0",
		"scopeType":         "global",
		"scopeId":           "",
		"confirmationToken": rollbackToken,
	})
	if rollbackResult["version"] != "1.0.0" {
		t.Fatalf("unexpected rollback result: %v", rollbackResult)
	}
	assertDistributionVersion(t, container, extensionID, "1.0.0")

	retired := distributionJSONWithStatus(t, server.URL, http.MethodPost, "/api/extensions/retired/updates/install", map[string]any{}, http.StatusGone)
	if retired["error_code"] != "LEGACY_PACKAGE_ENDPOINT_RETIRED" {
		t.Fatalf("old update endpoint must be retired: %v", retired)
	}
}

type distributionNodeResolver struct {
	nodeBinary string
	workDir    string
}

type distributionArtifactResolver struct {
	entryPath string
}

func (r distributionArtifactResolver) Resolve(context.Context, script_host.Kind) (script_host.Artifact, error) {
	return script_host.Artifact{
		Kind:             script_host.KindPluginHost,
		EntryPath:        r.entryPath,
		DistributionRoot: filepath.Dir(filepath.Dir(r.entryPath)),
		Source:           script_host.SourceLegacyWorkspace,
	}, nil
}

func (r distributionNodeResolver) Resolve(context.Context) (nodeenv.Environment, error) {
	environment := nodeenv.Environment{
		NodeBinary:       r.nodeBinary,
		WorkDir:          r.workDir,
		DistributionRoot: filepath.Dir(filepath.Dir(r.nodeBinary)),
		Source:           nodeenv.SourceLegacyBundled,
		Guest:            distributionGuestPlatform(),
		Architecture:     runtime.GOARCH,
	}
	return environment, environment.Validate()
}

func distributionNodeBinary(t *testing.T) string {
	t.Helper()
	projectRoot := distributionProjectRoot(t)
	candidates := []string{
		filepath.Join(projectRoot, "AmitiaData", "node", "bin", "node"),
		filepath.Join(projectRoot, "AmitiaData", "node", "bin", "node.exe"),
	}
	for _, candidate := range candidates {
		absolute, _ := filepath.Abs(candidate)
		if info, err := os.Stat(absolute); err == nil && !info.IsDir() {
			return absolute
		}
	}
	t.Skip("project Node runtime is unavailable")
	return ""
}

func distributionProjectRoot(t *testing.T) string {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Skip("project source path is unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", "..", ".."))
}

func distributionGuestPlatform() platformruntime.GuestPlatform {
	switch runtime.GOOS {
	case "windows":
		return platformruntime.GuestPlatformWindows
	case "linux":
		return platformruntime.GuestPlatformLinux
	case "android":
		return platformruntime.GuestPlatformAndroid
	case "darwin":
		return platformruntime.GuestPlatformMacOS
	case "ios":
		return platformruntime.GuestPlatformIOS
	default:
		return platformruntime.GuestPlatformNone
	}
}

func installDistributionTestPackage(t *testing.T, baseURL, archivePath string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("scopeType", "global")
	_ = writer.WriteField("scopeId", "")
	part, err := writer.CreateFormFile("file", "distribution.amitiax")
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	_, copyErr := io.Copy(part, file)
	_ = file.Close()
	if copyErr != nil {
		t.Fatal(copyErr)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request, _ := http.NewRequest(http.MethodPost, baseURL+"/api/extensions/packages/artifacts", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("preview install package failed: status=%d body=%s", response.StatusCode, raw)
	}
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	preview, _ := envelope["preview"].(map[string]any)
	sessionID, _ := preview["sessionId"].(string)
	confirmation := distributionJSON(t, baseURL, http.MethodPost, "/api/extensions/packages/previews/"+sessionID+"/confirm", map[string]any{
		"scopeType":     "global",
		"scopeId":       "",
		"confirmations": distributionConfirmations(preview),
	})
	token, _ := confirmation["confirmationToken"].(string)
	operation := distributionJSONWithStatus(t, baseURL, http.MethodPost, "/api/extensions/packages/operations/install", map[string]any{
		"sessionId":         sessionID,
		"scopeType":         "global",
		"scopeId":           "",
		"confirmationToken": token,
		"idempotencyKey":    "distribution-install-v1",
	}, http.StatusAccepted)
	operationID, _ := operation["operationId"].(string)
	if operationID == "" {
		t.Fatalf("missing install operation: %v", operation)
	}
	latest := distributionJSON(t, baseURL, http.MethodGet, "/api/extensions/packages/operations/"+operationID, nil)
	if latest["status"] != "completed" {
		t.Fatalf("install operation did not complete: %v", latest)
	}
}

func waitForDistributionHTTPDownload(t *testing.T, baseURL, downloadID string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	var last map[string]any
	for time.Now().Before(deadline) {
		payload := distributionJSON(t, baseURL, http.MethodGet, "/api/extensions/updates/downloads/"+downloadID, nil)
		download, _ := payload["download"].(map[string]any)
		last = download
		switch download["status"] {
		case string(package_distribution.DownloadStatusCompleted):
			return
		case string(package_distribution.DownloadStatusFailed), string(package_distribution.DownloadStatusCancelled):
			t.Fatalf("download failed: %v", download)
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("download did not complete: %v", last)
}

func distributionJSON(t *testing.T, baseURL, method, path string, payload any) map[string]any {
	t.Helper()
	return distributionJSONWithStatus(t, baseURL, method, path, payload, http.StatusOK)
}

func distributionJSONWithStatus(t *testing.T, baseURL, method, path string, payload any, expected int) map[string]any {
	t.Helper()
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(raw)
	}
	request, err := http.NewRequest(method, baseURL+path, body)
	if err != nil {
		t.Fatal(err)
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(response.Body)
	if response.StatusCode != expected {
		t.Fatalf("%s %s returned %d, expected %d: %s", method, path, response.StatusCode, expected, raw)
	}
	var result map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &result); err != nil {
			t.Fatalf("decode %s %s: %v: %s", method, path, err, raw)
		}
	}
	return result
}

func distributionConfirmations(preview map[string]any) map[string]bool {
	return stringSetMap(preview["capabilityConfirmations"])
}

func stringSetMap(value any) map[string]bool {
	result := map[string]bool{}
	for _, item := range anyList(value) {
		key := fmt.Sprint(item)
		if key != "" {
			result[key] = true
		}
	}
	return result
}

func anyList(value any) []any {
	switch items := value.(type) {
	case []any:
		return items
	case []string:
		result := make([]any, len(items))
		for index, item := range items {
			result[index] = item
		}
		return result
	default:
		return nil
	}
}

func assertDistributionVersion(t *testing.T, container *kernelruntime.Container, extensionID, version string) {
	t.Helper()
	installation, err := container.InstallationRepository.GetInstallation(context.Background(), domain.ExtensionID(extensionID))
	if err != nil {
		t.Fatal(err)
	}
	if installation.InstalledVersion.String() != version {
		t.Fatalf("installed version = %s, want %s", installation.InstalledVersion, version)
	}
}

func assertDistributionMigrationStep(t *testing.T, operation map[string]any) {
	t.Helper()
	for _, item := range anyList(operation["steps"]) {
		step, _ := item.(map[string]any)
		if step["name"] == kernelruntime.StepExecuteMigrations && step["status"] == "completed" {
			return
		}
	}
	t.Fatalf("migration step missing from completed operation: %v", operation["steps"])
}

func startDistributionRegistry(t *testing.T, extensionID, archivePath string) (*httptest.Server, ed25519.PublicKey) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/extensions/" + extensionID + "/latest":
			index := package_distribution.ReleaseIndex{
				SchemaVersion: 1, GeneratedAt: time.Now().UTC().Format(time.RFC3339),
				ExtensionID: extensionID, PublisherID: "com.example", PublisherKeyID: "distribution-test",
				Channel: "stable", Version: "2.0.0", DownloadURL: "v2.amitiax",
				SignatureURL: "v2.amitiax.sig",
				SHA256:       hex.EncodeToString(sum[:]), Platforms: []string{runtime.GOOS},
				Architectures: []string{runtime.GOARCH}, PackageSize: int64(len(payload)), ManifestVersion: 1,
			}
			signed, signErr := package_distribution.SignReleaseIndex(index, privateKey)
			if signErr != nil {
				http.Error(w, signErr.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(signed)
		case "/v2.amitiax":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	originalTransport := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	return server, publicKey
}

func distributionMigrationDefinitions(extensionID string) []migration.MigrationDefinition {
	forward := "distribution-forward"
	reverse := "distribution-reverse"
	return []migration.MigrationDefinition{
		{
			MigrationID: forward, ExtensionID: extensionID, ModuleID: "main", FromVersionRange: "1.0.0", ToVersion: "2.0.0",
			Entry: "migrations/distribution-forward.js", RuntimeType: "javascript", Direction: migration.DirectionForward,
			Reversibility: migration.ReversibilityFullyReversible, Idempotency: migration.IdempotencyIdempotent,
			DefinitionHash: "distribution-forward-hash", ReverseMigrationID: &reverse,
		},
		{
			MigrationID: reverse, ExtensionID: extensionID, ModuleID: "main", FromVersionRange: "2.0.0", ToVersion: "1.0.0",
			Entry: "migrations/distribution-reverse.js", RuntimeType: "javascript", Direction: migration.DirectionReverse,
			Reversibility: migration.ReversibilityFullyReversible, Idempotency: migration.IdempotencyIdempotent,
			DefinitionHash: "distribution-reverse-hash", ForwardMigrationID: &forward,
		},
	}
}

func createDistributionTestArchive(t *testing.T, extensionID, version string, definitions []migration.MigrationDefinition) string {
	t.Helper()
	extension := map[string]any{
		"id": extensionID, "name": map[string]any{"default": "Distribution Test"},
		"description": map[string]any{"default": "Production distribution flow"},
		"version":     version,
	}
	if len(definitions) > 0 {
		extension["metadata"] = map[string]any{"migrations": map[string]any{"definitions": definitions}}
	}
	manifest := map[string]any{
		"manifestVersion": 1,
		"extension":       extension,
		"publisher":       map[string]any{"id": "com.example", "displayName": "Example"},
		"compatibility":   map[string]any{},
		"modules": []any{map[string]any{
			"id": "main", "name": map[string]any{"default": "Main"}, "type": "javascript", "version": version,
			"runtime": map[string]any{"type": "javascript", "entryPoint": "index.js"},
			"contributions": []any{map[string]any{
				"id": "distribution-tool", "kind": "tool",
				"name": map[string]any{"default": "Distribution Tool"}, "version": version,
			}},
		}},
		"integrity": map[string]any{"algorithm": "sha256", "contentTreeHash": ""},
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"manifest.json":         manifestRaw,
		"modules/main/index.js": []byte("export const version = " + fmt.Sprintf("%q", version)),
	}
	for _, definition := range definitions {
		files[definition.Entry] = []byte(`module.exports = {
  activate(context) {
    context.handlers.bindTool("up", async (input) => ({ ok: true, input }));
    context.handlers.bindTool("down", async (input) => ({ ok: true, input }));
  },
  deactivate() {},
};
`)
	}
	entries := make(map[string]amitiax.FileEntry, len(files))
	list := make([]amitiax.FileEntry, 0, len(files))
	for name, content := range files {
		sum := sha256.Sum256(content)
		entry := amitiax.FileEntry{Path: name, Size: int64(len(content)), Hash: hex.EncodeToString(sum[:])}
		entries[name] = entry
		list = append(list, entry)
	}
	filesRaw, _ := json.Marshal(amitiax.IntegrityFilesDoc{Algorithm: "sha256", Files: entries, GeneratedAt: time.Now().UTC()})
	treeRaw, _ := json.Marshal(amitiax.IntegrityTreeDoc{Algorithm: "sha256", TreeHash: amitiax.ComputeTreeHash(list), GeneratedAt: time.Now().UTC()})
	files["integrity/files.json"] = filesRaw
	files["integrity/content-tree.json"] = treeRaw
	path := filepath.Join(t.TempDir(), "distribution-"+version+".amitiax")
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(out)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		entry, createErr := writer.Create(name)
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, writeErr := entry.Write(files[name]); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
