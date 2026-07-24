// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"testing"

	"github.com/u-ai/backend/internal/imageprovider/backgroundremoval"
)

func TestAnchorDefaultFeetCenter(t *testing.T) {
	a := DefaultAnchorForActionKey("idle_normal")
	if a.Type != AnchorFeetCenter {
		t.Errorf("expected feet_center, got %s", a.Type)
	}
	if a.X != 0.5 || a.Y != 0.92 {
		t.Errorf("expected 0.5,0.92, got %v,%v", a.X, a.Y)
	}
}

func TestAnchorDefaultBreathing(t *testing.T) {
	a := DefaultAnchorForActionKey("idle_breathing")
	if a.Type != AnchorFeetCenter {
		t.Errorf("expected feet_center, got %s", a.Type)
	}
}

func TestAnchorDefaultWalkLeft(t *testing.T) {
	a := DefaultAnchorForActionKey("walk_left")
	if a.Type != AnchorFeetCenter {
		t.Errorf("expected feet_center for walk_left, got %s", a.Type)
	}
}

func TestAnchorDefaultFall(t *testing.T) {
	a := DefaultAnchorForActionKey("fall")
	if a.Type != AnchorBodyCenter {
		t.Errorf("expected body_center, got %s", a.Type)
	}
	if a.X != 0.5 || a.Y != 0.5 {
		t.Errorf("expected 0.5,0.5, got %v,%v", a.X, a.Y)
	}
}

func TestAnchorDefaultPickedUp(t *testing.T) {
	a := DefaultAnchorForActionKey("picked_up")
	if a.Type != AnchorBodyCenter {
		t.Errorf("expected body_center, got %s", a.Type)
	}
}

func TestAnchorDefaultSitWindowEdge(t *testing.T) {
	a := DefaultAnchorForActionKey("sit_window_edge")
	if a.Type != AnchorWindowEdgeContact {
		t.Errorf("expected window_edge_contact, got %s", a.Type)
	}
	if a.Y != 0.95 {
		t.Errorf("expected Y=0.95, got %v", a.Y)
	}
}

func TestAnchorDefaultClimbWindow(t *testing.T) {
	a := DefaultAnchorForActionKey("climb_window")
	if a.Type != AnchorHandContact {
		t.Errorf("expected hand_contact, got %s", a.Type)
	}
	if a.Y != 0.1 {
		t.Errorf("expected Y=0.1, got %v", a.Y)
	}
}

func TestAnchorDefaultSleepOnDesk(t *testing.T) {
	a := DefaultAnchorForActionKey("sleep_on_desk")
	if a.Type != AnchorBodyCenter {
		t.Errorf("expected body_center, got %s", a.Type)
	}
	if a.Y != 0.95 {
		t.Errorf("expected Y=0.95, got %v", a.Y)
	}
}

func TestAnchorDefaultSpecsAlias(t *testing.T) {
	cases := map[string]AnchorMode{
		"sleep_on_desktop": AnchorBodyCenter,
		"edge_sit":         AnchorWindowEdgeContact,
		"edge_climb":       AnchorHandContact,
	}
	for key, expected := range cases {
		a := DefaultAnchorForActionKey(key)
		if a.Type != expected {
			t.Errorf("for %s expected %s, got %s", key, expected, a.Type)
		}
	}
}

func TestAnchorDefaultUnknown(t *testing.T) {
	a := DefaultAnchorForActionKey("unknown_action_xyz")
	if a.Type != AnchorFeetCenter {
		t.Errorf("expected feet_center for unknown, got %s", a.Type)
	}
	if a.X != DefaultFeetCenterAnchor.X || a.Y != DefaultFeetCenterAnchor.Y {
		t.Errorf("expected default feet center anchor, got %+v", a)
	}
}

func TestAnchorComputeFeetCenter(t *testing.T) {
	box := backgroundremoval.SubjectBox{MinX: 10, MinY: 20, MaxX: 50, MaxY: 80, Width: 41, Height: 61, Empty: false}
	a := ComputeAnchor(box, AnchorFeetCenter)
	if a.X != 30 || a.Y != 80 {
		t.Errorf("expected (30,80), got (%v,%v)", a.X, a.Y)
	}
	if a.Type != AnchorFeetCenter {
		t.Errorf("expected type feet_center, got %s", a.Type)
	}
}

func TestAnchorComputeBodyCenter(t *testing.T) {
	box := backgroundremoval.SubjectBox{MinX: 10, MinY: 20, MaxX: 50, MaxY: 80, Width: 41, Height: 61, Empty: false}
	a := ComputeAnchor(box, AnchorBodyCenter)
	if a.X != 30 || a.Y != 50 {
		t.Errorf("expected (30,50), got (%v,%v)", a.X, a.Y)
	}
}

func TestAnchorComputeHeadCenter(t *testing.T) {
	box := backgroundremoval.SubjectBox{MinX: 10, MinY: 20, MaxX: 50, MaxY: 80, Width: 41, Height: 61, Empty: false}
	a := ComputeAnchor(box, AnchorHeadCenter)
	if a.X != 30 || a.Y != 20 {
		t.Errorf("expected (30,20), got (%v,%v)", a.X, a.Y)
	}
}

func TestAnchorComputeHandContact(t *testing.T) {
	box := backgroundremoval.SubjectBox{MinX: 10, MinY: 20, MaxX: 50, MaxY: 80, Width: 41, Height: 61, Empty: false}
	a := ComputeAnchor(box, AnchorHandContact)
	if a.X != 30 || a.Y != 20 {
		t.Errorf("expected (30,20), got (%v,%v)", a.X, a.Y)
	}
}

func TestAnchorComputeWindowEdgeContact(t *testing.T) {
	box := backgroundremoval.SubjectBox{MinX: 10, MinY: 20, MaxX: 50, MaxY: 80, Width: 41, Height: 61, Empty: false}
	a := ComputeAnchor(box, AnchorWindowEdgeContact)
	if a.X != 30 || a.Y != 80 {
		t.Errorf("expected (30,80), got (%v,%v)", a.X, a.Y)
	}
}

func TestAnchorComputeEmptyBox(t *testing.T) {
	box := backgroundremoval.SubjectBox{Empty: true}
	a := ComputeAnchor(box, AnchorFeetCenter)
	if a.X != 0.5 || a.Y != 0.5 {
		t.Errorf("expected (0.5,0.5) for empty box, got (%v,%v)", a.X, a.Y)
	}
}
