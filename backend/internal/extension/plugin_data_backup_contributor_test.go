package extension

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/system/dataportability"
)

type stubInstallationRepository struct {
	installations []domain.ExtensionInstallation
}

func (s *stubInstallationRepository) PutInstallation(context.Context, domain.ExtensionInstallation) error {
	return nil
}

func (s *stubInstallationRepository) GetInstallation(_ context.Context, id domain.ExtensionID) (domain.ExtensionInstallation, error) {
	for _, installation := range s.installations {
		if installation.ExtensionID == id {
			return installation, nil
		}
	}
	return domain.ExtensionInstallation{}, fmt.Errorf("installation %s not found", id)
}

func (s *stubInstallationRepository) ListInstallations(context.Context) ([]domain.ExtensionInstallation, error) {
	return s.installations, nil
}

func (s *stubInstallationRepository) DeleteInstallation(context.Context, domain.ExtensionID) error {
	return nil
}

type stubDefinitionRepository struct {
	definitions map[string]domain.ExtensionDefinition
}

func (s *stubDefinitionRepository) PutExtension(context.Context, domain.ExtensionDefinition) error {
	return nil
}

func (s *stubDefinitionRepository) GetExtension(_ context.Context, id domain.ExtensionID, version domain.SemanticVersion) (domain.ExtensionDefinition, error) {
	definition, found := s.definitions[string(id)+"@"+version.String()]
	if !found {
		return domain.ExtensionDefinition{}, fmt.Errorf("definition %s@%s not found", id, version.String())
	}
	return definition, nil
}

func (s *stubDefinitionRepository) ListExtensions(context.Context) ([]domain.ExtensionDefinition, error) {
	items := make([]domain.ExtensionDefinition, 0, len(s.definitions))
	for _, definition := range s.definitions {
		items = append(items, definition)
	}
	return items, nil
}

func (s *stubDefinitionRepository) DeleteExtension(context.Context, domain.ExtensionID, domain.SemanticVersion) error {
	return nil
}

type nopWriteCloser struct {
	writer io.Writer
}

func (n nopWriteCloser) Write(payload []byte) (int, error) { return n.writer.Write(payload) }
func (n nopWriteCloser) Close() error                     { return nil }

type fakeBackupWriter struct {
	components map[string]*bytes.Buffer
}

func (w *fakeBackupWriter) CreateComponent(id, logicalName string, kind dataportability.ComponentKind) (io.WriteCloser, error) {
	buffer := &bytes.Buffer{}
	w.components[id] = buffer
	return nopWriteCloser{writer: buffer}, nil
}

func (w *fakeBackupWriter) WriteJSON(string, interface{}) error { return nil }

type fakeBackupReader struct {
	components map[string]*bytes.Buffer
}

func (r *fakeBackupReader) ReadComponent(id string) (io.ReadCloser, error) {
	buffer, found := r.components[id]
	if !found {
		return nil, dataportability.ErrBackupComponentFailed
	}
	return io.NopCloser(bytes.NewReader(buffer.Bytes())), nil
}

func (r *fakeBackupReader) ReadJSON(id string, value interface{}) error {
	reader, err := r.ReadComponent(id)
	if err != nil {
		return err
	}
	defer reader.Close()
	return json.NewDecoder(reader).Decode(value)
}

func (r *fakeBackupReader) ListComponents() []string {
	ids := make([]string, 0, len(r.components))
	for id := range r.components {
		ids = append(ids, id)
	}
	return ids
}

func newPluginDataContributor(t *testing.T) (*ExtensionBackupContributor, string) {
	t.Helper()
	extRoot := t.TempDir()
	runtime, err := kernel.NewRuntime(extRoot)
	if err != nil {
		t.Fatal(err)
	}
	version, err := domain.ParseVersion("0.1.100")
	if err != nil {
		t.Fatal(err)
	}
	installation := domain.ExtensionInstallation{ExtensionID: "com.amitia/dsh-compatible-host", InstalledVersion: version}
	definitions := map[string]domain.ExtensionDefinition{
		"com.amitia/dsh-compatible-host@0.1.100": {
			ID:      "com.amitia/dsh-compatible-host",
			Version: version,
			Metadata: map[string]any{
				"data.backup":             true,
				"data.export":             true,
				"data.uninstall":          "retain",
				"data.pluginData.include": []any{"dsh-data"},
			},
		},
	}
	runtime.SetContainer(&kernel.Container{
		ExtRoot:                extRoot,
		InstallationRepository: &stubInstallationRepository{installations: []domain.ExtensionInstallation{installation}},
		DefinitionRepository:   &stubDefinitionRepository{definitions: definitions},
	})
	return &ExtensionBackupContributor{Runtime: runtime}, extRoot
}

func TestPluginDataBackupExportAndRestoreUseDeclaredInclude(t *testing.T) {
	ctx := context.Background()
	contributor, extRoot := newPluginDataContributor(t)
	pluginDataRoot := kernel.PluginDataRoot(extRoot, "com.amitia/dsh-compatible-host")
	durableFile := filepath.Join(pluginDataRoot, "dsh-data", "storage", "workspace.json")
	if err := os.MkdirAll(filepath.Dir(durableFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(durableFile, []byte(`{"initialized":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cacheFile := filepath.Join(pluginDataRoot, "dsh-packages", "package.json")
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cacheFile, []byte(`{"name":"cache"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	request := dataportability.BackupRequest{Scope: dataportability.ScopeAll, Profile: dataportability.ProfileFull}
	plan, err := contributor.pluginDataPlan(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if plan.ID != extensionPluginDataComponentID || plan.ItemCount != 1 || plan.EstimatedSize != int64(len(`{"initialized":true}`)) {
		t.Fatalf("unexpected plugin data plan: %+v", plan)
	}
	writer := &fakeBackupWriter{components: map[string]*bytes.Buffer{}}
	if err := contributor.pluginDataExport(ctx, request, writer); err != nil {
		t.Fatal(err)
	}
	exported := writer.components[extensionPluginDataComponentID]
	if exported == nil {
		t.Fatal("plugin data component was not created")
	}
	lines := bytes.Split(bytes.TrimSpace(exported.Bytes()), []byte("\n"))
	if len(lines) != 1 {
		t.Fatalf("expected a single plugin data record, got %d", len(lines))
	}
	var record extensionPluginDataRecord
	if err := json.Unmarshal(lines[0], &record); err != nil {
		t.Fatal(err)
	}
	if record.ExtensionID != "com.amitia/dsh-compatible-host" || record.Path != "dsh-data/storage/workspace.json" || record.SHA256 == "" {
		t.Fatalf("unexpected plugin data record: %+v", record)
	}
	if err := os.RemoveAll(durableFile); err != nil {
		t.Fatal(err)
	}
	if err := contributor.restorePluginData(ctx, &fakeBackupReader{components: writer.components}); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(durableFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != `{"initialized":true}` {
		t.Fatalf("unexpected restored payload: %s", restored)
	}
	if _, err := os.Stat(cacheFile); err != nil {
		t.Fatalf("cache file must not be touched by plugin data restore: %v", err)
	}
}

func TestPluginDataPlanSkipsExtensionsWithoutBackupDeclaration(t *testing.T) {
	ctx := context.Background()
	contributor, extRoot := newPluginDataContributor(t)
	container := contributor.Runtime.Container()
	container.DefinitionRepository = &stubDefinitionRepository{definitions: map[string]domain.ExtensionDefinition{
		"com.amitia/dsh-compatible-host@0.1.100": {
			ID:       "com.amitia/dsh-compatible-host",
			Metadata: map[string]any{"data.backup": false, "data.pluginData.include": []any{"dsh-data"}},
		},
	}}
	durableFile := filepath.Join(kernel.PluginDataRoot(extRoot, "com.amitia/dsh-compatible-host"), "dsh-data", "storage", "workspace.json")
	if err := os.MkdirAll(filepath.Dir(durableFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(durableFile, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := contributor.pluginDataPlan(ctx, dataportability.BackupRequest{Scope: dataportability.ScopeAll})
	if err != nil {
		t.Fatal(err)
	}
	if plan.ItemCount != 0 || plan.EstimatedSize != 0 {
		t.Fatalf("expected plugin data to be skipped without data.backup, got %+v", plan)
	}
}
