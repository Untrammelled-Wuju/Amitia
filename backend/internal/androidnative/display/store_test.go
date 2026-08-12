package display

import (
	"sync"
	"testing"

	"github.com/u-ai/backend/internal/androidnative/virtualdisplay"
)

func TestStore_EmptySnapshot(t *testing.T) {
	classifier := NewDisplayClassifier()
	store := NewDisplayStore(classifier)
	snapshot := store.Snapshot()
	if snapshot.Generation != 0 {
		t.Errorf("expected generation 0, got %d", snapshot.Generation)
	}
	if len(snapshot.Displays) != 0 {
		t.Errorf("expected 0 displays, got %d", len(snapshot.Displays))
	}
}

func TestStore_Put_NewDisplay(t *testing.T) {
	classifier := NewDisplayClassifier()
	store := NewDisplayStore(classifier)
	info := DisplayInfo{
		DisplayID:  0,
		Generation: 1,
		IsDefault:  true,
		Width:      1080,
		Height:     2400,
		DensityDPI: 420,
		Rotation:   0,
		State:      string(DisplayStateOn),
		IsValid:    true,
		Name:       "Built-in Display",
	}

	isNew, prevGen := store.Put(info)
	if !isNew {
		t.Error("expected new display")
	}
	if prevGen != 0 {
		t.Errorf("expected prevGen 0, got %d", prevGen)
	}
	if store.Count() != 1 {
		t.Errorf("expected count 1, got %d", store.Count())
	}
	if store.GlobalGeneration() == 0 {
		t.Error("expected global generation to be bumped")
	}
}

func TestStore_Put_UpdateGenerationOnSizeChange(t *testing.T) {
	classifier := NewDisplayClassifier()
	store := NewDisplayStore(classifier)

	store.Put(DisplayInfo{DisplayID: 0, Generation: 1, IsDefault: true, Width: 1080, Height: 2400, DensityDPI: 420, State: string(DisplayStateOn), IsValid: true, Name: "Default"})
	prevGlobal := store.GlobalGeneration()

	isNew, prevGen := store.Put(DisplayInfo{DisplayID: 0, Generation: 2, IsDefault: true, Width: 1440, Height: 3200, DensityDPI: 420, State: string(DisplayStateOn), IsValid: true, Name: "Default"})
	if isNew {
		t.Error("expected update, not new")
	}
	if prevGen != 1 {
		t.Errorf("expected prevGen 1, got %d", prevGen)
	}
	if store.GlobalGeneration() <= prevGlobal {
		t.Errorf("global generation should have increased beyond %d, got %d", prevGlobal, store.GlobalGeneration())
	}
}

func TestStore_Remove(t *testing.T) {
	classifier := NewDisplayClassifier()
	store := NewDisplayStore(classifier)
	store.Put(DisplayInfo{DisplayID: 1, Generation: 1, Width: 1920, Height: 1080, DensityDPI: 160, State: string(DisplayStateOn), IsValid: true, Name: "External"})

	rec, ok := store.Remove(1)
	if !ok {
		t.Fatal("expected display removed")
	}
	if rec.Info.DisplayID != 1 {
		t.Errorf("expected display id 1, got %d", rec.Info.DisplayID)
	}
	if store.Count() != 0 {
		t.Errorf("expected count 0 after removal, got %d", store.Count())
	}
}

func TestStore_Remove_NotFound(t *testing.T) {
	classifier := NewDisplayClassifier()
	store := NewDisplayStore(classifier)
	_, ok := store.Remove(99)
	if ok {
		t.Error("expected false for non-existent display")
	}
}

func TestStore_GetAll(t *testing.T) {
	classifier := NewDisplayClassifier()
	store := NewDisplayStore(classifier)
	store.Put(DisplayInfo{DisplayID: 0, Generation: 1, IsDefault: true})
	store.Put(DisplayInfo{DisplayID: 1, Generation: 1, Name: "External"})
	store.Put(DisplayInfo{DisplayID: 2, Generation: 1, Presentation: true})

	all := store.GetAll()
	if len(all) != 3 {
		t.Errorf("expected 3 displays, got %d", len(all))
	}
	if _, ok := all[0]; !ok {
		t.Error("expected display 0")
	}
	if _, ok := all[1]; !ok {
		t.Error("expected display 1")
	}
	if _, ok := all[2]; !ok {
		t.Error("expected display 2")
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	classifier := NewDisplayClassifier()
	store := NewDisplayStore(classifier)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			store.Put(DisplayInfo{DisplayID: id, Generation: 1, Width: 1080, Height: 2400})
		}(i)
	}
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			store.Remove(id)
		}(i)
	}
	wg.Wait()
	if store.Count() > 50 {
		t.Errorf("expected <= 50 displays, got %d", store.Count())
	}
}

func TestStore_SetManagedVirtual(t *testing.T) {
	classifier := NewDisplayClassifier()
	store := NewDisplayStore(classifier)
	ref := virtualdisplay.VirtualDisplayRef("vd_amitia_test001")
	store.Put(DisplayInfo{DisplayID: 4, Generation: 1, Width: 1080, Height: 1920, IsValid: true})

	store.SetManagedVirtual(4, &ref)
	rec, ok := store.Get(4)
	if !ok {
		t.Fatal("expected display 4 to exist")
	}
	if !rec.Info.ManagedByAmitia {
		t.Error("expected ManagedByAmitia=true")
	}
	if rec.Info.Type != string(DisplayTypeVirtualAmitia) {
		t.Errorf("expected type virtual_amitia, got %s", rec.Info.Type)
	}
}

func TestStore_RemoveManagedVirtual(t *testing.T) {
	classifier := NewDisplayClassifier()
	store := NewDisplayStore(classifier)
	ref := virtualdisplay.VirtualDisplayRef("vd_amitia_test001")
	store.Put(DisplayInfo{DisplayID: 4, Generation: 1, Width: 1080, Height: 1920, IsValid: true})
	store.SetManagedVirtual(4, &ref)
	store.RemoveManagedVirtual(4)

	rec, ok := store.Get(4)
	if !ok {
		t.Fatal("expected display 4 to exist")
	}
	if rec.Info.ManagedByAmitia {
		t.Error("expected ManagedByAmitia=false after removal")
	}
}
