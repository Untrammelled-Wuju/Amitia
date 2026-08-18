package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newBootstrapTicketTestRepo(t *testing.T) *BootstrapTicketRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&BootstrapTicket{}); err != nil {
		t.Fatalf("migrate tickets: %v", err)
	}
	return NewBootstrapTicketRepository(db)
}

func TestBootstrapTicketConsumeWithValidationIsOneTimeAndTicketOwnsIdentity(t *testing.T) {
	repo := newBootstrapTicketTestRepo(t)
	ctx := context.Background()
	raw, _, err := repo.Create(ctx, "user-ticket", "device-1", "runtime-1", time.Minute)
	if err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	consumed, err := repo.ConsumeWithValidation(ctx, raw, "runtime-1", "device-1")
	if err != nil {
		t.Fatalf("consume ticket: %v", err)
	}
	if consumed.UserID != "user-ticket" || consumed.DeviceID != "device-1" || consumed.RuntimeID != "runtime-1" {
		t.Fatalf("ticket identity changed: %#v", consumed)
	}
	if _, err := repo.ConsumeWithValidation(ctx, raw, "runtime-1", "device-1"); !errors.Is(err, ErrTicketConsumed) {
		t.Fatalf("second consumption expected ErrTicketConsumed, got %v", err)
	}
}

func TestBootstrapTicketRejectsDeviceAndRuntimeMismatchWithoutConsuming(t *testing.T) {
	repo := newBootstrapTicketTestRepo(t)
	ctx := context.Background()
	raw, _, err := repo.Create(ctx, "user-1", "device-1", "runtime-1", time.Minute)
	if err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	if _, err := repo.ConsumeWithValidation(ctx, raw, "runtime-1", "device-wrong"); !errors.Is(err, ErrTicketRevoked) {
		t.Fatalf("device mismatch expected rejection, got %v", err)
	}
	if _, err := repo.ConsumeWithValidation(ctx, raw, "runtime-wrong", "device-1"); !errors.Is(err, ErrTicketRevoked) {
		t.Fatalf("runtime mismatch expected rejection, got %v", err)
	}
	if _, err := repo.ConsumeWithValidation(ctx, raw, "runtime-1", "device-1"); err != nil {
		t.Fatalf("mismatch attempt must not consume valid ticket: %v", err)
	}
}

func TestBootstrapTicketRejectsMissingAndExpired(t *testing.T) {
	repo := newBootstrapTicketTestRepo(t)
	ctx := context.Background()
	if _, err := repo.ConsumeWithValidation(ctx, "", "runtime", "device"); !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("missing ticket expected ErrTicketNotFound, got %v", err)
	}
	raw, ticket, err := repo.Create(ctx, "u", "d", "r", time.Millisecond)
	if err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	time.Sleep(3 * time.Millisecond)
	if _, err := repo.ConsumeWithValidation(ctx, raw, "r", "d"); !errors.Is(err, ErrTicketExpired) {
		t.Fatalf("expired ticket expected ErrTicketExpired, got %v", err)
	}
	stored, err := repo.GetByID(ctx, ticket.ID)
	if err != nil {
		t.Fatalf("get expired ticket: %v", err)
	}
	if stored.Status != BootstrapTicketStatusExpired {
		t.Fatalf("expired status not persisted: %s", stored.Status)
	}
}
