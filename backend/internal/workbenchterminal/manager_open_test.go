package workbenchterminal

import (
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestManagerSlowLauncherDoesNotBlockExistingSessions(t *testing.T) {
	m := &Manager{sessions: make(map[string]*session), stop: make(chan struct{})}
	entered := make(chan struct{})
	release := make(chan struct{})
	m.startProcess = func(string, uint16, uint16) (process, error) {
		close(entered)
		<-release
		return newFakeProcess(), nil
	}
	result := make(chan error, 1)
	go func() { _, err := m.Open("first", "/safe", 30, 100); result <- err }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("launcher never started")
	}
	readDone := make(chan error, 1)
	go func() { _, err := m.Read("first", "missing", 0); readDone <- err }()
	select {
	case err := <-readDone:
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("unexpected read result: %v", err)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("slow process launch blocked other terminal operations")
	}
	m.Close()
	close(release)
	if err := <-result; !errors.Is(err, ErrClosed) {
		t.Fatalf("shutdown allowed a late process: %v", err)
	}
}

func TestManagerPendingOpenRespectsOwnerQuota(t *testing.T) {
	m := &Manager{sessions: make(map[string]*session), stop: make(chan struct{})}
	entered := make(chan struct{}, maxSessionsPerOwner)
	release := make(chan struct{})
	var lock sync.Mutex
	var created []*fakeProcess
	m.startProcess = func(string, uint16, uint16) (process, error) {
		entered <- struct{}{}
		<-release
		p := newFakeProcess()
		lock.Lock()
		created = append(created, p)
		lock.Unlock()
		return p, nil
	}
	result := make(chan error, maxSessionsPerOwner)
	for i := 0; i < maxSessionsPerOwner; i++ {
		go func() { _, err := m.Open("owner", "/safe", 30, 100); result <- err }()
	}
	for i := 0; i < maxSessionsPerOwner; i++ {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("pending launches never filled the quota")
		}
	}
	if _, err := m.Open("owner", "/safe", 30, 100); !errors.Is(err, ErrLimit) {
		t.Fatalf("pending launches must count toward owner quota: %v", err)
	}
	m.Close()
	close(release)
	for i := 0; i < maxSessionsPerOwner; i++ {
		if err := <-result; !errors.Is(err, ErrClosed) {
			t.Fatalf("late launch escaped shutdown: %v", err)
		}
	}
	lock.Lock()
	defer lock.Unlock()
	if len(created) != maxSessionsPerOwner {
		t.Fatalf("missing launched process: %d", len(created))
	}
	for _, p := range created {
		p.mu.Lock()
		closed := p.closed
		p.mu.Unlock()
		if !closed {
			t.Fatal("late process was not reclaimed")
		}
	}
	if m.openingTotal != 0 || len(m.openingByOwner) != 0 {
		t.Fatal("reservation leak")
	}
}

func TestManagerFailedLauncherReleasesReservation(t *testing.T) {
	m := &Manager{sessions: make(map[string]*session), stop: make(chan struct{})}
	defer m.Close()
	launchError := errors.New("launch failed")
	m.startProcess = func(string, uint16, uint16) (process, error) { return nil, launchError }
	if _, err := m.Open("owner", "/safe", 30, 100); !errors.Is(err, launchError) {
		t.Fatalf("launcher error hidden: %v", err)
	}
	if m.openingTotal != 0 || len(m.openingByOwner) != 0 {
		t.Fatal("failed reservation leaked")
	}
	if _, err := m.Open("owner", "/safe", 30, 100); !errors.Is(err, launchError) {
		t.Fatalf("retry error unexpected: %v", err)
	}
}

func TestManagerRejectsNewSessionsAfterClose(t *testing.T) {
	m := NewManager()
	m.Close()
	if _, err := m.Open("owner", "/safe", 30, 100); !errors.Is(err, ErrClosed) {
		t.Fatalf("closed manager accepted a launch: %v", err)
	}
}

func TestManagerPartialLauncherFailureClosesProcess(t *testing.T) {
	m := &Manager{sessions: make(map[string]*session), stop: make(chan struct{})}
	defer m.Close()
	proc := newFakeProcess()
	failure := errors.New("partial launch failure")
	m.startProcess = func(string, uint16, uint16) (process, error) { return proc, failure }
	if _, err := m.Open("owner", "/safe", 30, 100); !errors.Is(err, failure) {
		t.Fatalf("unexpected result: %v", err)
	}
	proc.mu.Lock()
	closed := proc.closed
	proc.mu.Unlock()
	if !closed {
		t.Fatal("failed partial process was not closed")
	}
	if m.openingTotal != 0 || len(m.openingByOwner) != 0 {
		t.Fatal("reservation leak")
	}
}

func TestManagerPendingOpenRespectsGlobalQuota(t *testing.T) {
	m := &Manager{sessions: make(map[string]*session), stop: make(chan struct{})}
	started := make(chan struct{}, maxSessionsTotal)
	release := make(chan struct{})
	m.startProcess = func(string, uint16, uint16) (process, error) {
		started <- struct{}{}
		<-release
		return nil, errors.New("no process")
	}
	results := make(chan error, maxSessionsTotal)
	for i := 0; i < maxSessionsTotal; i++ {
		owner := "owner-" + strconv.Itoa(i)
		go func() { _, err := m.Open(owner, "/safe", 30, 100); results <- err }()
	}
	for i := 0; i < maxSessionsTotal; i++ {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatal("pending opens were not launched")
		}
	}
	if _, err := m.Open("another-owner", "/safe", 30, 100); !errors.Is(err, ErrLimit) {
		t.Fatalf("global pending opens exceeded maximum: %v", err)
	}
	close(release)
	for i := 0; i < maxSessionsTotal; i++ {
		if err := <-results; err == nil {
			t.Fatal("failed launcher reported success")
		}
	}
	if m.openingTotal != 0 || len(m.openingByOwner) != 0 {
		t.Fatal("pending quota slots were not released")
	}
	m.Close()
}
