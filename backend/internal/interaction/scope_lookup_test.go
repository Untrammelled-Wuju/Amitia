package interaction

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newScopeLookupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
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
	if err := db.Exec(`CREATE TABLE conversations (
		id TEXT PRIMARY KEY,
		character_id TEXT DEFAULT '',
		channel TEXT DEFAULT 'web',
		source TEXT DEFAULT 'manual',
		peer_id TEXT DEFAULT ''
	)`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func TestConversationScopeBindingLookupFindsPeerBinding(t *testing.T) {
	db := newScopeLookupTestDB(t)
	if err := db.Exec("INSERT INTO conversations (id, character_id, channel, source, peer_id) VALUES (?, ?, ?, ?, ?)", "conv-1", "char-1", "QQ", "qq", "peer-1").Error; err != nil {
		t.Fatal(err)
	}
	lookup := NewConversationScopeBindingLookup(db)

	bindings, err := lookup.FindScopeBindings(context.Background(), "qq", "peer-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 1 {
		t.Fatalf("expected one binding, got %#v", bindings)
	}
	binding := bindings[0]
	if binding.ID != "conv-1" || binding.ConversationID != "conv-1" || binding.CharacterID != "char-1" || binding.State != ScopeBindingStateActive {
		t.Fatalf("unexpected binding: %#v", binding)
	}
}

func TestConversationScopeBindingLookupFiltersByChannelAndPeer(t *testing.T) {
	db := newScopeLookupTestDB(t)
	rows := []struct {
		id      string
		channel string
		peerID  string
	}{
		{"conv-1", "qq", "peer-1"},
		{"conv-2", "wechat", "peer-1"},
		{"conv-3", "qq", "peer-2"},
	}
	for _, row := range rows {
		if err := db.Exec("INSERT INTO conversations (id, character_id, channel, source, peer_id) VALUES (?, ?, ?, ?, ?)", row.id, "char-1", row.channel, row.channel, row.peerID).Error; err != nil {
			t.Fatal(err)
		}
	}
	lookup := NewConversationScopeBindingLookup(db)

	bindings, err := lookup.FindScopeBindings(context.Background(), "qq", "peer-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 1 || bindings[0].ID != "conv-1" {
		t.Fatalf("unexpected bindings: %#v", bindings)
	}
}

func TestConversationScopeBindingLookupReturnsDuplicatesForResolver(t *testing.T) {
	db := newScopeLookupTestDB(t)
	if err := db.Exec("INSERT INTO conversations (id, character_id, channel, source, peer_id) VALUES (?, ?, ?, ?, ?)", "conv-1", "char-1", "qq", "qq", "peer-1").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO conversations (id, character_id, channel, source, peer_id) VALUES (?, ?, ?, ?, ?)", "conv-2", "char-2", "qq", "qq", "peer-1").Error; err != nil {
		t.Fatal(err)
	}
	lookup := NewConversationScopeBindingLookup(db)

	bindings, err := lookup.FindScopeBindings(context.Background(), "qq", "peer-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 2 {
		t.Fatalf("expected duplicate bindings for resolver, got %#v", bindings)
	}
}
