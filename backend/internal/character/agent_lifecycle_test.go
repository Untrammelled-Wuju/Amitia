package character

import (
	"strings"
	"testing"

	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type roleLifecycleSpy struct {
	ensured  int
	archived int
	lastID   string
}

func (s *roleLifecycleSpy) EnsureRoleAgent(_ *gorm.DB, role *Character) error {
	s.ensured++
	s.lastID = role.ID
	return nil
}

func (s *roleLifecycleSpy) ArchiveRoleAgent(_ *gorm.DB, role *Character) error {
	s.archived++
	s.lastID = role.ID
	return nil
}

func TestCharacterMutationSynchronizesRoleAgent(t *testing.T) {
	db := newTestDB(t)
	ctx := app.NewAppContext(db, nil)
	service := NewService(NewRepository(ctx), ctx).(*service)
	spy := &roleLifecycleSpy{}
	service.SetAgentRoleLifecycle(spy)
	created, err := service.CreateForSpace(&CreateCharacterRequest{Name: "自动同步角色"}, "space-test")
	if err != nil {
		t.Fatal(err)
	}
	if spy.ensured != 1 || spy.lastID != created.ID {
		t.Fatalf("create did not synchronize role agent: %+v", spy)
	}
	name := "自动同步角色更新"
	if _, err := service.UpdateForSpace(created.ID, &UpdateCharacterRequest{Name: &name}, "space-test"); err != nil {
		t.Fatal(err)
	}
	if spy.ensured != 2 || spy.lastID != created.ID {
		t.Fatalf("update did not synchronize role agent: %+v", spy)
	}
	if err := service.DeleteForSpace(created.ID, "space-test"); err != nil {
		t.Fatal(err)
	}
	if spy.archived != 1 || spy.lastID != created.ID {
		t.Fatalf("delete did not archive role agent: %+v", spy)
	}
}

func TestCharacterNameBoundaryAndDefaultRoleDeletion(t *testing.T) {
	db := newTestDB(t)
	ctx := app.NewAppContext(db, nil)
	service := NewService(NewRepository(ctx), ctx).(*service)

	first, err := service.CreateForSpace(&CreateCharacterRequest{Name: "第一个角色"}, "space-test")
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateForSpace(&CreateCharacterRequest{Name: "第二个角色"}, "space-test")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteForSpace(first.ID, "space-test"); err != nil {
		t.Fatal(err)
	}
	replacement, err := service.GetByIDForSpace(second.ID, "space-test")
	if err != nil {
		t.Fatal(err)
	}
	if replacement.IsDefault != 1 {
		t.Fatal("default role was not promoted after deletion")
	}
	tooLong := strings.Repeat("角", 129)
	if _, err := service.CreateForSpace(&CreateCharacterRequest{Name: tooLong}, "space-test"); err == nil {
		t.Fatal("accepted a character name over 128 characters")
	}
	if _, err := service.UpdateForSpace(second.ID, &UpdateCharacterRequest{Name: &tooLong}, "space-test"); err == nil {
		t.Fatal("updated a character name over 128 characters")
	}
}
