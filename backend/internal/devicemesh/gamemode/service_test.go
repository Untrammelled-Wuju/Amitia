package gamemode

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setup(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "game-mode.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(db)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestGameModeIsExclusivePerSpace(t *testing.T) {
	service := setup(t)
	first, err := service.Acquire(t.Context(), AcquireInput{SpaceID: "space", DeviceID: "device-a", GamePluginID: "game-a"})
	if err != nil || first.DeviceID != "device-a" || first.State != StateActive {
		t.Fatalf("first acquire: %+v %v", first, err)
	}
	if _, err := service.Acquire(t.Context(), AcquireInput{SpaceID: "space", DeviceID: "device-b", GamePluginID: "game-b"}); !errors.Is(err, ErrOccupied) {
		t.Fatalf("second device acquired game mode: %v", err)
	}
	active, err := service.Update(t.Context(), "space", "device-a", first.SessionID, UpdateInput{State: StateActive, GameTitle: "Game A", ExpectedRevision: first.Revision})
	if err != nil || active.State != StateActive || active.GameTitle != "Game A" {
		t.Fatalf("activate: %+v %v", active, err)
	}
	visible, err := service.Get(t.Context(), "space")
	if err != nil || visible.DeviceID != "device-a" || visible.State != StateActive {
		t.Fatalf("visible state: %+v %v", visible, err)
	}
}

func TestOnlyOwnerReleasesUnlessForced(t *testing.T) {
	service := setup(t)
	current, err := service.Acquire(t.Context(), AcquireInput{SpaceID: "space", DeviceID: "device-a"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Release(t.Context(), "space", "device-b", current.SessionID, current.Revision, false); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("other device released lease: %v", err)
	}
	released, err := service.Release(t.Context(), "space", "", "", current.Revision, true)
	if err != nil || released.State != StateInactive {
		t.Fatalf("force release: %+v %v", released, err)
	}
	next, err := service.Acquire(t.Context(), AcquireInput{SpaceID: "space", DeviceID: "device-b"})
	if err != nil || next.DeviceID != "device-b" {
		t.Fatalf("next acquire: %+v %v", next, err)
	}
}

func TestConcurrentAcquireHasSingleWinner(t *testing.T) {
	service := setup(t)
	const devices = 16
	var wg sync.WaitGroup
	results := make(chan string, devices)
	for i := 0; i < devices; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			device := "device-" + string(rune('a'+index))
			_, err := service.Acquire(t.Context(), AcquireInput{SpaceID: "space", DeviceID: device})
			if err == nil {
				results <- device
			} else if !errors.Is(err, ErrOccupied) {
				t.Errorf("unexpected acquire error: %v", err)
			}
		}(i)
	}
	wg.Wait()
	close(results)
	winners := make([]string, 0, devices)
	for winner := range results {
		winners = append(winners, winner)
	}
	if len(winners) != 1 {
		t.Fatalf("expected one winner, got %v", winners)
	}
	current, err := service.Get(t.Context(), "space")
	if err != nil || current.DeviceID != winners[0] {
		t.Fatalf("stored winner mismatch: %+v %v", current, err)
	}
}

func TestInvalidTransitionFails(t *testing.T) {
	service := setup(t)
	current, err := service.Acquire(t.Context(), AcquireInput{SpaceID: "space", DeviceID: "device-a"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Update(t.Context(), "space", "device-a", current.SessionID, UpdateInput{State: StateInactive, ExpectedRevision: current.Revision}); !errors.Is(err, ErrState) {
		t.Fatalf("active -> inactive update should require release: %v", err)
	}
}

func TestLostDeviceMarksUnknownAndCannotAutomaticRelease(t *testing.T) {
	service := setup(t)
	current, err := service.Acquire(t.Context(), AcquireInput{SpaceID: "space", DeviceID: "device-a", GameTitle: "Game A"})
	if err != nil {
		t.Fatal(err)
	}
	unknown, err := service.MarkUnknown(t.Context(), "space", "device-a")
	if err != nil || unknown.State != StateUnknown || unknown.Revision != current.Revision+1 {
		t.Fatalf("mark unknown: %+v %v", unknown, err)
	}
	if _, err := service.Acquire(t.Context(), AcquireInput{SpaceID: "space", DeviceID: "device-b"}); !errors.Is(err, ErrOccupied) {
		t.Fatalf("unknown owner must keep the lease fenced: %v", err)
	}
	ready, err := service.MarkReady(t.Context(), "space", "device-a")
	if err != nil || ready.State != StateReconciling || ready.Revision != unknown.Revision+1 {
		t.Fatalf("mark ready: %+v %v", ready, err)
	}
}

func TestOwnerCanReleaseAfterRecoveryAdvancesRevision(t *testing.T) {
	service := setup(t)
	current, err := service.Acquire(t.Context(), AcquireInput{SpaceID: "space", DeviceID: "device-a", GameTitle: "Game A"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.MarkUnknown(t.Context(), "space", "device-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.MarkReady(t.Context(), "space", "device-a"); err != nil {
		t.Fatal(err)
	}
	released, err := service.Release(t.Context(), "space", "device-a", current.SessionID, current.Revision, false)
	if err != nil || released.State != StateInactive {
		t.Fatalf("owner could not release recovered lease: %+v %v", released, err)
	}
	repeated, err := service.Release(t.Context(), "space", "device-a", current.SessionID, current.Revision, false)
	if err != nil || repeated.State != StateInactive {
		t.Fatalf("repeated release is not idempotent: %+v %v", repeated, err)
	}
}

func TestStaleRevisionCannotOverwriteGameMode(t *testing.T) {
	service := setup(t)
	current, err := service.Acquire(t.Context(), AcquireInput{SpaceID: "space", DeviceID: "device-a", GameTitle: "Game A"})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := service.Update(t.Context(), "space", "device-a", current.SessionID, UpdateInput{State: StatePaused, GameTitle: "Game B", ExpectedRevision: current.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Update(t.Context(), "space", "device-a", current.SessionID, UpdateInput{State: StateActive, GameTitle: "Stale", ExpectedRevision: current.Revision}); !errors.Is(err, ErrRevision) {
		t.Fatalf("stale revision overwrote session: %v", err)
	}
	visible, err := service.Get(t.Context(), "space")
	if err != nil || visible.State != StatePaused || visible.GameTitle != "Game B" || visible.Revision != updated.Revision {
		t.Fatalf("session changed after stale update: %+v %v", visible, err)
	}
}

func TestGameModeSessionSurvivesServiceRestartAndStillFencesOtherDevices(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game-mode-restart.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(db)
	if err != nil {
		t.Fatal(err)
	}
	current, err := service.Acquire(t.Context(), AcquireInput{SpaceID: "space", DeviceID: "device-a", GameTitle: "Persisted Game"})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := NewService(reopened)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := restarted.Get(t.Context(), "space")
	if err != nil || persisted.SessionID != current.SessionID || persisted.DeviceID != "device-a" || persisted.GameTitle != "Persisted Game" {
		t.Fatalf("session was not restored: %+v %v", persisted, err)
	}
	if _, err := restarted.Acquire(t.Context(), AcquireInput{SpaceID: "space", DeviceID: "device-b"}); !errors.Is(err, ErrOccupied) {
		t.Fatalf("restart released the lease to another device: %v", err)
	}
}
