package virtualdisplay

import (
	"testing"
)

func TestStore_InsertAndGet(t *testing.T) {
	s := &Store{}
	rec := &VirtualDisplayRecord{
		DisplayID:  100,
		Width:      1080,
		Height:     1920,
		DensityDPI: 420,
		State:      StateReady,
	}
	s.Insert(rec)
	got := s.Get()
	if got == nil {
		t.Fatal("expected record, got nil")
	}
	if got.DisplayID != 100 {
		t.Errorf("displayID: got %d, want 100", got.DisplayID)
	}
	if got.Ref.IsEmpty() {
		t.Error("expected ref to be set")
	}
	if got.Generation == 0 {
		t.Error("expected generation to be non-zero")
	}
}

func TestStore_HasActive(t *testing.T) {
	s := &Store{}
	if s.HasActive() {
		t.Error("expected no active display")
	}
	s.Insert(&VirtualDisplayRecord{State: StateReady})
	if !s.HasActive() {
		t.Error("expected active display")
	}
	s.Insert(&VirtualDisplayRecord{State: StateReleased})
	if s.HasActive() {
		t.Error("expected no active display after release")
	}
}

func TestStore_Update_Mismatch(t *testing.T) {
	s := &Store{}
	s.Insert(&VirtualDisplayRecord{State: StateReady})
	err := s.Update("vd_wrong", func(r *VirtualDisplayRecord) error {
		return nil
	})
	if err == nil {
		t.Error("expected error on ref mismatch")
	}
}

func TestStore_Remove(t *testing.T) {
	s := &Store{}
	s.Insert(&VirtualDisplayRecord{State: StateReady})
	got := s.Get()
	removed, err := s.Remove(got.Ref)
	if err != nil {
		t.Fatalf("remove error: %v", err)
	}
	if removed == nil {
		t.Fatal("expected removed record")
	}
	if s.HasActive() {
		t.Error("expected no active display after remove")
	}
}

func TestStore_Remove_NotFound(t *testing.T) {
	s := &Store{}
	_, err := s.Remove("vd_nonexistent")
	if err == nil {
		t.Error("expected error on remove nonexistent")
	}
}

func TestStore_BumpGeneration(t *testing.T) {
	s := &Store{}
	s.Insert(&VirtualDisplayRecord{State: StateReady})
	rec := s.Get()
	gen := rec.Generation
	err := s.BumpGeneration(rec.Ref)
	if err != nil {
		t.Fatalf("bump error: %v", err)
	}
	if s.Get().Generation != gen+1 {
		t.Errorf("expected generation %d, got %d", gen+1, s.Get().Generation)
	}
}

func TestStore_Get_Empty(t *testing.T) {
	s := &Store{}
	if s.Get() != nil {
		t.Error("expected nil for empty store")
	}
}

func TestUintToOpaque(t *testing.T) {
	cases := map[uint64]string{
		0:  "0",
		1:  "1",
		35: "z",
		36: "10",
		71: "1z",
	}
	for n, want := range cases {
		got := uintToOpaque(n)
		if got != want {
			t.Errorf("uintToOpaque(%d) = %s, want %s", n, got, want)
		}
	}
}
