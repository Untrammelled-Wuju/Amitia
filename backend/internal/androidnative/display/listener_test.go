package display

import (
	"testing"
	"time"
)

func TestListener_EmitAndSubscribe(t *testing.T) {
	l := NewListener()
	ch, cancel := l.Subscribe()
	defer cancel()

	evt := DisplayEvent{
		Type:       string(EventTypeAdded),
		DisplayID:  1,
		Generation: 1,
		ObservedAt: time.Now().UnixMilli(),
	}

	l.Emit(evt)

	select {
	case got := <-ch:
		if got.DisplayID != 1 {
			t.Errorf("expected display 1, got %d", got.DisplayID)
		}
		if got.Type != string(EventTypeAdded) {
			t.Errorf("expected type added, got %s", got.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestListener_MultipleSubscribers(t *testing.T) {
	l := NewListener()
	ch1, cancel1 := l.Subscribe()
	defer cancel1()
	ch2, cancel2 := l.Subscribe()
	defer cancel2()

	if l.CountSubscribers() != 2 {
		t.Errorf("expected 2 subscribers, got %d", l.CountSubscribers())
	}

	evt := DisplayEvent{Type: string(EventTypeRemoved), DisplayID: 2}
	l.Emit(evt)

	select {
	case <-ch1:
	case <-time.After(time.Second):
		t.Fatal("ch1 timeout")
	}
	select {
	case <-ch2:
	case <-time.After(time.Second):
		t.Fatal("ch2 timeout")
	}
}

func TestListener_Close(t *testing.T) {
	l := NewListener()
	ch, _ := l.Subscribe()
	l.Close()

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected closed channel")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestBuildEventFromAdd(t *testing.T) {
	info := DisplayInfo{DisplayID: 3, Generation: 1, Name: "External"}
	evt := BuildEventFromAdd(info, 1000)
	if evt.Type != string(EventTypeAdded) {
		t.Errorf("expected added, got %s", evt.Type)
	}
	if evt.DisplayID != 3 {
		t.Errorf("expected display 3, got %d", evt.DisplayID)
	}
	if evt.ObservedAt != 1000 {
		t.Errorf("expected observedAt 1000, got %d", evt.ObservedAt)
	}
}

func TestBuildEventFromRemove(t *testing.T) {
	rec := &DisplayRecord{
		Info: DisplayInfo{DisplayID: 5, Generation: 2},
	}
	evt := BuildEventFromRemove(rec, 2000)
	if evt.Type != string(EventTypeRemoved) {
		t.Errorf("expected removed, got %s", evt.Type)
	}
	if evt.DisplayID != 5 {
		t.Errorf("expected display 5, got %d", evt.DisplayID)
	}
}

func TestBuildEventFromChange(t *testing.T) {
	prev := DisplayInfo{DisplayID: 1, Width: 1080, Height: 2400}
	next := DisplayInfo{DisplayID: 1, Width: 1440, Height: 3200}
	evt := BuildEventFromChange(prev, next, []string{"size"}, 3000)
	if evt.Type != string(EventTypeChanged) {
		t.Errorf("expected changed, got %s", evt.Type)
	}
	if len(evt.ChangedFields) != 1 || evt.ChangedFields[0] != "size" {
		t.Errorf("expected changedFields [size], got %v", evt.ChangedFields)
	}
}

func TestChangedFieldsDisplay(t *testing.T) {
	tests := []struct {
		name     string
		prev     DisplayInfo
		next     DisplayInfo
		expected []string
	}{
		{
			name:     "no change",
			prev:     DisplayInfo{Width: 1080, Height: 2400, Rotation: 0, DensityDPI: 420, State: "on"},
			next:     DisplayInfo{Width: 1080, Height: 2400, Rotation: 0, DensityDPI: 420, State: "on"},
			expected: nil,
		},
		{
			name:     "size change",
			prev:     DisplayInfo{Width: 1080, Height: 2400},
			next:     DisplayInfo{Width: 1440, Height: 3200},
			expected: []string{"size"},
		},
		{
			name:     "rotation change",
			prev:     DisplayInfo{Rotation: 0},
			next:     DisplayInfo{Rotation: 1},
			expected: []string{"rotation"},
		},
		{
			name:     "state change",
			prev:     DisplayInfo{State: "on"},
			next:     DisplayInfo{State: "off"},
			expected: []string{"state"},
		},
		{
			name:     "multiple changes",
			prev:     DisplayInfo{Width: 1080, Height: 2400, Rotation: 0, DensityDPI: 420, RefreshRate: 60, State: "on", IsValid: true},
			next:     DisplayInfo{Width: 1440, Height: 3200, Rotation: 1, DensityDPI: 480, RefreshRate: 120, State: "off", IsValid: false},
			expected: []string{"size", "rotation", "density", "state", "refreshRate", "valid"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ChangedFieldsDisplay(tt.prev, tt.next)
			if len(got) != len(tt.expected) {
				t.Errorf("expected %d fields, got %d: %v", len(tt.expected), len(got), got)
				return
			}
			for i, f := range got {
				if f != tt.expected[i] {
					t.Errorf("field %d: expected %s, got %s", i, tt.expected[i], f)
				}
			}
		})
	}
}
