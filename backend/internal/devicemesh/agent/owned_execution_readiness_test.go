package agent

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/executionjournal"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestOwnedExecutionReadinessRejectsTypedNilRoutesAndMissingJournalSchema(t *testing.T) {
	h := NewLocalHandler(t.TempDir(), runtimeidentity.PlatformWindows)
	t.Cleanup(h.Stop)
	h.localCoreID = "core"
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "readiness.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	h.executionGuard = NewOwnedToolGuard(db, h.dataDir)
	h.executionJournal = executionjournal.NewStore(db)
	dispatcher := NewRuntimeDispatcher()
	handler := func(context.Context, protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
		return nil, nil
	}
	dispatcher.RegisterCancellable("coordination.data", handler)
	dispatcher.RegisterCancellable("task.host.source-event", handler)
	h.dispatcher = dispatcher
	if err := h.ValidateOwnedExecutionWiring(); err == nil {
		t.Fatal("missing durable tables accepted")
	}
	for _, schema := range []string{executionjournal.Schema, executionjournal.FenceSchema} {
		if _, err := db.Exec(schema); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.ValidateOwnedExecutionWiring(); err != nil {
		t.Fatal(err)
	}
	foreign, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "foreign-readiness.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = foreign.Close() })
	for _, schema := range []string{executionjournal.Schema, executionjournal.FenceSchema} {
		if _, err := foreign.Exec(schema); err != nil {
			t.Fatal(err)
		}
	}
	h.executionJournal = executionjournal.NewStore(foreign)
	if err := h.ValidateOwnedExecutionWiring(); err != nil {
		t.Fatal("foreign fixture did not have complete journal schema", err)
	}
	if err := h.ValidateJournalDatabaseWiring(db); err == nil {
		t.Fatal("complete foreign journal authority accepted")
	}
	h.executionJournal = executionjournal.NewStore(db)
	if err := h.ValidateJournalDatabaseWiring(db); err != nil {
		t.Fatal(err)
	}
	var absent *defaultRuntimeDispatcher
	h.dispatcher = absent
	if err := h.ValidateOwnedExecutionWiring(); err == nil {
		t.Fatal("typed-nil routes accepted")
	}
	h.dispatcher = dispatcher
	h.executionJournal = executionjournal.NewStore(nil)
	if err := h.ValidateOwnedExecutionWiring(); err == nil {
		t.Fatal("journal without database accepted")
	}
	h.executionJournal = executionjournal.NewStore(db)
	h.localCoreID = ""
	if err := h.ValidateOwnedExecutionWiring(); err == nil {
		t.Fatal("missing actual Core identity accepted")
	}
	h.localCoreID = "core"
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := h.ValidateOwnedExecutionWiring(); err == nil {
		t.Fatal("closed durable journal database accepted")
	}
}
