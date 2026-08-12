package uitree

import (
	"testing"
)

func TestGenerateSnapshotID(t *testing.T) {
	id1 := GenerateSnapshotID(1, 100)
	id2 := GenerateSnapshotID(1, 101)
	id3 := GenerateSnapshotID(1, 100)

	if id1 == id2 {
		t.Fatal("different counters should produce different IDs")
	}
	if id1 != id3 {
		t.Fatal("same generation and counter should produce same ID")
	}
}

func TestGenerateNodeID(t *testing.T) {
	id1 := GenerateNodeID("uis_1", "win_1", "src1", "res1", "android.widget.Button", Rect{0, 0, 100, 50}, 2)
	id2 := GenerateNodeID("uis_1", "win_1", "src1", "res1", "android.widget.Button", Rect{0, 0, 100, 50}, 2)
	id3 := GenerateNodeID("uis_1", "win_1", "src2", "res1", "android.widget.Button", Rect{0, 0, 100, 50}, 2)
	id4 := GenerateNodeID("uis_1", "win_2", "src1", "res1", "android.widget.Button", Rect{0, 0, 100, 50}, 2)

	if id1 != id2 {
		t.Fatal("same inputs should produce same ID")
	}
	if id1 == id3 {
		t.Fatal("different sourceRef should produce different ID")
	}
	if id1 == id4 {
		t.Fatal("different windowID should produce different ID")
	}
}

func TestGenerateWindowID(t *testing.T) {
	id1 := GenerateWindowID("src1", WindowTypeApplication, "com.example.app")
	id2 := GenerateWindowID("src1", WindowTypeApplication, "com.example.app")
	id3 := GenerateWindowID("src1", WindowTypeSystem, "com.example.app")

	if id1 != id2 {
		t.Fatal("same inputs should produce same ID")
	}
	if id1 == id3 {
		t.Fatal("different window types should produce different ID")
	}
}

func TestNodeIDDeterminism(t *testing.T) {
	id1 := GenerateNodeID("uis_test", "win_test", "src_test_1", "res_test", "class_test", Rect{10, 20, 110, 70}, 3)
	id2 := GenerateNodeID("uis_test", "win_test", "src_test_1", "res_test", "class_test", Rect{10, 20, 110, 70}, 3)
	if id1 != id2 {
		t.Fatalf("same inputs should produce same ID: %s vs %s", id1, id2)
	}
}

func TestNodeIDUniqueness(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		srcRef := "src_test_" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		id := GenerateNodeID("uis_test", "win_test", srcRef, "res_test", "class_test", Rect{10, 20, 110, 70}, 3)
		if ids[id] {
			t.Fatalf("duplicate node ID generated: %s for srcRef=%s", id, srcRef)
		}
		ids[id] = true
	}
}
