package chat

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/profile"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func TestLoadHistoryExcludingCurrentMessage(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB.Close()
	})
	if err := db.Exec(`CREATE TABLE messages (
id TEXT PRIMARY KEY,
conversation_id TEXT,
sequence INTEGER NOT NULL DEFAULT 0,
role TEXT,
content TEXT,
msg_type TEXT DEFAULT 'text',
tokens INTEGER DEFAULT 0,
source TEXT DEFAULT 'manual',
safety_level TEXT DEFAULT 'normal',
status TEXT DEFAULT 'sent',
include_in_context INTEGER DEFAULT 1,
audio_url TEXT DEFAULT '',
audio_duration REAL DEFAULT 0,
image_url TEXT DEFAULT '',
video_url TEXT DEFAULT '',
request_id TEXT DEFAULT '',
created_at TEXT DEFAULT '',
updated_at TEXT DEFAULT ''
)`).Error; err != nil {
		t.Fatal(err)
	}
	inserts := []struct {
		id        string
		sequence  int
		role      string
		content   string
		createdAt string
	}{
		{"m1", 1, "user", "重复内容", "2026-07-01 09:00:00"},
		{"m2", 2, "assistant", "收到", "2026-07-01 09:00:01"},
		{"m3", 3, "user", "重复内容", "2026-07-01 09:00:02"},
	}
	for _, row := range inserts {
		if err := db.Exec("INSERT INTO messages (id, conversation_id, sequence, role, content, include_in_context, created_at) VALUES (?, 'c1', ?, ?, ?, 1, ?)", row.id, row.sequence, row.role, row.content, row.createdAt).Error; err != nil {
			t.Fatal(err)
		}
	}
	svc := &service{db: db}
	history := svc.loadHistoryExcluding("c1", "m3")
	if len(history) != 2 {
		t.Fatalf("expected 2 history messages, got %d", len(history))
	}
	if history[0]["content"] != "重复内容" {
		t.Fatalf("expected previous duplicate content to remain, got %q", history[0]["content"])
	}
	if history[0]["role"] != "user" || history[1]["role"] != "assistant" {
		t.Fatalf("unexpected history roles: %#v", history)
	}
}

func TestFindRequestMessages(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB.Close()
	})
	if err := db.Exec(`CREATE TABLE messages (
id TEXT PRIMARY KEY,
conversation_id TEXT,
sequence INTEGER NOT NULL DEFAULT 0,
role TEXT,
content TEXT,
msg_type TEXT DEFAULT 'text',
tokens INTEGER DEFAULT 0,
source TEXT DEFAULT 'manual',
safety_level TEXT DEFAULT 'normal',
status TEXT DEFAULT 'sent',
include_in_context INTEGER DEFAULT 1,
audio_url TEXT DEFAULT '',
audio_duration REAL DEFAULT 0,
image_url TEXT DEFAULT '',
video_url TEXT DEFAULT '',
request_id TEXT DEFAULT '',
created_at TEXT DEFAULT '',
updated_at TEXT DEFAULT ''
)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO messages (id, conversation_id, sequence, role, content, request_id, created_at) VALUES ('u1', 'c1', 1, 'user', '你好', 'req-1', '2026-07-01 09:00:00')").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO messages (id, conversation_id, sequence, role, content, request_id, created_at) VALUES ('a1', 'c1', 2, 'assistant', '第一段', 'req-1', '2026-07-01 09:00:01')").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO messages (id, conversation_id, sequence, role, content, request_id, created_at) VALUES ('a2', 'c1', 3, 'assistant', '第二段', 'req-1', '2026-07-01 09:00:02')").Error; err != nil {
		t.Fatal(err)
	}
	svc := &service{db: db}
	user, assistants, ok := svc.findRequestMessages("c1", "req-1")
	if !ok {
		t.Fatal("expected user message")
	}
	if user.ID != "u1" {
		t.Fatalf("unexpected user message: %#v", user)
	}
	if len(assistants) != 2 || assistants[0].ID != "a1" || assistants[1].ID != "a2" {
		t.Fatalf("unexpected assistant messages: %#v", assistants)
	}
}

func TestBuildRoleSystemPartsUsesRuntimeProfile(t *testing.T) {
	parts := buildRoleSystemParts(&character.RoleRuntimeProfile{
		CharacterID:       "char-1",
		Name:              "Amitia",
		Identity:          "心理模拟伙伴",
		Personality:       "温和、敏锐",
		SpeakingStyle:     "短句回应",
		RelationshipStyle: "稳定陪伴",
		SystemPrompt:      "保持一致",
		BoundaryRules:     "不越界",
		PersonalityConfig: map[string]interface{}{"version": "runtime-profile-v1", "openness": 72},
		ChatStyleConfig:   map[string]interface{}{"version": "runtime-profile-v1", "pace": "slow"},
	}, nil)
	joined := strings.Join(parts, "\n")
	for _, want := range []string{"你是Amitia，心理模拟伙伴。", "【角色性格】", "温和、敏锐", "【聊天风格】", "短句回应", "【关系风格】", "稳定陪伴", "【场景规则】", "不越界", "openness", "pace"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected runtime prompt to contain %q, got %s", want, joined)
		}
	}
	if strings.Contains(joined, "【场景配置】") {
		t.Fatalf("default-only scene rules should not be appended: %s", joined)
	}
}

func TestSys1BuilderUsesCharacterScopedProfilePrompt(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB.Close()
	})
	if err := db.AutoMigrate(&profile.UserProfile{}); err != nil {
		t.Fatal(err)
	}
	rows := []profile.UserProfile{
		{ID: "global-1", UserID: "char-1", CharacterID: "", Category: "preference", AttributeName: "饮品", AttributeValue: "茶", Confidence: 80},
		{ID: "char-1", UserID: "char-1", CharacterID: "char-1", Category: "preference", AttributeName: "称呼", AttributeValue: "小安", Confidence: 90},
		{ID: "char-2", UserID: "char-1", CharacterID: "char-2", Category: "preference", AttributeName: "称呼", AttributeValue: "小北", Confidence: 95},
	}
	for _, row := range rows {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	ctx := app.NewAppContext(db, nil)
	svc := &service{
		db:         db,
		profileSvc: profile.NewService(profile.NewRepository(ctx), ctx, nil),
	}
	parts := svc.sys1Builder(&character.RoleRuntimeProfile{
		CharacterID: "char-1",
		Name:        "Amitia",
		Identity:    "心理模拟伙伴",
	}, "你好", nil)
	joined := strings.Join(parts, "\n")
	if !strings.Contains(joined, "小安") {
		t.Fatalf("expected scoped profile prompt to include char-1 profile, got %s", joined)
	}
	if !strings.Contains(joined, "茶") {
		t.Fatalf("expected scoped profile prompt to retain global fallback, got %s", joined)
	}
	if strings.Contains(joined, "小北") {
		t.Fatalf("expected scoped profile prompt to exclude other character profile, got %s", joined)
	}
}
