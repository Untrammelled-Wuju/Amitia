package workbenchterminal

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTaskReceiptJournalMultipleInstancesMonotonic(t *testing.T) {
	first, fixture := receiptFixture(t)
	second, err := NewTaskReceiptJournal(first.dir)
	if err != nil {
		t.Fatal(err)
	}
	fixture.LatestSequence = 100
	if err = first.Record("owner", fixture); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			copy := fixture
			copy.LatestSequence = uint64(i + 1)
			if err := second.Record("owner", copy); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	list, err := second.List("owner", fixture.ConversationID, fixture.TaskID, map[string]bool{fixture.SessionID: true})
	if err != nil || len(list) != 1 || list[0].LatestSequence != 100 {
		t.Fatalf("stale overwrite: %+v %v", list, err)
	}
}

func TestTaskReceiptJournalLockTimeout(t *testing.T) {
	first, _ := receiptFixture(t)
	unlock, err := lockReceiptJournal(filepath.Join(first.dir, ".journal.lock"), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	second := make(chan error, 1)
	go func() {
		_, e := lockReceiptJournal(filepath.Join(first.dir, ".journal.lock"), 75*time.Millisecond)
		second <- e
	}()
	select {
	case err := <-second:
		if !errors.Is(err, ErrReceiptUnavailable) {
			t.Fatalf("expected unavailable: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("lock timeout blocked")
	}
	unlock()
	unlock2, err := lockReceiptJournal(filepath.Join(first.dir, ".journal.lock"), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	unlock2()
}

func TestTaskReceiptCrossProcessWorker(t *testing.T) {
	path := os.Getenv("WORKBENCH_RECEIPT_WORKER_PATH")
	if path == "" {
		return
	}
	journal, err := NewTaskReceiptJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	index, err := strconv.Atoi(os.Getenv("WORKBENCH_RECEIPT_WORKER_INDEX"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		r := TaskReceipt{SessionID: strings.Repeat(strconv.FormatInt(int64(index+1), 16), 38) + strconv.FormatInt(int64(i/10), 16) + strconv.FormatInt(int64(i%10), 16), TaskID: "task-a", ConversationID: "conv-a", WorkspaceURI: "amitia://workspace/@repo/", Mode: "pty", StartedAt: time.Now().UTC(), State: "running", LatestSequence: 1}
		if err := journal.Record("owner", r); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTaskReceiptJournalCrossProcessWrites(t *testing.T) {
	journal, _ := receiptFixture(t)
	const workers = 4
	commands := make([]*exec.Cmd, workers)
	for i := 0; i < workers; i++ {
		cmd := exec.Command(os.Args[0], "-test.run=^TestTaskReceiptCrossProcessWorker$")
		cmd.Env = append(os.Environ(), "WORKBENCH_RECEIPT_WORKER_PATH="+journal.dir, "WORKBENCH_RECEIPT_WORKER_INDEX="+strconv.Itoa(i))
		commands[i] = cmd
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
	}
	for _, cmd := range commands {
		if err := cmd.Wait(); err != nil {
			t.Fatalf("worker failed: %v", err)
		}
	}
	entries, err := os.ReadDir(journal.dir)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			count++
		}
	}
	if count != workers*12 {
		t.Fatalf("lost writes or collisions: %d", count)
	}
}
