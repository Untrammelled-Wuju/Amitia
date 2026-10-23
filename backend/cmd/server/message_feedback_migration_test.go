package main

import "testing"

func TestLegacyMessageFeedbackScopeMigration(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	if err := p.services.DB.Exec(`INSERT INTO conversations(id,space_id,title,created_at,updated_at) VALUES('chat','core','chat','now','now')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := p.services.DB.Exec(`INSERT INTO messages(id,conversation_id,character_id,role,content,created_at) VALUES('reply','chat','role','assistant','answer','now')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := p.services.DB.Exec(`INSERT INTO message_feedback(message_id,feedback_type,reason,created_at) VALUES('reply','good','useful','2026-10-23T00:00:00Z')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := p.services.DB.Exec(`INSERT INTO message_feedback(message_id,feedback_type,reason,created_at) VALUES('unknown','other','legacy','2026-10-23T00:00:00Z')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateLegacyMessageFeedback(t.Context(), p.services.DB, "core", "core", "core"); err != nil {
		t.Fatal(err)
	}
	if err := migrateLegacyMessageFeedback(t.Context(), p.services.DB, "core", "core", "core"); err != nil {
		t.Fatal(err)
	}
	var scoped struct {
		SpaceID, OwnerID, CoreID, RoleID, ConversationID, RequestID string
		Revision                                                    int64
	}
	if err := p.services.DB.Table("message_feedback").Select("space_id,owner_id,core_id,role_id,conversation_id,request_id,revision").Where("message_id='reply'").Take(&scoped).Error; err != nil {
		t.Fatal(err)
	}
	if scoped.SpaceID != "core" || scoped.OwnerID != "core" || scoped.CoreID != "core" || scoped.RoleID != "role" || scoped.ConversationID != "chat" || scoped.RequestID == "" || scoped.Revision != 1 {
		t.Fatalf("scoped=%+v", scoped)
	}
	var unscoped struct {
		SpaceID, OwnerID, CoreID, RequestID string
	}
	if err := p.services.DB.Table("message_feedback").Select("space_id,owner_id,core_id,request_id").Where("message_id='unknown'").Take(&unscoped).Error; err != nil {
		t.Fatal(err)
	}
	if unscoped.SpaceID != "legacy-unscoped" || unscoped.OwnerID != "legacy-unscoped" || unscoped.CoreID != "legacy-unscoped" || unscoped.RequestID == "" {
		t.Fatalf("unscoped=%+v", unscoped)
	}
}
