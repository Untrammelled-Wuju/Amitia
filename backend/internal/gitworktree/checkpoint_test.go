package gitworktree

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func checkpointFixture(t *testing.T) (*Manager, string, Entry) {
	t.Helper()
	manager, root := fixture(t)
	entry, _, err := manager.EnsureTask(context.Background(), root, "", "HEAD", "task-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	return manager, root, entry
}

func TestCheckpointPrepareAndIdempotency(t *testing.T) {
	m, root, entry := checkpointFixture(t)
	ctx := context.Background()
	first, err := m.PrepareCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	again, err := m.PrepareCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1")
	if err != nil || first.Revision != again.Revision || !first.CreatedAt.Equal(again.CreatedAt) {
		t.Fatalf("idempotency: %v %#v", err, again)
	}
	fresh, err := New(m.dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := fresh.ReadCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1")
	if err != nil || recovered.Revision != first.Revision {
		t.Fatalf("recovery: %#v %v", recovered, err)
	}
	if _, err := m.ReadCheckpoint(ctx, root, entry.ID, "other-task", "conversation-1"); !errors.Is(err, ErrBindingMismatch) {
		t.Fatalf("wrong task accepted: %v", err)
	}
}

func TestCheckpointRejectsDirtyAndStaleRevision(t *testing.T) {
	m, root, entry := checkpointFixture(t)
	ctx := context.Background()
	first, err := m.PrepareCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(entry.Path, "draft.txt"), []byte("pending"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := m.PrepareCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1"); !errors.Is(err, ErrDirty) {
		t.Fatalf("dirty checkpoint accepted: %v", err)
	}
	gitTest(t, entry.Path, "add", "draft.txt")
	gitTest(t, entry.Path, "-c", "user.name=Test", "-c", "user.email=test@example.org", "commit", "-m", "change")
	if _, err := m.ReadCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1"); !errors.Is(err, ErrCheckpointRequired) {
		t.Fatalf("stale accepted %v", err)
	}
	second, err := m.PrepareCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1")
	if err != nil || second.Revision == first.Revision {
		t.Fatalf("not refreshed: %v", err)
	}
}

func TestCheckpointRequiresVerifiedExecutionAndExplicitApproval(t *testing.T) {
	m, root, entry := checkpointFixture(t)
	ctx := context.Background()
	first, err := m.PrepareCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.ApproveCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1", first.Revision, "local-owner"); !errors.Is(err, ErrTestRequired) {
		t.Fatalf("approval without test: %v", err)
	}
	if _, err := m.RunCheckpointTest(ctx, root, entry.ID, "task-1", "conversation-1", first.Revision, "arbitrary-command"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("arbitrary command accepted: %v", err)
	}
	if err := os.WriteFile(filepath.Join(entry.Path, "go.mod"), []byte("module sandbox.example/checkpoint\n\ngo 1.23\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(entry.Path, "check_test.go"), []byte("package checkpoint\nimport \"testing\"\nfunc TestExample(t *testing.T) {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, entry.Path, "add", "go.mod", "check_test.go")
	gitTest(t, entry.Path, "-c", "user.name=Test", "-c", "user.email=test@example.org", "commit", "-m", "add tests")
	updated, err := m.PrepareCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	afterTest, err := m.RunCheckpointTest(ctx, root, entry.ID, "task-1", "conversation-1", updated.Revision, "go")
	if err != nil {
		t.Fatal(err)
	}
	if len(afterTest.Tests) != 1 || !afterTest.Tests[0].Passed || afterTest.Tests[0].ExitCode != 0 || afterTest.Tests[0].OutputDigest == "" {
		t.Fatalf("test evidence: %+v", afterTest.Tests)
	}
	approved, err := m.ApproveCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1", updated.Revision, "local-owner")
	if err != nil || approved.ApprovedAt == nil {
		t.Fatalf("approval: %+v %v", approved, err)
	}
	if _, err = m.ApproveCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1", updated.Revision, "second-user"); !errors.Is(err, ErrApprovalDenied) {
		t.Fatalf("approval should be once only %v", err)
	}
}

func TestCheckpointFailureCannotBeApproved(t *testing.T) {
	m, root, entry := checkpointFixture(t)
	if err := os.WriteFile(filepath.Join(entry.Path, "go.mod"), []byte("module sandbox.example/fail\n\ngo 1.23\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(entry.Path, "fail_test.go"), []byte("package fail\nimport \"testing\"\nfunc TestFail(t *testing.T){t.Fatal(\"fail\")}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, entry.Path, "add", "go.mod", "fail_test.go")
	gitTest(t, entry.Path, "-c", "user.name=Test", "-c", "user.email=test@example.org", "commit", "-m", "failing test")
	ctx := context.Background()
	record, err := m.PrepareCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	tested, err := m.RunCheckpointTest(ctx, root, entry.ID, "task-1", "conversation-1", record.Revision, "go")
	if err != nil {
		t.Fatal(err)
	}
	if len(tested.Tests) != 1 || tested.Tests[0].Passed || tested.Tests[0].ExitCode == 0 {
		t.Fatalf("false pass %+v", tested.Tests)
	}
	if _, err = m.ApproveCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1", record.Revision, "local-owner"); !errors.Is(err, ErrTestRequired) {
		t.Fatalf("failed test approved %v", err)
	}
}

func TestCheckpointCorruptStorageIsNotSilentlyReset(t *testing.T) {
	m, root, entry := checkpointFixture(t)
	ctx := context.Background()
	_, err := m.PrepareCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	_, key, err := m.repository(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	path := m.checkpointPath(key, entry.ID)
	if err = os.WriteFile(path, []byte("{malformed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = m.PrepareCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1"); err == nil {
		t.Fatal("corrupt approval storage overwritten")
	}
}

func TestCheckpointRejectsChangedWorktreeDuringTest(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not installed")
	}
	m, root, entry := checkpointFixture(t)
	ctx := context.Background()
	record, err := m.PrepareCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(entry.Path, "modified.txt"), []byte("modified"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = m.RunCheckpointTest(ctx, root, entry.ID, "task-1", "conversation-1", record.Revision, "go")
	if !errors.Is(err, ErrDirty) {
		t.Fatalf("dirty change must invalidate evidence: %v", err)
	}
}

func TestCheckpointSymlinkStorageRejected(t *testing.T) {
	m, root, entry := checkpointFixture(t)
	_, key, err := m.repository(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Dir(m.checkpointPath(key, entry.ID))
	if err = os.MkdirAll(filepath.Dir(directory), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(t.TempDir(), directory); err != nil {
		t.Skipf("symlink unsupported %v", err)
	}
	if _, err = m.PrepareCheckpoint(context.Background(), root, entry.ID, "task-1", "conversation-1"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("symlink checkpoint path accepted: %v", err)
	}
}

func TestTestProfilesCannotInjectParameters(t *testing.T) {
	for _, candidate := range []string{"go;rm -rf /", "go --run", "", "python -c 'x'", "--help"} {
		if _, ok := verifiedProfile(candidate); ok {
			t.Fatalf("unexpected profile %q", candidate)
		}
	}
	args, ok := verifiedProfile("go")
	if !ok || strings.Join(args, " ") != "go test -json -count=1 ./..." {
		t.Fatalf("invalid go profile: %v", args)
	}
}

func TestCheckpointZeroTestsDoNotPass(t *testing.T) {
	m, root, entry := checkpointFixture(t)
	if err := os.WriteFile(filepath.Join(entry.Path, "go.mod"), []byte("module sandbox.example/empty\n\ngo 1.23\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(entry.Path, "empty.go"), []byte("package empty\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, entry.Path, "add", "go.mod", "empty.go")
	gitTest(t, entry.Path, "-c", "user.name=Test", "-c", "user.email=test@example.org", "commit", "-m", "empty package")
	record, err := m.PrepareCheckpoint(context.Background(), root, entry.ID, "task-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	tested, err := m.RunCheckpointTest(context.Background(), root, entry.ID, "task-1", "conversation-1", record.Revision, "go")
	if err != nil {
		t.Fatal(err)
	}
	if tested.Tests[0].Passed {
		t.Fatalf("package without tests was accepted: %+v", tested.Tests)
	}
	if _, err := m.ApproveCheckpoint(context.Background(), root, entry.ID, "task-1", "conversation-1", record.Revision, "local-ui"); !errors.Is(err, ErrTestRequired) {
		t.Fatalf("empty suite approved: %v", err)
	}
}

func TestCheckpointLatestFailedTestInvalidatesPriorSuccess(t *testing.T) {
	m, root, entry := checkpointFixture(t)
	ctx := context.Background()
	record, err := m.PrepareCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	_, key, err := m.repository(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	record.Tests = []VerifiedTest{{Profile: "go", Revision: record.Revision, Passed: true}, {Profile: "go-race", Revision: record.Revision, Passed: false, ExitCode: 1}}
	if err = m.writeCheckpoint(key, entry.ID, *record); err != nil {
		t.Fatal(err)
	}
	if _, err = m.ApproveCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1", record.Revision, "local-ui"); !errors.Is(err, ErrTestRequired) {
		t.Fatalf("latest failure should block approval: %v", err)
	}
}

func TestCheckpointRemovedOnReleaseAndDeletion(t *testing.T) {
	m, root, entry := checkpointFixture(t)
	ctx := context.Background()
	_, err := m.PrepareCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	_, key, err := m.repository(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	path := m.checkpointPath(key, entry.ID)
	if err = m.Release(ctx, root, entry.ID, "task-1", "conversation-1"); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("release kept stale checkpoint: %v", err)
	}
	if _, err = m.Remove(ctx, root, entry.ID); err != nil {
		t.Fatal(err)
	}
}

func TestBoundedTestEvidenceTracksActualResultsWithoutUnboundedOutput(t *testing.T) {
	w := &boundedTestEvidence{digest: sha256.New()}
	input := []byte(`{"Time":"2026-01-01T00:00:00Z","Action":"pass","Package":"example.org/worker","Test":"TestReady","Elapsed":0.015}` + "\n")
	for _, fragment := range [][]byte{input[:3], input[3:15], input[15:]} {
		if n, err := w.Write(fragment); err != nil || n != len(fragment) {
			t.Fatalf("write failed: %d %v", n, err)
		}
	}
	if !w.sawGoTest || w.truncated {
		t.Fatalf("split Go result not detected: %+v", w)
	}
	if _, err := w.Write(bytes.Repeat([]byte("x"), maxTestEvidenceBytes+1024)); err != nil {
		t.Fatal(err)
	}
	if !w.truncated || len(w.tail) != maxTestExcerptBytes {
		t.Fatalf("unbounded/undetected output: tail=%d overflow=%v", len(w.tail), w.truncated)
	}
	if w.bytes != int64(len(input)+maxTestEvidenceBytes+1024) {
		t.Fatalf("incorrect byte count: %d", w.bytes)
	}
	if len(w.digest.Sum(nil)) != sha256.Size {
		t.Fatal("missing complete stream digest")
	}
}

func TestBoundedTestEvidenceHandlesLargeNewlineChunks(t *testing.T) {
	w := &boundedTestEvidence{digest: sha256.New()}
	large := append(bytes.Repeat([]byte("x"), maxTestExcerptBytes+9000), []byte("\n"+`{"Action":"pass","Package":"example.org/verified","Test":"TestReady"}`+"\n")...)
	if n, err := w.Write(large); err != nil || n != len(large) {
		t.Fatalf("large chunk failure: %d %v", n, err)
	}
	if !w.sawGoTest || len(w.tail) != maxTestExcerptBytes || len(w.line) > maxTestExcerptBytes {
		t.Fatalf("lost result or unbounded buffer: saw=%v tail=%d line=%d", w.sawGoTest, len(w.tail), len(w.line))
	}
	if !bytes.Equal(w.tail, large[len(large)-maxTestExcerptBytes:]) {
		t.Fatal("tail is not the last bytes of the original stream")
	}
}

func TestBoundedTestEvidenceSplitLongLineDoesNotSpoofResult(t *testing.T) {
	w := &boundedTestEvidence{digest: sha256.New()}
	if _, err := w.Write(bytes.Repeat([]byte("a"), maxTestExcerptBytes+1)); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(`{"Action":"pass","Package":"fake","Test":"Spoofed"}` + "\n")); err != nil {
		t.Fatal(err)
	}
	if w.sawGoTest || w.lineOverflow {
		t.Fatal("long line was incorrectly accepted as an independent result")
	}
	if _, err := w.Write([]byte(`{"Action":"pass","Package":"example.org/verified","Test":"TestReady"}` + "\n")); err != nil || !w.sawGoTest {
		t.Fatalf("valid next line not recovered: %v", err)
	}
}

func TestTestEvidenceRejectsPlainTextSpoofAndWrappedOutput(t *testing.T) {
	w := &boundedTestEvidence{digest: sha256.New()}
	for _, line := range []string{
		"ok  example.org/worker 0.015s\n",
		`{"Action":"output","Package":"example.org/worker","Output":"{\"Action\":\"pass\",\"Test\":\"Fake\",\"Package\":\"example.org/worker\"}\n"}` + "\n",
		`{"Action":"pass","Package":"example.org/worker"}` + "\n",
	} {
		if _, err := w.Write([]byte(line)); err != nil {
			t.Fatal(err)
		}
	}
	if w.sawGoTest {
		t.Fatal("text output or empty suite accepted as test execution")
	}
}

func TestCheckpointTestClearsInjectedGoFlags(t *testing.T) {
	m, root, entry := checkpointFixture(t)
	if err := os.WriteFile(filepath.Join(entry.Path, "go.mod"), []byte("module sandbox.example/injected\n\ngo 1.23\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(entry.Path, "injected_test.go"), []byte("package injected\nimport \"testing\"\nfunc TestMustExecute(t *testing.T) { t.Log(\"RUN_MARKER\") }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, entry.Path, "add", "go.mod", "injected_test.go")
	gitTest(t, entry.Path, "-c", "user.name=Test", "-c", "user.email=test@example.org", "commit", "-m", "added named test")
	checkpoint, err := m.PrepareCheckpoint(context.Background(), root, entry.ID, "task-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOFLAGS", "-run=^$")
	t.Setenv("GOWORK", filepath.Join(t.TempDir(), "missing.work"))
	result, err := m.RunCheckpointTest(context.Background(), root, entry.ID, "task-1", "conversation-1", checkpoint.Revision, "go")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tests) != 1 || !result.Tests[0].Passed {
		t.Fatalf("named test did not execute with isolated flags: %+v", result.Tests)
	}
}

func TestCheckpointPendingAndCancelledRunCannotApproveStaleSuccess(t *testing.T) {
	m, root, entry := checkpointFixture(t)
	if err := os.WriteFile(filepath.Join(entry.Path, "go.mod"), []byte("module sandbox.example/interrupt\n\ngo 1.23\n"), 0600); err != nil {
		t.Fatal(err)
	}
	content := `package interrupt
import (
 "testing"
 "time"
)
func TestSlow(t *testing.T) { time.Sleep(1200 * time.Millisecond) }
`
	if err := os.WriteFile(filepath.Join(entry.Path, "slow_test.go"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, entry.Path, "add", "go.mod", "slow_test.go")
	gitTest(t, entry.Path, "-c", "user.name=Test", "-c", "user.email=test@example.org", "commit", "-m", "slow test")
	ctx := context.Background()
	checkpoint, err := m.PrepareCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	first, err := m.RunCheckpointTest(ctx, root, entry.ID, "task-1", "conversation-1", checkpoint.Revision, "go")
	if err != nil || len(first.Tests) != 1 || !first.Tests[0].Passed {
		t.Fatalf("initial verified execution: %+v %v", first, err)
	}
	cancelledCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	type result struct {
		checkpoint *ReviewCheckpoint
		err        error
	}
	done := make(chan result, 1)
	go func() {
		value, err := m.RunCheckpointTest(cancelledCtx, root, entry.ID, "task-1", "conversation-1", checkpoint.Revision, "go")
		done <- result{value, err}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		current, err := m.ReadCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1")
		if err == nil && len(current.Tests) == 2 && !current.Tests[1].Passed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("running test did not persist fail-closed marker: %+v %v", current, err)
		}
		time.Sleep(25 * time.Millisecond)
	}
	if _, err := m.ApproveCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1", checkpoint.Revision, "reviewer"); !errors.Is(err, ErrTestRequired) {
		t.Fatalf("running test permitted old success to be approved: %v", err)
	}
	cancel()
	select {
	case result := <-done:
		if result.err != nil {
			t.Fatalf("cancellation evidence was not persisted: %v", result.err)
		}
		if len(result.checkpoint.Tests) != 2 || result.checkpoint.Tests[1].Passed || result.checkpoint.Tests[1].ExitCode != -2 {
			t.Fatalf("cancelled run was accepted: %+v", result.checkpoint.Tests)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("cancelled test did not terminate")
	}
	if _, err := m.ApproveCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1", checkpoint.Revision, "reviewer"); !errors.Is(err, ErrTestRequired) {
		t.Fatalf("cancelled test left stale approval evidence: %v", err)
	}
	recovered, err := New(m.dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	afterRestart, err := recovered.ReadCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1")
	if err != nil || len(afterRestart.Tests) != 2 || afterRestart.Tests[1].Passed {
		t.Fatalf("failed attempt did not persist over restart: %+v %v", afterRestart, err)
	}
	fresh, err := recovered.RunCheckpointTest(ctx, root, entry.ID, "task-1", "conversation-1", checkpoint.Revision, "go")
	if err != nil || len(fresh.Tests) != 3 || !fresh.Tests[2].Passed {
		t.Fatalf("verified re-run after interruption: %+v %v", fresh, err)
	}
	if _, err := recovered.ApproveCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1", checkpoint.Revision, "reviewer"); err != nil {
		t.Fatalf("new verified execution could not be approved: %v", err)
	}
}

func TestCheckpointReviewFencedByMergeLockAcrossManagers(t *testing.T) {
	m, root, entry := checkpointFixture(t)
	other, err := New(m.dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	unlock, err := m.lockRepositoryMerge(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = other.PrepareCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1")
	if !errors.Is(err, context.DeadlineExceeded) {
		unlock()
		t.Fatalf("checkpoint mutation ignored merge fence: %v", err)
	}
	unlock()
	checkpoint, err := other.PrepareCheckpoint(context.Background(), root, entry.ID, "task-1", "conversation-1")
	if err != nil || checkpoint.Revision == "" {
		t.Fatalf("checkpoint was not recovered after lock release: %+v %v", checkpoint, err)
	}
}

func TestCheckpointMissingTestToolchainPersistsFailure(t *testing.T) {
	m, root, entry := checkpointFixture(t)
	record, err := m.PrepareCheckpoint(context.Background(), root, entry.ID, "task-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	previousPath := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir())
	tested, err := m.RunCheckpointTest(context.Background(), root, entry.ID, "task-1", "conversation-1", record.Revision, "go")
	if err := os.Setenv("PATH", previousPath); err != nil {
		t.Fatal(err)
	}
	if err != nil || tested == nil || len(tested.Tests) != 1 || tested.Tests[0].Passed || tested.Tests[0].ExitCode == 0 {
		t.Fatalf("missing executable not saved as failed verification: %+v %v", tested, err)
	}
	if tested.Tests[0].OutputDigest == "" || !strings.Contains(tested.Tests[0].LogExcerpt, "unavailable") {
		t.Fatalf("missing executable has no useful failure evidence: %+v", tested.Tests[0])
	}
	restarted, err := New(m.dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := restarted.ReadCheckpoint(context.Background(), root, entry.ID, "task-1", "conversation-1")
	if err != nil || len(restored.Tests) != 1 || restored.Tests[0].Passed {
		t.Fatalf("missing toolchain failure not recovered: %+v %v", restored, err)
	}
	if _, err := restarted.ApproveCheckpoint(context.Background(), root, entry.ID, "task-1", "conversation-1", record.Revision, "reviewer"); !errors.Is(err, ErrTestRequired) {
		t.Fatalf("missing toolchain evidence allowed approval: %v", err)
	}
}
