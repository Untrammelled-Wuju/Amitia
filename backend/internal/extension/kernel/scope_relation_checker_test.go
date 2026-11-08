package kernel

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/glebarez/sqlite"
)

func TestRepositoryScopeRelationCheckerUsesRelationDatabase(t *testing.T) {
	kernelDB, err := sql.Open("sqlite", "file:scope-kernel?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer kernelDB.Close()
	relationDB, err := sql.Open("sqlite", "file:scope-main?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer relationDB.Close()
	if _, err := relationDB.Exec(`CREATE TABLE characters (id TEXT PRIMARY KEY); CREATE TABLE conversations (id TEXT PRIMARY KEY, character_id TEXT); INSERT INTO characters (id) VALUES ('character-1'); INSERT INTO conversations (id, character_id) VALUES ('conversation-1', 'character-1')`); err != nil {
		t.Fatal(err)
	}
	checker := newRepositoryScopeRelationChecker(kernelDB, relationDB, nil, nil)
	ctx := context.Background()
	if checker.IsCharacterDeleted(ctx, "character-1") {
		t.Fatal("character should be resolved from relation database")
	}
	if checker.IsConversationDeleted(ctx, "conversation-1") {
		t.Fatal("conversation should be resolved from relation database")
	}
	if !checker.ConversationBelongsToCharacter(ctx, "conversation-1", "character-1") {
		t.Fatal("conversation ownership should be resolved from relation database")
	}
}

func TestRepositoryScopeRelationCheckerAcceptsAuthoritativeRemoteConversation(t *testing.T) {
	kernelDB, err := sql.Open("sqlite", "file:scope-remote-kernel?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer kernelDB.Close()
	relationDB, err := sql.Open("sqlite", "file:scope-remote-main?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer relationDB.Close()
	if _, err := relationDB.Exec(`CREATE TABLE characters (id TEXT PRIMARY KEY); CREATE TABLE conversations (id TEXT PRIMARY KEY, character_id TEXT)`); err != nil {
		t.Fatal(err)
	}
	checker := newRepositoryScopeRelationChecker(kernelDB, relationDB, nil, nil)
	ctx := WithValidatedConversationAuthority(context.Background(), "remote-conversation", "space", "role", "device", "core")
	if checker.IsConversationDeleted(ctx, "remote-conversation") {
		t.Fatal("authoritatively validated remote conversation was reported as deleted")
	}
	if !checker.ConversationBelongsToCharacter(ctx, "remote-conversation", "role") {
		t.Fatal("authoritatively validated remote conversation role was rejected")
	}
	if !checker.IsConversationDeleted(ctx, "forged-conversation") {
		t.Fatal("unvalidated conversation ID bypassed existence checks")
	}
	if checker.ConversationBelongsToCharacter(ctx, "remote-conversation", "other-role") {
		t.Fatal("remote conversation authority bypassed the verified role")
	}
}
