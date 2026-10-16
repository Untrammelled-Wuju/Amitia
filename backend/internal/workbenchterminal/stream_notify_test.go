package workbenchterminal

import (
	"testing"
	"time"
)

func TestStreamOutputBroadcastAndCursor(t *testing.T) {
	m, _, id := sessionFixture(t)
	s, err := m.get("first", id)
	if err != nil {
		t.Fatal(err)
	}
	_, channel := s.snapshotAndListen(0)
	s.mu.Lock()
	s.output.append([]byte("hello"))
	s.broadcastLocked()
	s.mu.Unlock()
	select {
	case <-channel:
	case <-time.After(time.Second):
		t.Fatal("missed output update")
	}
	snap, newChannel := s.snapshotAndListen(0)
	if snap.FromCursor != 0 || snap.Cursor != 5 || snap.Truncated {
		t.Fatalf("wrong snapshot: %+v", snap)
	}
	s.mu.Lock()
	s.running = false
	s.exitCode = 0
	s.broadcastLocked()
	s.mu.Unlock()
	select {
	case <-newChannel:
	case <-time.After(time.Second):
		t.Fatal("missed exit event")
	}
	final, _ := s.snapshotAndListen(5)
	if final.Running || final.ExitCode != 0 || final.FromCursor != 5 || final.Cursor != 5 {
		t.Fatalf("terminal exit lost: %+v", final)
	}
}

func TestStreamWakeMultipleReaders(t *testing.T) {
	m, _, id := sessionFixture(t)
	s, err := m.get("first", id)
	if err != nil {
		t.Fatal(err)
	}
	_, a := s.snapshotAndListen(0)
	_, b := s.snapshotAndListen(0)
	s.mu.Lock()
	s.output.append([]byte("x"))
	s.broadcastLocked()
	s.mu.Unlock()
	select {
	case <-a:
	default:
		t.Fatal("first observer did not wake")
	}
	select {
	case <-b:
	default:
		t.Fatal("second observer did not wake")
	}
}
