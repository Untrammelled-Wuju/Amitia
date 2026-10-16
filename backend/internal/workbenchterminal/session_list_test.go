package workbenchterminal

import (
	"testing"
	"time"
)

func TestSessionListingIsIsolatedAndSorted(t *testing.T) {
	m := NewManager()
	defer m.Close()
	start := time.Now()
	m.mu.Lock()
	m.sessions["older"] = &session{id: "older", owner: "owner-a", created: start.Add(-time.Hour), proc: fakeListProcess{}, running: true}
	m.sessions["newer"] = &session{id: "newer", owner: "owner-a", created: start, proc: fakeListProcess{}, running: false, exitCode: 7}
	m.sessions["private"] = &session{id: "private", owner: "owner-b", created: start.Add(time.Hour), proc: fakeListProcess{}, running: true}
	m.mu.Unlock()
	own := m.List("owner-a")
	if len(own) != 2 || own[0].ID != "newer" || own[1].ID != "older" || own[0].ExitCode != 7 {
		t.Fatalf("unexpected listing: %+v", own)
	}
	if len(m.List("owner-b")) != 1 || len(m.List("")) != 0 {
		t.Fatal("session listing escaped owner scope")
	}
}

type fakeListProcess struct{}

func (fakeListProcess) Read([]byte) (int, error)    { return 0, nil }
func (fakeListProcess) Write(p []byte) (int, error) { return len(p), nil }
func (fakeListProcess) Close() error                { return nil }
func (fakeListProcess) Resize(uint16, uint16) error { return nil }
func (fakeListProcess) Wait() error                 { return nil }
func (fakeListProcess) Mode() string                { return "pty" }
