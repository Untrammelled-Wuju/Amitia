package chat

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/profile"
	"gorm.io/gorm"
)

type captureProfileService struct {
	spaceID   string
	convID    string
	charID    string
	messages  []map[string]string
	callCount int
}

func (s *captureProfileService) List(q profile.ProfileListQuery) (*profile.ProfileListResponse, error) {
	return nil, nil
}

func (s *captureProfileService) Create(req *profile.CreateProfileRequest) (*profile.UserProfile, error) {
	return nil, nil
}

func (s *captureProfileService) Update(id string, req *profile.UpdateProfileRequest) (*profile.UserProfile, error) {
	return nil, nil
}

func (s *captureProfileService) Delete(id string) error {
	return nil
}

func (s *captureProfileService) GetBySpaceID(spaceID string, characterID ...string) ([]profile.UserProfile, error) {
	return nil, nil
}

func (s *captureProfileService) ExtractFromConversation(spaceID, convID string, messages []map[string]string, characterID ...string) error {
	s.callCount++
	s.spaceID = spaceID
	s.convID = convID
	s.messages = messages
	if len(characterID) > 0 {
		s.charID = characterID[0]
	}
	return nil
}

func (s *captureProfileService) ToSystemPrompt(spaceID string, characterID ...string) string {
	return ""
}

func (s *captureProfileService) UpsertFromTool(spaceID, category, attrName, attrValue string, confidence int, convID string, characterID ...string) (*profile.UserProfile, error) {
	return nil, nil
}

func (s *captureProfileService) SyncGraphProfile(id string) bool {
	return false
}

func (s *captureProfileService) Name() string {
	return "capture"
}

func (s *captureProfileService) Process(ctx context.Context, convID string, messages []map[string]string, newReply string) error {
	return nil
}

func setupMemoryIntegrationService(t *testing.T, profSvc profile.Service) *service {
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
	if err := db.AutoMigrate(&Conversation{}, &Message{}); err != nil {
		t.Fatal(err)
	}
	return &service{db: db, profilePort: profSvc}
}

func TestExtractProfileUsesConversationPeerID(t *testing.T) {
	profSvc := &captureProfileService{}
	svc := setupMemoryIntegrationService(t, profSvc)
	if err := svc.db.Create(&Conversation{ID: "conv-1", SpaceID: normalizeConversationOwner(""), CharacterID: "char-1", PeerID: "user-1"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Create(&Message{ID: "msg-1", ConversationID: "conv-1", Role: "user", Content: "我喜欢咖啡", IncludeInCtx: 1}).Error; err != nil {
		t.Fatal(err)
	}

	svc.extractProfile("conv-1", "char-1")

	if profSvc.callCount != 1 {
		t.Fatalf("ExtractFromConversation call count = %d, want 1", profSvc.callCount)
	}
	if profSvc.spaceID != "user-1" {
		t.Fatalf("profile spaceID = %q, want user-1", profSvc.spaceID)
	}
	if profSvc.charID != "char-1" {
		t.Fatalf("profile characterID = %q, want char-1", profSvc.charID)
	}
	if len(profSvc.messages) != 1 || profSvc.messages[0]["content"] != "我喜欢咖啡" {
		t.Fatalf("unexpected messages: %#v", profSvc.messages)
	}
}

func TestExtractProfileFallsBackToCharacterIDWithoutPeerID(t *testing.T) {
	profSvc := &captureProfileService{}
	svc := setupMemoryIntegrationService(t, profSvc)
	if err := svc.db.Create(&Conversation{ID: "conv-1", SpaceID: normalizeConversationOwner(""), CharacterID: "char-1"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Create(&Message{ID: "msg-1", ConversationID: "conv-1", Role: "user", Content: "我喜欢茶", IncludeInCtx: 1}).Error; err != nil {
		t.Fatal(err)
	}

	svc.extractProfile("conv-1", "char-1")

	if profSvc.spaceID != "char-1" {
		t.Fatalf("profile spaceID = %q, want char-1", profSvc.spaceID)
	}
}
