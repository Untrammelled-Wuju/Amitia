package agentrun

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/migration"
	"gorm.io/gorm"
)

func openProcessQADB(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := connection.Close(); err != nil {
			t.Error(err)
		}
	})
	return db
}

func TestAgentRunQAProcessHelper(t *testing.T) {
	stage := os.Getenv("AMITIA_QA_AGENTRUN_STAGE")
	if stage == "" {
		t.Skip("only executed by TestAgentRunProcessCrashQA")
	}
	db := openProcessQADB(t, os.Getenv("AMITIA_QA_AGENTRUN_DB"))
	service, err := NewService(db)
	if err != nil {
		t.Fatal(err)
	}
	claim, err := service.Claim(t.Context(), "space-a", ChildRunID("assignment-a"), stage, 5*time.Minute, 10)
	if err != nil || len(claim.Messages) != 1 {
		t.Fatalf("claim after process transition: %+v %v", claim, err)
	}
	if stage == "claim-crash" {
		os.Exit(23)
	}
	if stage != "recover-ack" {
		t.Fatalf("unknown process stage: %s", stage)
	}
	if claim.Messages[0].DeliveryAttempts != 2 {
		t.Fatalf("incorrect recovery attempts: %+v", claim.Messages[0])
	}
	if _, err := service.Acknowledge(t.Context(), "space-a", claim.RunID, claim.LeaseToken, []string{claim.Messages[0].ID}); err != nil {
		t.Fatal(err)
	}
}

func TestAgentRunProcessCrashQA(t *testing.T) {
	path := filepath.Join(t.TempDir(), "crash-recovery.db")
	db := openProcessQADB(t, path)
	if err := migration.ApplyBaseline(db); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(db)
	if err != nil {
		t.Fatal(err)
	}
	seedCoordination(t, service)
	message, err := service.Send(t.Context(), SendInput{
		SpaceID: "space-a", SenderRunID: RootRunID("coordination-a"), RecipientID: ChildRunID("assignment-a"),
		Kind: MessageInstruction, Content: "QA-process-crash-durable-message",
	})
	if err != nil {
		t.Fatal(err)
	}
	run := func(stage string, expectedExit int) {
		t.Helper()
		command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestAgentRunQAProcessHelper$")
		command.Env = append(os.Environ(), "AMITIA_QA_AGENTRUN_STAGE="+stage, "AMITIA_QA_AGENTRUN_DB="+path)
		output, err := command.CombinedOutput()
		if expectedExit == 0 && err == nil {
			return
		}
		var exit *exec.ExitError
		if expectedExit != 0 && errors.As(err, &exit) && exit.ExitCode() == expectedExit {
			return
		}
		t.Fatalf("stage=%s expectedExit=%d error=%v output=%s", stage, expectedExit, err, output)
	}
	run("claim-crash", 23)
	leased, err := service.Claim(t.Context(), "space-a", ChildRunID("assignment-a"), "must-not-duplicate", time.Minute, 10)
	if err != nil || len(leased.Messages) != 0 {
		t.Fatalf("process exit invalidated a committed lease: %+v %v", leased, err)
	}
	if err := db.Model(&Message{}).Where("id = ?", message.ID).Update("lease_expires_at", time.Now().UTC().Add(-time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	run("recover-ack", 0)
	after, err := service.Claim(t.Context(), "space-a", ChildRunID("assignment-a"), "must-not-redeliver", time.Minute, 10)
	if err != nil || len(after.Messages) != 0 {
		t.Fatalf("acknowledged message redelivered after process restart: %+v %v", after, err)
	}
	var stored Message
	if err := db.First(&stored, "id = ?", message.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Content != message.Content || stored.DeliveryAttempts != 2 || stored.DeliveryState != DeliveryAcked || stored.AcknowledgedAt == nil {
		t.Fatalf("durable business result incorrect: %+v", stored)
	}
}
