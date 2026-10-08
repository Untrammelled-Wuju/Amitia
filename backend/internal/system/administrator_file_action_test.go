package system

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestAdministratorFileActionReportsPartialCompletionAndRealFailure(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.Mkdir("logs", 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.log", "b.log"} {
		if err := os.WriteFile(filepath.Join("logs", name), []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	checks := 0
	ctx := coordination.WithAdditionalGuard(t.Context(), func(_ context.Context) error {
		checks++
		if checks >= 4 {
			return errors.New("revoked fixture authority")
		}
		return nil
	})
	result, err := (&service{}).AdministratorActionContext(ctx, "logs-delete", "")
	if err == nil || result["processedFiles"] != 1 || result["deleted"] == true {
		t.Fatal("partial file deletion reported complete", result, err)
	}
	if _, err := os.Stat(filepath.Join("logs", "a.log")); !os.IsNotExist(err) {
		t.Fatal("first authorized deletion did not occur")
	}
	if _, err := os.Stat(filepath.Join("logs", "b.log")); err != nil {
		t.Fatal("late file deletion was not canceled")
	}
	if err := os.Mkdir(filepath.Join("logs", "b.log.old"), 0700); err != nil {
		t.Fatal(err)
	}
	result, err = (&service{}).AdministratorActionContext(t.Context(), "logs-rotate", "")
	if err == nil || result["rotated"] == true {
		t.Fatal("failed file rename falsely reported success")
	}
	if _, err := os.Stat(filepath.Join("logs", "b.log")); err != nil {
		t.Fatal("failed rotation destroyed source")
	}
}
