package chat

import (
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEnrichConversationActivityUsesActiveAndTerminalTurns(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "activity.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := db.AutoMigrate(&Conversation{}, &AssistantTurn{}); err != nil {
		t.Fatal(err)
	}
	conversations := []Conversation{
		{ID: "running", Title: "Running"},
		{ID: "terminal", Title: "Terminal"},
	}
	if err := db.Create(&conversations).Error; err != nil {
		t.Fatal(err)
	}
	turns := []AssistantTurn{
		{ID: "turn-1", ConversationID: "running", Sequence: 1, Status: assistantTurnStatusRunning},
		{ID: "turn-2", ConversationID: "running", Sequence: 2, Status: assistantTurnStatusCompleted},
		{ID: "turn-3", ConversationID: "terminal", Sequence: 1, Status: assistantTurnStatusCompleted},
	}
	if err := db.Create(&turns).Error; err != nil {
		t.Fatal(err)
	}
	if err := EnrichConversationActivity(db, conversations); err != nil {
		t.Fatal(err)
	}
	if !conversations[0].IsGenerating || !conversations[0].HasUnread || conversations[0].LastTerminalTurnSequence != 2 {
		t.Fatalf("running conversation activity = %+v", conversations[0])
	}
	if conversations[1].IsGenerating || !conversations[1].HasUnread || conversations[1].LastTerminalTurnSequence != 1 {
		t.Fatalf("terminal conversation activity = %+v", conversations[1])
	}
	conversations[1].LastReadTurnSequence = 1
	if err := EnrichConversationActivity(db, conversations); err != nil {
		t.Fatal(err)
	}
	if conversations[1].HasUnread {
		t.Fatalf("read conversation remained unread: %+v", conversations[1])
	}
}
