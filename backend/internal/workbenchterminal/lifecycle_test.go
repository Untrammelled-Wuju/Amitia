package workbenchterminal

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestLifecycleOwnerIsolationAndCursor(t *testing.T) {
	m, _, id := sessionFixture(t)
	s, err := m.get("first", id)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.recordLifecycleLocked(LifecycleEvent{Kind: "session.started", Rows: 30, Cols: 80})
	s.recordLifecycleLocked(LifecycleEvent{Kind: "session.resized", Rows: 40, Cols: 100})
	s.mu.Unlock()
	if _, err := m.Lifecycle("second", id, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("owner leak: %v", err)
	}
	first, err := m.Lifecycle("first", id, 0)
	if err != nil || len(first.Events) != 2 || first.Events[0].Sequence != 1 || first.LatestSequence != 2 || first.Truncated {
		t.Fatalf("initial: %+v %v", first, err)
	}
	next, err := m.Lifecycle("first", id, 1)
	if err != nil || len(next.Events) != 1 || next.Events[0].Sequence != 2 {
		t.Fatalf("incremental: %+v %v", next, err)
	}
	future, err := m.Lifecycle("first", id, 999)
	if err != nil || len(future.Events) != 0 {
		t.Fatalf("future cursor: %+v %v", future, err)
	}
}

func TestLifecycleBoundedAndConcurrent(t *testing.T) {
	m, _, id := sessionFixture(t)
	s, _ := m.get("first", id)
	var wg sync.WaitGroup
	for n := 0; n < 4; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 70; i++ {
				s.mu.Lock()
				s.recordLifecycleLocked(LifecycleEvent{Kind: "session.resized", Rows: 30, Cols: 80})
				s.mu.Unlock()
			}
		}()
	}
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 70; i++ {
				_, _ = m.Lifecycle("first", id, 0)
			}
		}()
	}
	wg.Wait()
	events, err := m.Lifecycle("first", id, 0)
	if err != nil || !events.Truncated || len(events.Events) != maxLifecycleEvents || events.LatestSequence != 280 || events.Events[0].Sequence != 217 {
		t.Fatalf("bounded: %+v %v", events, err)
	}
	if _, err = m.Read("first", id, 0); err != nil {
		t.Fatal(err)
	}
}

func TestLifecycleResizeAndExit(t *testing.T) {
	m, _, id := sessionFixture(t)
	if err := m.Resize("first", id, 45, 110); err != nil {
		t.Fatal(err)
	}
	s, _ := m.get("first", id)
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()
	s.readDone = make(chan struct{})
	close(s.readDone)
	done := make(chan struct{})
	go func() { s.watchExit(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("wait deadlock")
	}
	events, _ := m.Lifecycle("first", id, 0)
	if len(events.Events) != 2 || events.Events[0].Kind != "session.resized" || events.Events[1].Kind != "process.exited" || events.Events[1].ExitCode == nil || *events.Events[1].ExitCode != 0 {
		t.Fatalf("incorrect exit events %+v", events)
	}
}

func TestLifecycleExportTracksRetainedEvents(t *testing.T) {
	m, _, id := sessionFixture(t)
	s, _ := m.get("first", id)
	s.mu.Lock()
	s.taskID = "task-9"
	s.recordLifecycleLocked(LifecycleEvent{Kind: "session.started", Rows: 30, Cols: 90})
	s.mu.Unlock()
	transcript, err := m.Export("first", id)
	if err != nil || transcript.TaskID != "task-9" || transcript.LifecycleLatestSequence != 1 || transcript.LifecycleTruncated || len(transcript.Lifecycle) != 1 || transcript.Lifecycle[0].Kind != "session.started" {
		t.Fatalf("export: %+v %v", transcript, err)
	}
	s.mu.Lock()
	for i := 0; i < 80; i++ {
		s.recordLifecycleLocked(LifecycleEvent{Kind: "session.resized", Rows: 30, Cols: 90})
	}
	s.mu.Unlock()
	transcript, err = m.Export("first", id)
	if err != nil || !transcript.LifecycleTruncated || transcript.LifecycleLatestSequence != 81 || len(transcript.Lifecycle) != maxLifecycleEvents {
		t.Fatal("export must disclose dropped events")
	}
	if _, err = m.Export("another", id); !errors.Is(err, ErrNotFound) {
		t.Fatal("export owner bypass")
	}
}
