package configwrite_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/search"
)

type configurationCredentialVault struct {
	values      map[string]string
	next        int
	deleteFails bool
	storeHook   func()
}

func (v *configurationCredentialVault) Store(ctx context.Context, namespace string, value []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	v.next++
	ref := fmt.Sprintf("secret://%s/%d", namespace, v.next)
	v.values[ref] = string(value)
	if v.storeHook != nil {
		v.storeHook()
	}
	return ref, nil
}

func (v *configurationCredentialVault) Resolve(ctx context.Context, ref string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	value, exists := v.values[ref]
	if !exists {
		return nil, errors.New("missing fixture secret")
	}
	return []byte(value), nil
}

func (v *configurationCredentialVault) Delete(ctx context.Context, ref string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if v.deleteFails {
		return errors.New("injected vault cleanup failure")
	}
	delete(v.values, strings.Replace(ref, "mcp-secret://", "secret://", 1))
	return nil
}

func TestConfigurationSearchCredentialCleanupKeepsActiveCanonicalAlias(t *testing.T) {
	db, _ := configurationDB(t)
	raw, _ := db.DB()
	vault := &configurationCredentialVault{values: map[string]string{}}
	store := search.NewCredentialStore(raw).WithVault(vault)
	if _, err := store.Set(t.Context(), "serper", "shared-fixture"); err != nil {
		t.Fatal(err)
	}
	var canonical string
	if err := raw.QueryRow("SELECT api_key FROM search_api_keys WHERE engine_id='serper'").Scan(&canonical); err != nil {
		t.Fatal(err)
	}
	legacy := strings.Replace(canonical, "secret://", "mcp-secret://", 1)
	if _, err := raw.Exec("UPDATE search_api_keys SET api_key=? WHERE engine_id='serper'", legacy); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("INSERT INTO search_api_keys(engine_id,api_key,updated_at) VALUES('youtube',?,CURRENT_TIMESTAMP)", canonical); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(t.Context(), "serper"); err != nil {
		t.Fatal(err)
	}
	if vault.values[canonical] != "shared-fixture" {
		t.Fatal("legacy alias cleanup destroyed another active canonical credential")
	}
}

func TestConfigurationSearchCredentialsRejectAdministratorABA(t *testing.T) {
	db, svc := configurationDB(t)
	ctx, finish := configurationScope(t, svc)
	defer finish()
	policy, err := svc.Get(t.Context(), "space", "admin")
	if err != nil {
		t.Fatal(err)
	}
	policy, err = svc.GrantAdministrator(t.Context(), "space", "admin", policy.PermissionRevision, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.GrantAdministrator(t.Context(), "space", "admin", policy.PermissionRevision, true); err != nil {
		t.Fatal(err)
	}
	raw, _ := db.DB()
	vault := &configurationCredentialVault{values: map[string]string{}}
	store := search.NewCredentialStore(raw).WithVault(vault)
	if _, err := store.Set(ctx, "serper", "fixture"); err == nil {
		t.Fatal("old credential write survived ABA")
	}
	if err := store.Delete(ctx, "serper"); err == nil {
		t.Fatal("old credential delete survived ABA")
	}
	if len(vault.values) != 0 {
		t.Fatal("revoked request created a secret")
	}
}

func TestConfigurationSearchCredentialStageCancellationAndSQLFailureKeepOldSecret(t *testing.T) {
	for _, failure := range []string{"cancel", "sql"} {
		t.Run(failure, func(t *testing.T) {
			db, svc := configurationDB(t)
			raw, _ := db.DB()
			vault := &configurationCredentialVault{values: map[string]string{}}
			store := search.NewCredentialStore(raw).WithVault(vault)
			if _, err := store.Set(t.Context(), "serper", "old-fixture"); err != nil {
				t.Fatal(err)
			}
			var old string
			if err := raw.QueryRow("SELECT api_key FROM search_api_keys WHERE engine_id='serper'").Scan(&old); err != nil {
				t.Fatal(err)
			}
			guarded, finish := configurationScope(t, svc)
			defer finish()
			ctx, cancel := context.WithCancel(guarded)
			defer cancel()
			if failure == "cancel" {
				vault.storeHook = cancel
			} else {
				if _, err := raw.Exec(`CREATE TRIGGER reject_credential BEFORE UPDATE ON search_api_keys BEGIN SELECT RAISE(ABORT,'injected SQL failure'); END`); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := store.Set(ctx, "serper", "new-fixture"); err == nil {
				t.Fatal("failed credential save reported success")
			}
			var current string
			if err := raw.QueryRow("SELECT api_key FROM search_api_keys WHERE engine_id='serper'").Scan(&current); err != nil || current != old {
				t.Fatal("previous credential reference changed")
			}
			if len(vault.values) != 1 || vault.values[old] != "old-fixture" {
				t.Fatal("failed stage deleted original secret or left new secret")
			}
		})
	}
}

func TestConfigurationSearchCredentialCleanupOutboxSurvivesFailureAndRestart(t *testing.T) {
	db, _ := configurationDB(t)
	raw, _ := db.DB()
	vault := &configurationCredentialVault{values: map[string]string{}}
	store := search.NewCredentialStore(raw).WithVault(vault)
	if _, err := store.Set(t.Context(), "serper", "old-fixture"); err != nil {
		t.Fatal(err)
	}
	vault.deleteFails = true
	result, err := store.Set(t.Context(), "serper", "new-fixture")
	if err != nil || !result.CleanupPending {
		t.Fatalf("committed write lost cleanup status: %v %+v", err, result)
	}
	var pending int
	if err := raw.QueryRow("SELECT count(*) FROM search_credential_cleanup WHERE attempts=1").Scan(&pending); err != nil || pending != 1 {
		t.Fatal("cleanup failure not durably tracked")
	}
	var active string
	if err := raw.QueryRow("SELECT api_key FROM search_api_keys WHERE engine_id='serper'").Scan(&active); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("INSERT INTO search_credential_cleanup(secret_ref,engine_id,created_at) VALUES(?,'serper',CURRENT_TIMESTAMP)", active); err != nil {
		t.Fatal(err)
	}
	vault.deleteFails = false
	restarted := search.NewCredentialStore(raw).WithVault(vault)
	if err := restarted.CleanupPendingCredentials(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(vault.values) != 1 || vault.values[active] != "new-fixture" {
		t.Fatal("cleanup deleted the active credential")
	}
	vault.deleteFails = true
	if err := restarted.Delete(t.Context(), "serper"); !errors.Is(err, search.ErrCredentialCleanupPending) {
		t.Fatalf("committed delete did not report pending cleanup: %v", err)
	}
	var count int
	if err := raw.QueryRow("SELECT count(*) FROM search_api_keys").Scan(&count); err != nil || count != 0 {
		t.Fatal("committed credential removal not visible")
	}
	vault.deleteFails = false
	if err := restarted.CleanupPendingCredentials(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(vault.values) != 0 {
		t.Fatal("queued deletion not recovered")
	}
}
