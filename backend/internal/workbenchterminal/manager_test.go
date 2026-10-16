package workbenchterminal

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

type fakeProcess struct {
	mu        sync.Mutex
	closed    bool
	writes    []byte
	readPipe  *io.PipeReader
	writePipe *io.PipeWriter
}

func newFakeProcess() *fakeProcess {
	r, w := io.Pipe()
	return &fakeProcess{readPipe: r, writePipe: w}
}
func (f *fakeProcess) Read(p []byte) (int, error) { return f.readPipe.Read(p) }
func (f *fakeProcess) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return 0, io.ErrClosedPipe
	}
	f.writes = append(f.writes, p...)
	return len(p), nil
}
func (f *fakeProcess) Close() error {
	f.mu.Lock()
	f.closed = true
	f.mu.Unlock()
	_ = f.readPipe.Close()
	_ = f.writePipe.Close()
	return nil
}
func (f *fakeProcess) Wait() error                    { return nil }
func (f *fakeProcess) Mode() string                   { return "fake" }
func (f *fakeProcess) Resize(rows, cols uint16) error { return nil }

func sessionFixture(t *testing.T) (*Manager, *fakeProcess, string) {
	t.Helper()
	manager := &Manager{sessions: make(map[string]*session), stop: make(chan struct{})}
	fake := newFakeProcess()
	id := "fixture"
	manager.sessions[id] = &session{id: id, owner: "first", proc: fake, running: true, exitCode: -1, readDone: make(chan struct{}), lastUsed: time.Now(), created: time.Now()}
	t.Cleanup(manager.Close)
	return manager, fake, id
}
func TestManagerRejectsOtherOwners(t *testing.T) {
	m, _, id := sessionFixture(t)
	if _, err := m.Read("second", id, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("read isolation: %v", err)
	}
	if err := m.Input("second", id, []byte("bad")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("input isolation: %v", err)
	}
	if err := m.CloseSession("second", id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete isolation: %v", err)
	}
}
func TestManagerInputAndClose(t *testing.T) {
	m, p, id := sessionFixture(t)
	if err := m.Input("first", id, []byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(p.writes, []byte("hello\n")) {
		t.Fatalf("output mismatch")
	}
	if err := m.CloseSession("first", id); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Read("first", id, 0); !errors.Is(err, ErrNotFound) {
		t.Fatal("closed session visible")
	}
}
func TestManagerInputLimits(t *testing.T) {
	m, _, id := sessionFixture(t)
	for _, payload := range [][]byte{nil, bytes.Repeat([]byte("x"), 4097)} {
		if err := m.Input("first", id, payload); !errors.Is(err, ErrInvalid) {
			t.Fatalf("input not limited: %v", err)
		}
	}
	if err := m.Resize("first", id, 1, 10); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad dimensions accepted")
	}
	if _, err := m.Read("first", id, -1); !errors.Is(err, ErrInvalid) {
		t.Fatal("negative cursor accepted")
	}
}
func TestManagerPrunesIdleSessions(t *testing.T) {
	m, f, id := sessionFixture(t)
	s := m.sessions[id]
	s.mu.Lock()
	s.lastUsed = time.Now().Add(-sessionTTL - time.Minute)
	s.mu.Unlock()
	m.prune()
	if _, err := m.Read("first", id, 0); !errors.Is(err, ErrNotFound) {
		t.Fatal("idle session retained")
	}
	f.mu.Lock()
	closed := f.closed
	f.mu.Unlock()
	if !closed {
		t.Fatal("expired process not closed")
	}
}
func TestManagerDuplicateClose(t *testing.T) {
	m, _, id := sessionFixture(t)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = m.CloseSession("first", id) }()
	}
	wg.Wait()
	if len(m.sessions) != 0 {
		t.Fatal("session leak")
	}
}
func TestExitCode(t *testing.T) {
	if exitCode(nil) != 0 {
		t.Fatal("success exit")
	}
}

func TestManagerKeepsRunningTaskWhenClientDisconnects(t *testing.T) {
	m, p, id := sessionFixture(t)
	s := m.sessions[id]
	s.mu.Lock()
	s.taskID = "task-running"
	s.conversationID = "conversation-1"
	s.lastUsed = time.Now().Add(-sessionTTL - 2*time.Minute)
	s.created = time.Now().Add(-2 * time.Hour)
	s.mu.Unlock()
	m.prune()
	if _, err := m.Read("first", id, 0); err != nil {
		t.Fatalf("running task incorrectly stopped after disconnect: %v", err)
	}
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		t.Fatal("running task process was killed")
	}
}

func TestManagerReclaimsFinishedDisconnectedTask(t *testing.T) {
	m, p, id := sessionFixture(t)
	s := m.sessions[id]
	s.mu.Lock()
	s.taskID = "task-finished"
	s.running = false
	s.lastUsed = time.Now().Add(-sessionTTL - time.Minute)
	s.mu.Unlock()
	m.prune()
	if _, err := m.Read("first", id, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("finished terminal retained: %v", err)
	}
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if !closed {
		t.Fatal("finished task process not reclaimed")
	}
}

func TestManagerRunningTaskStillRespectsAbsoluteTTL(t *testing.T) {
	m, p, id := sessionFixture(t)
	s := m.sessions[id]
	s.mu.Lock()
	s.taskID = "task-expired"
	s.created = time.Now().Add(-absoluteTTL - time.Minute)
	s.mu.Unlock()
	m.prune()
	if _, err := m.Read("first", id, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("absolute lifetime ignored: %v", err)
	}
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if !closed {
		t.Fatal("expired task process not killed")
	}
}
