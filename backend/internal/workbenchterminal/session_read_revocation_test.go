package workbenchterminal

import (
	"encoding/base64"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestRevocationFencesAllOutputReaders(t *testing.T) {
	for _, mode := range []string{"revoke", "close", "prune", "shutdown"} {
		t.Run(mode, func(t *testing.T) {
			m, _, id := sessionFixture(t)
			stale := m.sessions[id]
			stale.mu.Lock()
			stale.output.append([]byte("private-terminal-output"))
			stale.recordLifecycleLocked(LifecycleEvent{Kind: "session.test"})
			stale.mu.Unlock()
			initial, err := m.Read("first", id, 0)
			if err != nil || initial.Data == "" {
				t.Fatalf("live output unavailable: %+v %v", initial, err)
			}
			if _, err := m.Export("first", id); err != nil {
				t.Fatalf("live export unavailable: %v", err)
			}
			if _, err := m.Lifecycle("first", id, 0); err != nil {
				t.Fatalf("live lifecycle unavailable: %v", err)
			}
			_, notification := stale.snapshotAndListen(0)
			stale.mu.Lock()
			heldOutput := stale.output.data
			stale.mu.Unlock()
			switch mode {
			case "revoke":
				err = m.RevokeSession("first", id)
			case "close":
				err = m.CloseSession("first", id)
			case "prune":
				stale.mu.Lock()
				stale.lastUsed = time.Now().Add(-sessionTTL - time.Minute)
				stale.mu.Unlock()
				m.prune()
			case "shutdown":
				m.Close()
			}
			if err != nil {
				t.Fatalf("unable to revoke session: %v", err)
			}
			for _, b := range heldOutput {
				if b != 0 {
					t.Fatal("revoked output buffer was not cleared")
				}
			}
			select {
			case <-notification:
			case <-time.After(time.Second):
				t.Fatal("old output listener was not notified")
			}
			if _, err := m.Read("first", id, 0); !errors.Is(err, ErrNotFound) {
				t.Fatalf("removed session remained readable: %v", err)
			}
			if _, err := m.Export("first", id); !errors.Is(err, ErrNotFound) {
				t.Fatalf("removed session remained exportable: %v", err)
			}
			if _, err := m.Lifecycle("first", id, 0); !errors.Is(err, ErrNotFound) {
				t.Fatalf("removed session lifecycle remained accessible: %v", err)
			}
			if _, err := stale.readSnapshot(0); !errors.Is(err, ErrNotFound) {
				t.Fatalf("stale pointer read accepted: %v", err)
			}
			if _, err := stale.lifecyclePage(0); !errors.Is(err, ErrNotFound) {
				t.Fatalf("stale lifecycle accepted: %v", err)
			}
			for _, snap := range []Snapshot{stale.snapshot(0), func() Snapshot { s, _ := stale.snapshotAndListen(0); return s }()} {
				if snap.Data != "" || snap.Running || snap.TaskID != "" || snap.WorkspaceURI != "" {
					t.Fatalf("revoked snapshot leaked retained data: %+v", snap)
				}
			}
			stale.mu.Lock()
			stale.output.append([]byte("late-secret"))
			stale.mu.Unlock()
			if got := stale.snapshot(0).Data; got != "" {
				t.Fatalf("late terminal content became readable: %s", got)
			}
		})
	}
}

func TestConcurrentTerminalReadAndRevokeNeverReopensSession(t *testing.T) {
	m, _, id := sessionFixture(t)
	stale := m.sessions[id]
	stale.mu.Lock()
	stale.output.append([]byte("private\x00bytes"))
	stale.mu.Unlock()
	var readers sync.WaitGroup
	for i := 0; i < 12; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for n := 0; n < 40; n++ {
				_, _ = m.Read("first", id, 0)
				_, _ = m.Export("first", id)
				_, _ = m.Lifecycle("first", id, 0)
			}
		}()
	}
	if err := m.RevokeSession("first", id); err != nil {
		t.Fatal(err)
	}
	readers.Wait()
	if got, err := stale.readSnapshot(0); !errors.Is(err, ErrNotFound) || got.Data != "" {
		t.Fatalf("stale pointer survived concurrent revoke: %+v %v", got, err)
	}
	if data, err := base64.StdEncoding.DecodeString(stale.snapshot(0).Data); err != nil || len(data) != 0 {
		t.Fatalf("revoked raw terminal bytes leaked: %x %v", data, err)
	}
}
