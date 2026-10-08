package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

type installedTaskDefinitionStore struct {
	task_runtime.TaskStore
	definition *task_runtime.TaskDefinition
}

func (s *installedTaskDefinitionStore) PutTaskDefinition(_ context.Context, definition *task_runtime.TaskDefinition) error {
	copy := *definition
	s.definition = &copy
	return nil
}

func TestInstalledTaskDefinitionPinsActualEntryAndInstallationGeneration(t *testing.T) {
	for _, scenario := range []string{"valid", "wrong_hash", "wrong_module", "escaped_entry"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			installed := filepath.Join(root, "installations", "com.example__task")
			bundle := filepath.Join(installed, "generations", "generation-7")
			if err := os.MkdirAll(bundle, 0700); err != nil {
				t.Fatal(err)
			}
			for path, body := range map[string][]byte{filepath.Join(installed, "current.json"): []byte(`{"generationID":"generation-7"}`), filepath.Join(bundle, "manifest.json"): []byte(`{}`), filepath.Join(bundle, "task.cjs"): []byte("module.exports = async () => ({finished:true})")} {
				if err := os.WriteFile(path, body, 0600); err != nil {
					t.Fatal(err)
				}
			}
			store := &installedTaskDefinitionStore{}
			service := task_runtime.NewTaskRuntimeService(store, task_runtime.DefaultTaskRuntimeConfig())
			installer := NewTypedContributionInstaller(&Container{ExtRoot: root, TaskRuntimeService: service})
			definition := task_runtime.TaskDefinition{Entry: "task.cjs", RuntimeType: "task"}
			switch scenario {
			case "wrong_hash":
				definition.EntryHash = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
			case "wrong_module":
				definition.ModuleID = "other"
			case "escaped_entry":
				definition.Entry = "../task.cjs"
			}
			encoded, err := json.Marshal(definition)
			if err != nil {
				t.Fatal(err)
			}
			contribution := domain.ContributionDefinition{ID: "task-contribution", ExtensionID: "com.example/task", ModuleID: "background", Kind: domain.ContributionKindBackgroundTask, Version: "1.2.3"}
			op, err := installer.buildTaskDefinitionOp(t.Context(), contribution, encoded, 7)
			if err == nil {
				err = op.doInstall(t.Context())
			}
			if scenario != "valid" {
				if err == nil || store.definition != nil {
					t.Fatal("invalid installed task definition accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(filepath.Join(bundle, "task.cjs"))
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(content)
			bundleHash, hashErr := task_runtime.TaskBundleHash(t.Context(), bundle)
			if hashErr != nil {
				t.Fatal(hashErr)
			}
			if store.definition.BundleHash != bundleHash || store.definition.EntryHash != "sha256:"+hex.EncodeToString(digest[:]) || store.definition.InstalledGeneration != 7 || store.definition.Version != "1.2.3" || store.definition.DefinitionHash == "" {
				t.Fatalf("installed task lacks source or generation pin: %+v", store.definition)
			}
			if err := os.WriteFile(filepath.Join(bundle, "task.cjs"), []byte("changed source"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := task_runtime.ResolveTaskEntry(t.Context(), bundle, store.definition); err == nil {
				t.Fatal("changed installed source executed under old pin")
			}
		})
	}
}
