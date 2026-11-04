package devicemesh_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/devicemesh"
	"github.com/u-ai/backend/internal/devicemesh/server"
	"github.com/u-ai/backend/internal/deviceruntime"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
)

func meshAuthorityDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "mesh.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := devicemesh.EnsureSchema(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func newCloudRuntimeForTest(t *testing.T, db *sql.DB, registry *host_registry.Registry) *devicemesh.Runtime {
	t.Helper()
	sessionStore := deviceruntime.NewSQLiteSessionStore(db)
	if err := sessionStore.EnsureSchema(t.Context()); err != nil {
		t.Fatal(err)
	}
	sessions, err := deviceruntime.NewService(sessionStore, deviceruntime.ServiceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := devicemesh.NewCloudRuntimeWithHubAndSessions(db, registry, server.NewConnectionHub(), sessions)
	if err != nil {
		t.Fatal(err)
	}
	return runtime
}
