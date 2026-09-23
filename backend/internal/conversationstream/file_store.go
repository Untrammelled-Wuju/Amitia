package conversationstream

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type ConversationSnapshot struct {
	ConversationID    string `json:"conversationId"`
	LastEventSequence int64  `json:"lastEventSequence"`
	EventCount        int    `json:"eventCount"`
	UpdatedAt         string `json:"updatedAt"`
}

type fileConversationState struct {
	mu     sync.Mutex
	loaded bool
	events []AgentUIEvent
}

type FileStore struct {
	root       string
	mu         sync.Mutex
	states     map[string]*fileConversationState
	maxEvents  int
	keepEvents int
}

func NewFileStore(root string) (*FileStore, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("conversation event root is required")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create conversation event root: %w", err)
	}
	return &FileStore{root: root, states: map[string]*fileConversationState{}, maxEvents: 4096, keepEvents: 2048}, nil
}

func (s *FileStore) Persist(ctx context.Context, event AgentUIEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	conversationID := strings.TrimSpace(event.ConversationID)
	if conversationID == "" {
		return errors.New("conversation id required")
	}
	if event.EventSequence <= 0 {
		return errors.New("event sequence must be positive")
	}
	state := s.state(conversationID)
	state.mu.Lock()
	defer state.mu.Unlock()
	if err := s.loadLocked(state, conversationID); err != nil {
		return err
	}
	if len(state.events) > 0 {
		last := state.events[len(state.events)-1]
		if event.EventSequence <= last.EventSequence {
			if event.EventSequence == last.EventSequence {
				return nil
			}
			return fmt.Errorf("conversation event sequence moved backwards: got %d after %d", event.EventSequence, last.EventSequence)
		}
	}
	path := s.path(conversationID)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create conversation event directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open conversation event log: %w", err)
	}
	encoder := json.NewEncoder(file)
	encodeErr := encoder.Encode(event)
	syncErr := file.Sync()
	closeErr := file.Close()
	if encodeErr != nil {
		return fmt.Errorf("append conversation event: %w", encodeErr)
	}
	if syncErr != nil {
		return fmt.Errorf("sync conversation event: %w", syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close conversation event log: %w", closeErr)
	}
	state.events = append(state.events, cloneEvent(event))
	if isTerminalEvent(event.Type) {
		if err := s.writeSnapshotLocked(state, conversationID); err != nil {
			return err
		}
		if err := s.compactLocked(state, conversationID); err != nil {
			return err
		}
	}
	return nil
}

func (s *FileStore) ListAfter(ctx context.Context, conversationID string, afterSequence int64, limit int) ([]AgentUIEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return []AgentUIEvent{}, nil
	}
	state := s.state(conversationID)
	state.mu.Lock()
	defer state.mu.Unlock()
	if err := s.loadLocked(state, conversationID); err != nil {
		return nil, err
	}
	if len(state.events) == 0 {
		return []AgentUIEvent{}, nil
	}
	start := sort.Search(len(state.events), func(index int) bool {
		return state.events[index].EventSequence > afterSequence
	})
	end := len(state.events)
	if limit > 0 && start+limit < end {
		end = start + limit
	}
	result := make([]AgentUIEvent, 0, end-start)
	for _, event := range state.events[start:end] {
		result = append(result, cloneEvent(event))
	}
	return result, nil
}

func (s *FileStore) LatestSequence(ctx context.Context, conversationID string) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return 0, nil
	}
	state := s.state(conversationID)
	state.mu.Lock()
	defer state.mu.Unlock()
	if err := s.loadLocked(state, conversationID); err != nil {
		return 0, err
	}
	if len(state.events) == 0 {
		snapshot, err := s.readSnapshot(conversationID)
		if err == nil && snapshot != nil {
			return snapshot.LastEventSequence, nil
		}
		return 0, nil
	}
	return state.events[len(state.events)-1].EventSequence, nil
}

func (s *FileStore) Snapshot(ctx context.Context, conversationID string) (*ConversationSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil, errors.New("conversation id required")
	}
	state := s.state(conversationID)
	state.mu.Lock()
	defer state.mu.Unlock()
	if err := s.loadLocked(state, conversationID); err != nil {
		return nil, err
	}
	if len(state.events) == 0 {
		return s.readSnapshot(conversationID)
	}
	if err := s.writeSnapshotLocked(state, conversationID); err != nil {
		return nil, err
	}
	return s.readSnapshot(conversationID)
}

func (s *FileStore) state(conversationID string) *fileConversationState {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.states[conversationID]
	if state == nil {
		state = &fileConversationState{}
		s.states[conversationID] = state
	}
	return state
}

func (s *FileStore) loadLocked(state *fileConversationState, conversationID string) error {
	if state.loaded {
		return nil
	}
	data, err := os.ReadFile(s.path(conversationID))
	if errors.Is(err, os.ErrNotExist) {
		state.loaded = true
		return nil
	}
	if err != nil {
		return fmt.Errorf("read conversation event log: %w", err)
	}
	lines := bytes.Split(data, []byte{'\n'})
	events := make([]AgentUIEvent, 0, len(lines))
	var lastSequence int64
	truncatedTail := false
	for index, raw := range lines {
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 {
			continue
		}
		var event AgentUIEvent
		if err := json.Unmarshal(raw, &event); err != nil {
			if index == len(lines)-1 {
				truncatedTail = true
				break
			}
			return fmt.Errorf("decode conversation event line %d: %w", index+1, err)
		}
		if event.EventSequence <= lastSequence {
			if event.EventSequence == lastSequence {
				continue
			}
			return fmt.Errorf("conversation event sequence moved backwards at line %d: got %d after %d", index+1, event.EventSequence, lastSequence)
		}
		events = append(events, cloneEvent(event))
		lastSequence = event.EventSequence
	}
	if truncatedTail {
		validLength := bytes.LastIndexByte(data, '\n') + 1
		if err := os.Truncate(s.path(conversationID), int64(validLength)); err != nil {
			return fmt.Errorf("repair truncated conversation event log: %w", err)
		}
	}
	state.events = events
	state.loaded = true
	return nil
}

func (s *FileStore) path(conversationID string) string {
	sum := sha256.Sum256([]byte(conversationID))
	name := hex.EncodeToString(sum[:]) + ".jsonl"
	return filepath.Join(s.root, name[:2], name)
}

func (s *FileStore) snapshotPath(conversationID string) string {
	return strings.TrimSuffix(s.path(conversationID), ".jsonl") + ".snapshot.json"
}

func (s *FileStore) writeSnapshotLocked(state *fileConversationState, conversationID string) error {
	if state == nil || len(state.events) == 0 {
		return nil
	}
	snapshot := ConversationSnapshot{
		ConversationID:    conversationID,
		LastEventSequence: state.events[len(state.events)-1].EventSequence,
		EventCount:        len(state.events),
		UpdatedAt:         time.Now().UTC().Format(time.RFC3339Nano),
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	path := s.snapshotPath(conversationID)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".snapshot-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		_ = os.Remove(tempPath)
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		_ = os.Remove(tempPath)
		return err
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	_ = os.Remove(path)
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	return nil
}

func (s *FileStore) compactLocked(state *fileConversationState, conversationID string) error {
	if state == nil || s.maxEvents <= 0 || len(state.events) <= s.maxEvents {
		return nil
	}
	keep := s.keepEvents
	if keep <= 0 || keep > len(state.events) {
		keep = len(state.events)
	}
	events := state.events[len(state.events)-keep:]
	path := s.path(conversationID)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".events-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	encoder := json.NewEncoder(temp)
	for _, event := range events {
		if err := encoder.Encode(event); err != nil {
			_ = temp.Close()
			_ = os.Remove(tempPath)
			return err
		}
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		_ = os.Remove(tempPath)
		return err
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	_ = os.Remove(path)
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	state.events = append([]AgentUIEvent(nil), events...)
	return nil
}

func (s *FileStore) readSnapshot(conversationID string) (*ConversationSnapshot, error) {
	data, err := os.ReadFile(s.snapshotPath(conversationID))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var snapshot ConversationSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func isTerminalEvent(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case "turn.completed", "turn.failed", "turn.interrupted":
		return true
	default:
		return false
	}
}
