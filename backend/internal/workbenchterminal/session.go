package workbenchterminal

import (
	"io"
	amitialog "log"
	"sync"
	"time"
)

type Snapshot struct {
	ID           string `json:"id"`
	TaskID       string `json:"taskId,omitempty"`
	WorkspaceURI string `json:"workspaceUri,omitempty"`
	Mode         string `json:"mode"`
	Running      bool   `json:"running"`
	ExitCode     int    `json:"exitCode"`
	Data         string `json:"data"`
	Cursor       int64  `json:"cursor"`
	FromCursor   int64  `json:"fromCursor"`
	Truncated    bool   `json:"truncated"`
}

type session struct {
	id                string
	owner             string
	taskID            string
	workspaceURI      string
	conversationID    string
	receiptJournal    *TaskReceiptJournal
	cwd               string
	proc              process
	mu                sync.Mutex
	inputMu           sync.Mutex
	closing           bool
	output            outputBuffer
	lastUsed          time.Time
	created           time.Time
	running           bool
	exitCode          int
	readDone          chan struct{}
	changed           chan struct{}
	lifecycle         []LifecycleEvent
	lifecycleSequence uint64
}

func (s *session) snapshot(cursor int64) Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotLocked(cursor)
}

func (s *session) snapshotLocked(cursor int64) Snapshot {
	if s.closing {
		return Snapshot{ID: s.id, Running: false, ExitCode: s.exitCode}
	}
	s.lastUsed = time.Now()
	data, start, next, truncated := s.output.readWithStart(cursor)
	return Snapshot{ID: s.id, TaskID: s.taskID, WorkspaceURI: s.workspaceURI, Mode: s.proc.Mode(), Running: s.running, ExitCode: s.exitCode, Data: data, Cursor: next, FromCursor: start, Truncated: truncated}
}

func (s *session) readSnapshot(cursor int64) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return Snapshot{}, ErrNotFound
	}
	return s.snapshotLocked(cursor), nil
}
func (s *session) readOutput() {
	defer close(s.readDone)
	buf := make([]byte, 16*1024)
	for {
		n, err := s.proc.Read(buf)
		if n > 0 {
			s.mu.Lock()
			if !s.closing {
				s.output.append(buf[:n])
				s.broadcastLocked()
			}
			s.mu.Unlock()
		}
		if err != nil {
			return
		}
	}
}
func (s *session) watchExit() {
	err := s.proc.Wait()
	select {
	case <-s.readDone:
	case <-time.After(2 * time.Second):
		if err := s.proc.Close(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		select {
		case <-s.readDone:
		case <-time.After(time.Second):
		}
	}
	s.mu.Lock()
	s.exitCode = exitCode(err)
	s.running = false
	code := s.exitCode
	s.recordLifecycleLocked(LifecycleEvent{Kind: "process.exited", ExitCode: &code})
	s.broadcastLocked()
	s.mu.Unlock()
	s.recordTaskReceipt("exited")
}
func (s *session) stopAcceptingCommands() {
	s.mu.Lock()
	s.closing = true
	for i := range s.output.data {
		s.output.data[i] = 0
	}
	s.output.data = nil
	s.broadcastLocked()
	s.mu.Unlock()
}

func (s *session) input(data []byte) error {
	s.inputMu.Lock()
	defer s.inputMu.Unlock()
	s.mu.Lock()
	running := s.running && !s.closing
	s.lastUsed = time.Now()
	s.mu.Unlock()
	if !running {
		return ErrExited
	}
	for len(data) > 0 {
		n, err := s.proc.Write(data)
		if err != nil {
			return err
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}
func (s *session) resize(rows, cols uint16) error {
	s.inputMu.Lock()
	defer s.inputMu.Unlock()
	s.mu.Lock()
	s.lastUsed = time.Now()
	running := s.running && !s.closing
	s.mu.Unlock()
	if !running {
		return ErrExited
	}
	if err := s.proc.Resize(rows, cols); err != nil {
		return err
	}
	s.mu.Lock()
	s.recordLifecycleLocked(LifecycleEvent{Kind: "session.resized", Rows: rows, Cols: cols})
	s.mu.Unlock()
	return nil
}
