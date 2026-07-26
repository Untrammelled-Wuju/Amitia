package dev_mode

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type FileChangeEvent struct {
	WorkspaceID WorkspaceID
	Path        string
	Kind        FileChangeKind
	Timestamp   time.Time
}

type FileChangeKind string

const (
	FileChangeKindModified FileChangeKind = "modified"
	FileChangeKindCreated  FileChangeKind = "created"
	FileChangeKindDeleted  FileChangeKind = "deleted"
	FileChangeKindRenamed  FileChangeKind = "renamed"
)

type FileWatcher struct {
	mu          sync.Mutex
	interval    time.Duration
	patterns    []string
	ignore      []string
	stopCh      map[WorkspaceID]chan struct{}
	running     map[WorkspaceID]bool
	snapshots   map[WorkspaceID]map[string]int64
}

func NewFileWatcher(interval time.Duration) *FileWatcher {
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	return &FileWatcher{
		interval: interval,
		patterns: []string{".ts", ".tsx", ".js", ".mjs", ".cjs", ".json", ".css", ".html"},
		ignore:   []string{"node_modules", "dist", "package", ".git", "tmp", "cache"},
		stopCh:   make(map[WorkspaceID]chan struct{}),
		running:  make(map[WorkspaceID]bool),
		snapshots: make(map[WorkspaceID]map[string]int64),
	}
}

var (
	ErrWatcherNotRunning = errors.New("dev_mode: watcher not running")
	ErrWatcherAlreadyRunning = errors.New("dev_mode: watcher already running")
)

func (w *FileWatcher) Start(ctx context.Context, id WorkspaceID, root string, events chan<- FileChangeEvent) error {
	w.mu.Lock()
	if w.running[id] {
		w.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrWatcherAlreadyRunning, id)
	}
	stop := make(chan struct{})
	w.stopCh[id] = stop
	w.running[id] = true
	w.snapshots[id] = make(map[string]int64)
	w.mu.Unlock()

	go w.loop(ctx, id, root, events, stop)
	return nil
}

func (w *FileWatcher) Stop(id WorkspaceID) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	stop, ok := w.stopCh[id]
	if !ok {
		return ErrWatcherNotRunning
	}
	close(stop)
	delete(w.stopCh, id)
	delete(w.running, id)
	delete(w.snapshots, id)
	return nil
}

func (w *FileWatcher) IsRunning(id WorkspaceID) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running[id]
}

func (w *FileWatcher) loop(ctx context.Context, id WorkspaceID, root string, events chan<- FileChangeEvent, stop chan struct{}) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	w.scan(id, root, events)
	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case <-ticker.C:
			w.scan(id, root, events)
		}
	}
}

func (w *FileWatcher) scan(id WorkspaceID, root string, events chan<- FileChangeEvent) {
	current := make(map[string]int64)
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			rel, _ := filepath.Rel(root, path)
			for _, ig := range w.ignore {
				if rel == ig || strings.HasPrefix(rel, ig+string(filepath.Separator)) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		matched := false
		for _, p := range w.patterns {
			if ext == p {
				matched = true
				break
			}
		}
		if !matched {
			return nil
		}
		current[path] = info.ModTime().UnixNano()
		return nil
	})

	w.mu.Lock()
	prev := w.snapshots[id]
	w.snapshots[id] = current
	w.mu.Unlock()

	if prev == nil {
		return
	}

	now := time.Now().UTC()
	for path, mtime := range current {
		old, exists := prev[path]
		if !exists {
			w.emit(events, FileChangeEvent{WorkspaceID: id, Path: path, Kind: FileChangeKindCreated, Timestamp: now})
			continue
		}
		if old != mtime {
			w.emit(events, FileChangeEvent{WorkspaceID: id, Path: path, Kind: FileChangeKindModified, Timestamp: now})
		}
	}
	for path := range prev {
		if _, exists := current[path]; !exists {
			w.emit(events, FileChangeEvent{WorkspaceID: id, Path: path, Kind: FileChangeKindDeleted, Timestamp: now})
		}
	}
}

func (w *FileWatcher) emit(events chan<- FileChangeEvent, ev FileChangeEvent) {
	select {
	case events <- ev:
	default:
		// drop on full channel; watcher continues
	}
}
