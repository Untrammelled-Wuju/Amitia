package configwrite_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/u-ai/backend/internal/embedding_config"
	"github.com/u-ai/backend/internal/system"
	"github.com/u-ai/backend/internal/system/dataportability"
)

func configurationSupplementaryBackup(kind string) configurationBackupReader {
	if kind == "embedding" {
		return configurationBackupReader{components: map[string]string{embedding_config.ComponentIDEmbeddingConfigs + ".v1": `{"id":10,"name":"imported-embedding","apiType":"openai","isActive":0}` + "\n" + `{"id":11,"name":"failed-embedding","apiType":"openai","isActive":0}`}}
	}
	return configurationBackupReader{components: map[string]string{system.ComponentIDSettings: `{"key":"seed","value":"changed"}` + "\n" + `{"key":"zzz","value":"new"}`}}
}

func TestConfigurationEmbeddingAndSettingsBackupRejectAdministratorABA(t *testing.T) {
	for _, kind := range []string{"embedding", "settings"} {
		t.Run(kind, func(t *testing.T) {
			db, svc := configurationDB(t)
			ctx, finish := configurationScope(t, svc)
			defer finish()
			reader := configurationSupplementaryBackup(kind)
			reader.onRead = func() {
				policy, err := svc.Get(t.Context(), "space", "admin")
				if err != nil {
					t.Fatal(err)
				}
				policy, err = svc.GrantAdministrator(t.Context(), "space", "admin", policy.PermissionRevision, false)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := svc.GrantAdministrator(t.Context(), "space", "admin", policy.PermissionRevision, true); err != nil {
					t.Fatal(err)
				}
			}
			var err error
			if kind == "embedding" {
				err = embedding_config.NewEmbeddingBackupContributor(db).RestoreEmbeddings(ctx, reader, dataportability.RestoreOptions{})
			} else {
				err = system.NewSettingsBackupContributor(db).RestoreSettings(ctx, reader, dataportability.RestoreOptions{})
			}
			if err == nil {
				t.Fatal("backup accepted old authority after administrator ABA")
			}
			var count int64
			if err := db.Table("embedding_configs").Count(&count).Error; err != nil || count != 2 {
				t.Fatal("stale embedding backup persisted")
			}
			if err := db.Table("app_settings").Count(&count).Error; err != nil || count != 0 {
				t.Fatal("stale settings backup persisted")
			}
		})
	}
}

func TestConfigurationEmbeddingAndSettingsBackupRollBackOnFailure(t *testing.T) {
	for _, kind := range []string{"embedding", "settings"} {
		t.Run(kind, func(t *testing.T) {
			db, svc := configurationDB(t)
			ctx, finish := configurationScope(t, svc)
			defer finish()
			if err := db.Exec(`INSERT INTO app_settings(key,value,revision) VALUES('seed','previous',5)`).Error; err != nil {
				t.Fatal(err)
			}
			var statement string
			if kind == "embedding" {
				statement = `CREATE TRIGGER reject_embedding BEFORE INSERT ON embedding_configs WHEN NEW.name='failed-embedding' BEGIN SELECT RAISE(ABORT,'injected embedding failure'); END`
			} else {
				statement = `CREATE TRIGGER reject_portable_setting BEFORE INSERT ON app_settings WHEN NEW.key='zzz' BEGIN SELECT RAISE(ABORT,'injected settings failure'); END`
			}
			if err := db.Exec(statement).Error; err != nil {
				t.Fatal(err)
			}
			reader := configurationSupplementaryBackup(kind)
			var err error
			if kind == "embedding" {
				err = embedding_config.NewEmbeddingBackupContributor(db).RestoreEmbeddings(ctx, reader, dataportability.RestoreOptions{})
			} else {
				err = system.NewSettingsBackupContributor(db).RestoreSettings(ctx, reader, dataportability.RestoreOptions{})
			}
			if err == nil {
				t.Fatal("partial backup failure was swallowed")
			}
			var count int64
			if err := db.Table("embedding_configs").Count(&count).Error; err != nil || count != 2 {
				t.Fatal("partial embedding survived rollback")
			}
			if err := db.Table("app_settings").Count(&count).Error; err != nil || count != 1 {
				t.Fatal("partial setting survived rollback")
			}
			if err := db.Table("app_settings").Where("key='seed' AND value='previous' AND revision=5").Count(&count).Error; err != nil || count != 1 {
				t.Fatal("failed settings backup changed original value/revision")
			}
		})
	}
}

func TestConfigurationCoreSwitchRejectsPreviousConfigurationRequests(t *testing.T) {
	db, svc := configurationDB(t)
	ctx, finish := configurationScope(t, svc)
	defer finish()
	if _, changed, err := svc.BindProvider(t.Context(), "core-c"); err != nil || !changed {
		t.Fatalf("provider switch: %v %v", changed, err)
	}
	if context.Cause(ctx) == nil {
		t.Fatal("provider switch did not cancel old configuration request")
	}
	router := configurationProviderRouter(db)
	for _, path := range []string{"/embedding/2/activate", "/vision/2/activate", "/imagegen/2/activate", "/config/import"} {
		if code := configurationHTTPResult(t, router, ctx, http.MethodPost, path, `{"settings":{"late":"forbidden"}}`); code == 200 || code == 0 {
			t.Fatalf("old Core accepted configuration write: %s", path)
		}
	}
	var count int64
	if err := db.Table("app_settings").Count(&count).Error; err != nil || count != 0 {
		t.Fatal("Core switch saved late settings")
	}
}
