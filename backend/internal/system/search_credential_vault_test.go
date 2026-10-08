package system

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/secret"
	"github.com/u-ai/backend/internal/search"
)

type trackedPreparedSearchVault struct {
	*searchCredentialVault
	db         *sql.DB
	registered bool
	cancel     context.CancelFunc
}

func (v *trackedPreparedSearchVault) StoreReference(ctx context.Context, ref string, value []byte) error {
	var count int
	if err := v.db.QueryRow("SELECT count(*) FROM search_credential_cleanup WHERE secret_ref=?", ref).Scan(&count); err != nil {
		return err
	}
	v.registered = count == 1
	if !v.registered {
		return errors.New("secret written before durable stage registration")
	}
	if v.cancel != nil {
		v.cancel()
	}
	return v.searchCredentialVault.StoreReference(ctx, ref, value)
}

func TestPreparedSearchCredentialActualAdapterRegistersBeforeEncryptedWrite(t *testing.T) {
	svc := newSetupServiceTest(t)
	for _, statement := range []string{
		`CREATE TABLE search_api_keys(engine_id TEXT PRIMARY KEY,api_key TEXT NOT NULL,updated_at DATETIME NOT NULL)`,
		`CREATE TABLE search_credential_cleanup(secret_ref TEXT PRIMARY KEY,engine_id TEXT NOT NULL,attempts INTEGER NOT NULL DEFAULT 0,last_error TEXT NOT NULL DEFAULT '',created_at DATETIME NOT NULL)`,
	} {
		if err := svc.db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	raw, err := svc.db.DB()
	if err != nil {
		t.Fatal(err)
	}
	raw.SetMaxOpenConns(1)
	dir := t.TempDir()
	encrypted, err := secret.NewEncryptedFileStore(filepath.Join(dir, "vault.json"), filepath.Join(dir, "vault.key"))
	if err != nil {
		t.Fatal(err)
	}
	broker, err := secret.NewBroker(secret.BrokerConfig{Store: encrypted})
	if err != nil {
		t.Fatal(err)
	}
	adapter := &trackedPreparedSearchVault{searchCredentialVault: newSearchCredentialVault(broker), db: raw}
	store := search.NewCredentialStore(raw).WithVault(adapter)
	if _, err := store.Set(t.Context(), "serper", "actual-fixture"); err != nil || !adapter.registered {
		t.Fatal("actual prepared save failed", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	adapter.cancel = cancel
	if _, err := store.Set(ctx, "serper", "canceled-fixture"); err == nil {
		t.Fatal("canceled stage was committed")
	}
	adapter.cancel = nil
	var active string
	if err := raw.QueryRow("SELECT api_key FROM search_api_keys WHERE engine_id='serper'").Scan(&active); err != nil {
		t.Fatal(err)
	}
	value, err := adapter.Resolve(t.Context(), active)
	if err != nil || string(value) != "actual-fixture" {
		t.Fatal("old secret damaged", err)
	}
	reserved, err := adapter.ReserveReference(t.Context(), "search/serper")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("INSERT INTO search_credential_cleanup(secret_ref,engine_id,created_at) VALUES(?,'serper',CURRENT_TIMESTAMP)", reserved); err != nil {
		t.Fatal(err)
	}
	if err := adapter.StoreReference(t.Context(), reserved, []byte("crash-before-sql-fixture")); err != nil {
		t.Fatal(err)
	}
	restarted := search.NewCredentialStore(raw).WithVault(newSearchCredentialVault(broker))
	if err := restarted.CleanupPendingCredentials(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := encrypted.Get(t.Context(), reserved); !errors.Is(err, secret.ErrSecretNotFound) {
		t.Fatal("pre-commit crash stage not recovered", err)
	}
	if _, err := encrypted.Get(t.Context(), active); err != nil {
		t.Fatal("cleanup deleted active secret", err)
	}
}
