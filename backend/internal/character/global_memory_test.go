package character

import (
	"github.com/u-ai/backend/pkg/app"
	"testing"
)

func TestGlobalMemoryCharacterDefaultAndFalsePersist(t *testing.T) {
	db := newTestDB(t)
	ctx := app.NewAppContext(db, nil)
	service := NewService(NewRepository(ctx), ctx)
	global, err := service.Create(&CreateCharacterRequest{Name: "全局角色"})
	if err != nil {
		t.Fatal(err)
	}
	if !global.GlobalMemoryEnabled {
		t.Fatal("new roles must default to global memory")
	}
	off := false
	private, err := service.Create(&CreateCharacterRequest{Name: "私有角色", GlobalMemoryEnabled: &off})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := service.GetByID(private.ID)
	if err != nil || saved.GlobalMemoryEnabled {
		t.Fatalf("private setting lost: %+v %v", saved, err)
	}
	on := true
	saved, err = service.Update(private.ID, &UpdateCharacterRequest{GlobalMemoryEnabled: &on})
	if err != nil || !saved.GlobalMemoryEnabled {
		t.Fatalf("toggle on failed: %+v %v", saved, err)
	}
	saved, err = service.Update(private.ID, &UpdateCharacterRequest{GlobalMemoryEnabled: &off})
	if err != nil || saved.GlobalMemoryEnabled {
		t.Fatalf("toggle off failed: %+v %v", saved, err)
	}
	profile, err := NewRepository(ctx).GetRuntimeProfile(private.ID)
	if err != nil || profile.GlobalMemoryEnabled {
		t.Fatalf("runtime policy lost: %+v %v", profile, err)
	}
}
