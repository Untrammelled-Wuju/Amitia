package configwrite_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/system/dataportability"
	"github.com/u-ai/backend/internal/tts"
	"gorm.io/gorm"
)

type configurationBackupReader struct {
	components map[string]string
	onRead     func()
}

func (r configurationBackupReader) ReadComponent(id string) (io.ReadCloser, error) {
	if r.onRead != nil {
		r.onRead()
	}
	data, exists := r.components[id]
	if !exists {
		return nil, dataportability.ErrBackupComponentFailed
	}
	return io.NopCloser(strings.NewReader(data)), nil
}

func (configurationBackupReader) ReadJSON(string, interface{}) error {
	return errors.New("unused JSON")
}
func (r configurationBackupReader) ListComponents() []string {
	var result []string
	for id := range r.components {
		result = append(result, id)
	}
	return result
}

func privateConfigurationBackup() configurationBackupReader {
	return configurationBackupReader{components: map[string]string{
		chat.ComponentIDModelConfigs: `{"id":10,"name":"imported-model","api_type":"openai","model_name":"fixture","is_active":1}`,
		"voice.tts.v1":               `{"name":"imported-voice","apiType":"openai","voiceType":"echo","isActive":1}`,
		"voice.asr.v1":               `{"name":"imported-asr","apiType":"openai","isActive":1}`,
		"voice.clones.v1":            `{"spaceId":"space","speakerId":"imported-clone","name":"imported-clone","voiceConfigName":"imported-voice"}`,
	}}
}

func assertConfigurationBackupNotPersisted(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, table := range []string{"model_configs", "tts_configs", "asr_configs"} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil || count != 2 {
			t.Fatalf("%s gained partial config: %d %v", table, count, err)
		}
		if err := db.Table(table).Where("id=1 AND name='previous' AND is_active=1").Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("%s lost existing active: %d %v", table, count, err)
		}
	}
	var clones int64
	if err := db.Table("tts_cloned_voices").Count(&clones).Error; err != nil || clones != 0 {
		t.Fatalf("partial clone: %d %v", clones, err)
	}
}

func TestConfigurationBackupRejectsScopeRevokedWhileReading(t *testing.T) {
	for _, kind := range []string{"model", "voice"} {
		t.Run(kind, func(t *testing.T) {
			db, svc := configurationDB(t)
			ctx, finish := configurationScope(t, svc)
			defer finish()
			reader := privateConfigurationBackup()
			reader.onRead = func() {
				policy, err := svc.Get(t.Context(), "space", "admin")
				if err != nil {
					t.Fatal(err)
				}
				if policy.Administrator {
					if _, err := svc.GrantAdministrator(t.Context(), "space", "admin", policy.PermissionRevision, false); err != nil {
						t.Fatal(err)
					}
				}
			}
			var err error
			if kind == "model" {
				err = chat.NewModelConfigBackupContributor(db).RestoreModelConfigs(ctx, reader, dataportability.RestoreOptions{})
			} else {
				err = tts.NewVoiceBackupContributor(db).RestoreVoices(ctx, reader, dataportability.RestoreOptions{})
			}
			if err == nil {
				t.Fatal("revoked backup import succeeded")
			}
			assertConfigurationBackupNotPersisted(t, db)
		})
	}
}

func TestConfigurationVoiceBackupRollsBackAcrossComponents(t *testing.T) {
	for _, failure := range []string{"database", "canceled", "scanner", "invalid-json"} {
		t.Run(failure, func(t *testing.T) {
			db, svc := configurationDB(t)
			guarded, finish := configurationScope(t, svc)
			defer finish()
			ctx, cancel := context.WithCancel(guarded)
			defer cancel()
			reader := privateConfigurationBackup()
			switch failure {
			case "database":
				if err := db.Exec(`CREATE TRIGGER reject_asr BEFORE INSERT ON asr_configs BEGIN SELECT RAISE(ABORT,'injected ASR failure'); END`).Error; err != nil {
					t.Fatal(err)
				}
			case "canceled":
				if err := db.Callback().Create().After("gorm:create").Register("test:cancel-backup", func(tx *gorm.DB) {
					if tx.Statement.Table == "asr_configs" {
						cancel()
					}
				}); err != nil {
					t.Fatal(err)
				}
			case "scanner":
				reader.components["voice.asr.v1"] = `{"name":"` + strings.Repeat("x", 70<<10) + `"}`
			case "invalid-json":
				reader.components["voice.asr.v1"] = `{`
			}
			if err := tts.NewVoiceBackupContributor(db).RestoreVoices(ctx, reader, dataportability.RestoreOptions{}); err == nil {
				t.Fatal("failed component reported successful import")
			}
			assertConfigurationBackupNotPersisted(t, db)
		})
	}
}

func TestConfigurationModelBackupRollsBackWholeComponent(t *testing.T) {
	db, svc := configurationDB(t)
	ctx, finish := configurationScope(t, svc)
	defer finish()
	if err := db.Exec(`CREATE TRIGGER reject_model BEFORE INSERT ON model_configs WHEN NEW.id=11 BEGIN SELECT RAISE(ABORT,'injected model failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	reader := privateConfigurationBackup()
	reader.components[chat.ComponentIDModelConfigs] += "\n" + `{"id":11,"name":"failed-model"}`
	if err := chat.NewModelConfigBackupContributor(db).RestoreModelConfigs(ctx, reader, dataportability.RestoreOptions{}); err == nil {
		t.Fatal("failed model import reported success")
	}
	assertConfigurationBackupNotPersisted(t, db)
}

func TestConfigurationBackupSuccessPreservesActiveAndCloneBinding(t *testing.T) {
	db, svc := configurationDB(t)
	ctx, finish := configurationScope(t, svc)
	defer finish()
	reader := privateConfigurationBackup()
	if err := chat.NewModelConfigBackupContributor(db).RestoreModelConfigs(ctx, reader, dataportability.RestoreOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := tts.NewVoiceBackupContributor(db).RestoreVoices(ctx, reader, dataportability.RestoreOptions{}); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"model_configs", "tts_configs", "asr_configs"} {
		var count int64
		if err := db.Table(table).Where("is_active=1 AND id=1").Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("%s replaced active: %d %v", table, count, err)
		}
		if err := db.Table(table).Where("id>2 AND is_active=0").Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("%s import missing/inappropriately active: %d %v", table, count, err)
		}
	}
	var binding struct {
		TtsConfigID int
		SpaceID     string
	}
	if err := db.Table("tts_cloned_voices").Where("speaker_id='imported-clone'").First(&binding).Error; err != nil || binding.TtsConfigID != 3 || binding.SpaceID != "space" {
		t.Fatalf("clone binding: %+v %v", binding, err)
	}
}
