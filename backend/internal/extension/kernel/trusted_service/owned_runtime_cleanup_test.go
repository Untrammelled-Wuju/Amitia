package trusted_service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupOwnedRuntimeRemovesOnlyDeclaredAssets(t *testing.T) {
	root := t.TempDir()
	asset := filepath.Join(root, "data", "mobile")
	if err := os.MkdirAll(asset, 0700); err != nil {
		t.Fatal(err)
	}
	retained := filepath.Join(root, "data", "qq-personal-config.json")
	if err := os.WriteFile(retained, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	receipt, _ := json.Marshal(ownedRuntimeReceipt{ExtensionID: "test/qq", Paths: []string{"data/mobile"}})
	if err := os.WriteFile(filepath.Join(root, ".amitia-owned-runtime.json"), receipt, 0600); err != nil {
		t.Fatal(err)
	}
	if err := cleanupOwnedRuntime(root, "test/qq"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(asset); !os.IsNotExist(err) {
		t.Fatalf("asset remains: %v", err)
	}
	if _, err := os.Stat(retained); err != nil {
		t.Fatal(err)
	}
	if err := cleanupOwnedRuntime(root, "test/qq"); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupOwnedRuntimeRejectsEscapeBeforeDeleting(t *testing.T) {
	root := t.TempDir()
	asset := filepath.Join(root, "mobile")
	if err := os.Mkdir(asset, 0700); err != nil {
		t.Fatal(err)
	}
	receipt, _ := json.Marshal(ownedRuntimeReceipt{ExtensionID: "test/qq", Paths: []string{"mobile", "../outside"}})
	if err := os.WriteFile(filepath.Join(root, ".amitia-owned-runtime.json"), receipt, 0600); err != nil {
		t.Fatal(err)
	}
	if err := cleanupOwnedRuntime(root, "test/qq"); err == nil {
		t.Fatal("escape accepted")
	}
	if _, err := os.Stat(asset); err != nil {
		t.Fatal("asset deleted before validation")
	}
	if err := cleanupOwnedRuntime(root, "other/plugin"); err == nil {
		t.Fatal("identity mismatch accepted")
	}
}

func TestCleanupOwnedRuntimeFindsStoppedService(t *testing.T) {
	root := t.TempDir()
	working := filepath.Join(root, "services", "qq", "instance")
	asset := filepath.Join(working, "data", "mobile")
	if err := os.MkdirAll(asset, 0700); err != nil {
		t.Fatal(err)
	}
	receipt, _ := json.Marshal(ownedRuntimeReceipt{ExtensionID: "test/qq", Paths: []string{"data/mobile"}})
	if err := os.WriteFile(filepath.Join(working, ".amitia-owned-runtime.json"), receipt, 0600); err != nil {
		t.Fatal(err)
	}
	s := NewProcessSupervisor(root)
	if err := s.CleanupOwnedRuntime("other/plugin"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(asset); err != nil {
		t.Fatal("another plugin's asset was removed")
	}
	if err := s.CleanupOwnedRuntime("test/qq"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(asset); !os.IsNotExist(err) {
		t.Fatalf("stopped service asset remains: %v", err)
	}
}

func TestCleanupOwnedRuntimeRemovesOwnedLinkAndPreservesReplacement(t *testing.T) {
	root := t.TempDir()
	asset := filepath.Join(root, "mobile")
	if err := os.MkdirAll(asset, 0700); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	owned := filepath.Join(outside, "owned")
	replaced := filepath.Join(outside, "replaced")
	source := filepath.Join(asset, "xkbcomp")
	if err := os.Symlink(source, owned); err != nil {
		t.Skip(err)
	}
	if err := os.Symlink("other-program", replaced); err != nil {
		t.Fatal(err)
	}
	links, _ := json.Marshal([]map[string]string{{"target": owned, "source": source}, {"target": replaced, "source": source}})
	if err := os.WriteFile(filepath.Join(asset, "links.json"), links, 0600); err != nil {
		t.Fatal(err)
	}
	receipt, _ := json.Marshal(ownedRuntimeReceipt{ExtensionID: "test/qq", Paths: []string{"mobile"}, LinksFile: "mobile/links.json"})
	if err := os.WriteFile(filepath.Join(root, ".amitia-owned-runtime.json"), receipt, 0600); err != nil {
		t.Fatal(err)
	}
	if err := cleanupOwnedRuntime(root, "test/qq"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(owned); !os.IsNotExist(err) {
		t.Fatal("owned link remains")
	}
	if target, err := os.Readlink(replaced); err != nil || target != "other-program" {
		t.Fatal("replacement link was removed")
	}
}
