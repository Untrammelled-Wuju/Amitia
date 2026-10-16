package gitworktree

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitCommandIgnoresInheritedRepositoryAndConfigOverrides(t *testing.T) {
	m, root := fixture(t)
	expected := gitTest(t, root, "rev-parse", "HEAD")
	t.Setenv("GIT_DIR", t.TempDir())
	t.Setenv("GIT_INDEX_FILE", filepath.Join(t.TempDir(), "wrong-index"))
	t.Setenv("GIT_WORK_TREE", t.TempDir())
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "filter.injected.process")
	t.Setenv("GIT_CONFIG_VALUE_0", "false")
	got, err := m.command(context.Background(), root, "rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(string(got)) != expected {
		t.Fatalf("git escaped intended repository: %q %v", got, err)
	}
	if err = m.refuseConfiguredFilters(context.Background(), root); err != nil {
		t.Fatalf("untrusted environment injected filter: %v", err)
	}
	entries, err := m.List(context.Background(), root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("repository discovery under inherited environment: %v %+v", err, entries)
	}
}

func TestCheckpointTestEnvironmentResetsUnsafeFlags(t *testing.T) {
	t.Setenv("LOCALAPPDATA", filepath.Join(t.TempDir(), "local-app-data"))
	t.Setenv("GOFLAGS", "-run=^$")
	t.Setenv("GO111MODULE", "off")
	t.Setenv("GIT_DIR", t.TempDir())
	t.Setenv("GOTOOLCHAIN", "garbage")
	t.Setenv("GOENV", t.TempDir())
	env := safeCheckpointTestEnvironment()
	values := map[string]string{}
	for _, item := range env {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			values[key] = value
		}
	}
	if values["GOFLAGS"] != "" || values["GO111MODULE"] != "on" || values["GOWORK"] != "off" || values["GOENV"] != "off" || values["GOTOOLCHAIN"] != "local" || values["GIT_DIR"] != "" {
		t.Fatalf("unsafe environment reached child: %v", values)
	}
	if values["GIT_CONFIG_GLOBAL"] != os.DevNull {
		t.Fatal("unexpected global config")
	}
	localAppData := ""
	for key, value := range values {
		if strings.EqualFold(key, "LOCALAPPDATA") {
			localAppData = value
		}
	}
	if localAppData != os.Getenv("LOCALAPPDATA") {
		t.Fatal("Windows build cache location was not preserved")
	}
}

func TestCheckpointTestEnvironmentDoesNotInheritHostSecrets(t *testing.T) {
	t.Setenv("AMITIA_JWT_SECRET", "private-token-in-host")
	t.Setenv("DATABASE_PASSWORD", "private-database-password")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "private-cloud-key")
	t.Setenv("OPENAI_API_KEY", "private-model-key")
	t.Setenv("HTTP_PROXY", "http://untrusted.internal:8080")
	t.Setenv("CGO_CFLAGS", "-DUNTRUSTED_CODE=1")
	t.Setenv("PATH", os.Getenv("PATH"))
	env := safeCheckpointTestEnvironment()
	values := map[string]string{}
	for _, item := range env {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			values[strings.ToUpper(key)] = value
		}
	}
	for _, key := range []string{"AMITIA_JWT_SECRET", "DATABASE_PASSWORD", "AWS_SECRET_ACCESS_KEY", "OPENAI_API_KEY", "HTTP_PROXY", "CGO_CFLAGS"} {
		if _, exists := values[key]; exists {
			t.Fatalf("host secret or execution control %s propagated to test subprocess", key)
		}
	}
	for _, key := range []string{"PATH", "GOFLAGS", "GOTOOLCHAIN", "GOENV", "GIT_CONFIG_GLOBAL"} {
		if _, exists := values[key]; !exists {
			t.Fatalf("required controlled key %s not supplied", key)
		}
	}
}
