package conversationstream

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRound93DistinctFileStoresPreserveBothConfirmedEvents(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	firstStore, err := NewFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	secondStore, err := NewFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	first := NewManager()
	second := NewManager()
	first.SetDurableStore(firstStore)
	second.SetDurableStore(secondStore)
	if _, err := first.RuntimeSnapshotCurrent(ctx, "shared"); err != nil {
		t.Fatal(err)
	}
	if _, err := second.RuntimeSnapshotCurrent(ctx, "shared"); err != nil {
		t.Fatal(err)
	}
	one, err := first.Publish(ctx, AgentUIEvent{ConversationID: "shared", Type: "turn.started", TurnID: "first"}, true)
	if err != nil {
		t.Fatal(err)
	}
	two, err := second.Publish(ctx, AgentUIEvent{ConversationID: "shared", Type: "turn.started", TurnID: "second"}, true)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := NewFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := reopened.ListAfter(ctx, "shared", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted) != 2 || one.EventSequence == two.EventSequence || persisted[0].EventID != one.EventID || persisted[1].EventID != two.EventID {
		t.Fatalf("acknowledged events not durably preserved: first=%+v second=%+v disk=%+v", one, two, persisted)
	}
	fromFirst, err := firstStore.ListAfter(ctx, "shared", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(fromFirst) != 2 {
		t.Fatalf("existing FileStore failed to refresh durable state: %+v", fromFirst)
	}
}

func TestRound93FileStoreProcessHelper(t *testing.T) {
	if os.Getenv("AMITIA_ROUND93_HELPER") != "1" {
		return
	}
	root := os.Getenv("AMITIA_ROUND93_ROOT")
	eventID := os.Getenv("AMITIA_ROUND93_EVENT")
	readyFile := filepath.Join(root, "ready-"+eventID)
	store, err := NewFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(readyFile, []byte("ready"), 0600); err != nil {
		t.Fatal(err)
	}
	event := AgentUIEvent{ConversationID: "multi-process", EventID: eventID, EventSequence: 1, Type: "turn.started"}
	if err := store.Persist(context.Background(), event); err != nil {
		if errors.Is(err, ErrSequenceConflict) {
			fmt.Println("CONFLICT")
			return
		}
		t.Fatal(err)
	}
	fmt.Println("COMMITTED")
}

func TestRound93IndependentProcessesCannotBothCommitSameSequence(t *testing.T) {
	root := t.TempDir()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	commands := []*exec.Cmd{}
	for _, id := range []string{"one", "two"} {
		cmd := exec.Command(binary, "-test.run=^TestRound93FileStoreProcessHelper$", "-test.v")
		cmd.Env = append(os.Environ(), "AMITIA_ROUND93_HELPER=1", "AMITIA_ROUND93_ROOT="+root, "AMITIA_ROUND93_EVENT="+id)
		commands = append(commands, cmd)
	}
	results := make([]string, len(commands))
	errorsOut := make([]error, len(commands))
	done := make(chan int, len(commands))
	for i, cmd := range commands {
		go func(i int, cmd *exec.Cmd) {
			output, runErr := cmd.CombinedOutput()
			results[i] = string(output)
			errorsOut[i] = runErr
			done <- i
		}(i, cmd)
	}
	for range commands {
		<-done
	}
	successes := 0
	for i, output := range results {
		if errorsOut[i] != nil {
			t.Fatalf("subprocess %d failed: %v output=%s", i, errorsOut[i], output)
		}
		if strings.Contains(output, "COMMITTED") {
			successes++
		} else if !strings.Contains(output, "CONFLICT") {
			t.Fatalf("unexpected helper output: %s", output)
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly one same-sequence commit, got %d, outputs %v", successes, results)
	}
	reopened, err := NewFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := reopened.ListAfter(context.Background(), "multi-process", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 {
		t.Fatalf("lost or duplicated durable entry: %+v", stored)
	}
}

func TestRound93CancelledWaitCannotBypassFileLock(t *testing.T) {
	root := t.TempDir()
	store, err := NewFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	conversationID := "held"
	lockPath := strings.TrimSuffix(store.path(conversationID), ".jsonl") + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0700); err != nil {
		t.Fatal(err)
	}
	held, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	locked, err := tryFileLock(held)
	if err != nil || !locked {
		t.Fatalf("acquire initial lock: %v %v", locked, err)
	}
	defer func() { _ = unlockFile(held); _ = held.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	begin := time.Now()
	err = store.Persist(ctx, AgentUIEvent{ConversationID: conversationID, EventID: "blocked", EventSequence: 1, Type: "turn.started"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected expired lock wait, got %v", err)
	}
	if time.Since(begin) > time.Second {
		t.Fatal("file lock wait ignored context cancellation")
	}
	if err := unlockFile(held); err != nil {
		t.Fatal(err)
	}
	if err := store.Persist(context.Background(), AgentUIEvent{ConversationID: conversationID, EventID: "committed", EventSequence: 1, Type: "turn.started"}); err != nil {
		t.Fatal(err)
	}
	persisted, err := store.ListAfter(context.Background(), conversationID, 0, 10)
	if err != nil || len(persisted) != 1 || persisted[0].EventID != "committed" {
		t.Fatalf("cancelled persist wrote to disk: %+v, %v", persisted, err)
	}
}
