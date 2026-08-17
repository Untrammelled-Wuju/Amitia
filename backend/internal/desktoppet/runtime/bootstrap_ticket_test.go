// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtime

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestTicketDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&BootstrapTicket{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestBootstrapTicket_MissingTicket(t *testing.T) {
	db := newTestTicketDB(t)
	repo := NewBootstrapTicketRepository(db)

	_, err := repo.ConsumeWithValidation(context.Background(), "", "runtime-1", "device-1")
	if err != ErrTicketNotFound {
		t.Errorf("missing ticket should return ErrTicketNotFound, got: %v", err)
	}
}

func TestBootstrapTicket_ConsumedTicket(t *testing.T) {
	db := newTestTicketDB(t)
	repo := NewBootstrapTicketRepository(db)

	raw, _, err := repo.Create(context.Background(), "user-1", "device-1", "runtime-1", time.Hour)
	if err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	_, err = repo.ConsumeWithValidation(context.Background(), raw, "runtime-1", "device-1")
	if err != nil {
		t.Fatalf("first consume should succeed: %v", err)
	}

	_, err = repo.ConsumeWithValidation(context.Background(), raw, "runtime-1", "device-1")
	if err != ErrTicketConsumed {
		t.Errorf("consumed ticket should return ErrTicketConsumed, got: %v", err)
	}
}

func TestBootstrapTicket_DeviceMismatch(t *testing.T) {
	db := newTestTicketDB(t)
	repo := NewBootstrapTicketRepository(db)

	raw, _, err := repo.Create(context.Background(), "user-1", "device-1", "runtime-1", time.Hour)
	if err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	_, err = repo.ConsumeWithValidation(context.Background(), raw, "runtime-1", "device-2")
	if err != ErrTicketRevoked {
		t.Errorf("device mismatch should return ErrTicketRevoked, got: %v", err)
	}
}

func TestBootstrapTicket_RuntimeMismatch(t *testing.T) {
	db := newTestTicketDB(t)
	repo := NewBootstrapTicketRepository(db)

	raw, _, err := repo.Create(context.Background(), "user-1", "device-1", "runtime-1", time.Hour)
	if err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	_, err = repo.ConsumeWithValidation(context.Background(), raw, "runtime-2", "device-1")
	if err != ErrTicketRevoked {
		t.Errorf("runtime mismatch should return ErrTicketRevoked, got: %v", err)
	}
}

func TestBootstrapTicket_ExpiredTicket(t *testing.T) {
	db := newTestTicketDB(t)
	repo := NewBootstrapTicketRepository(db)

	raw, _, err := repo.Create(context.Background(), "user-1", "device-1", "runtime-1", -time.Hour)
	if err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	_, err = repo.ConsumeWithValidation(context.Background(), raw, "runtime-1", "device-1")
	if err != ErrTicketExpired {
		t.Errorf("expired ticket should return ErrTicketExpired, got: %v", err)
	}
}

func TestBootstrapTicket_RevokedTicket(t *testing.T) {
	db := newTestTicketDB(t)
	repo := NewBootstrapTicketRepository(db)

	_, ticket, err := repo.Create(context.Background(), "user-1", "device-1", "runtime-1", time.Hour)
	if err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	err = repo.UpdateStatus(context.Background(), ticket.ID, BootstrapTicketStatusRevoked, "test-revoke")
	if err != nil {
		t.Fatalf("revoke ticket: %v", err)
	}

	raw2, _, err := repo.Create(context.Background(), "user-1", "device-1", "runtime-1", time.Hour)
	if err != nil {
		t.Fatalf("create second ticket: %v", err)
	}

	_, err = repo.ConsumeWithValidation(context.Background(), raw2, "runtime-1", "device-1")
	if err != nil {
		t.Errorf("valid ticket should succeed, got: %v", err)
	}
}

func TestBootstrapTicket_ValidConsume(t *testing.T) {
	db := newTestTicketDB(t)
	repo := NewBootstrapTicketRepository(db)

	raw, _, err := repo.Create(context.Background(), "user-1", "device-1", "runtime-1", time.Hour)
	if err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	ticket, err := repo.ConsumeWithValidation(context.Background(), raw, "runtime-1", "device-1")
	if err != nil {
		t.Fatalf("valid consume should succeed: %v", err)
	}

	if ticket.Status != BootstrapTicketStatusConsumed {
		t.Errorf("status should be consumed, got: %s", ticket.Status)
	}
	if ticket.ConsumedByRuntime != "runtime-1" {
		t.Errorf("consumed by runtime should be runtime-1, got: %s", ticket.ConsumedByRuntime)
	}
}

func TestBootstrapTicket_CorruptedExpiresAt(t *testing.T) {
	db := newTestTicketDB(t)
	repo := NewBootstrapTicketRepository(db)

	raw, ticket, err := repo.Create(context.Background(), "user-1", "device-1", "runtime-1", time.Hour)
	if err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	db.Model(&BootstrapTicket{}).Where("id = ?", ticket.ID).Update("expires_at", "invalid-timestamp")

	_, err = repo.ConsumeWithValidation(context.Background(), raw, "runtime-1", "device-1")
	if err != ErrTicketExpired {
		t.Errorf("corrupted expires_at should return ErrTicketExpired, got: %v", err)
	}
}

func TestBootstrapTicket_NewTicketAfterConsume(t *testing.T) {
	db := newTestTicketDB(t)
	repo := NewBootstrapTicketRepository(db)

	raw1, _, err := repo.Create(context.Background(), "user-1", "device-1", "runtime-1", time.Hour)
	if err != nil {
		t.Fatalf("create first ticket: %v", err)
	}

	_, err = repo.ConsumeWithValidation(context.Background(), raw1, "runtime-1", "device-1")
	if err != nil {
		t.Fatalf("consume first ticket: %v", err)
	}

	raw2, _, err := repo.Create(context.Background(), "user-1", "device-1", "runtime-1", time.Hour)
	if err != nil {
		t.Fatalf("create second ticket: %v", err)
	}

	ticket, err := repo.ConsumeWithValidation(context.Background(), raw2, "runtime-1", "device-1")
	if err != nil {
		t.Errorf("new ticket should succeed, got: %v", err)
	}
	if ticket.Status != BootstrapTicketStatusConsumed {
		t.Errorf("status should be consumed, got: %s", ticket.Status)
	}
}

func TestBootstrapTicket_WrongUserTicket(t *testing.T) {
	db := newTestTicketDB(t)
	repo := NewBootstrapTicketRepository(db)

	raw, _, err := repo.Create(context.Background(), "user-1", "device-1", "runtime-1", time.Hour)
	if err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	_, ticket2, err := repo.Create(context.Background(), "user-2", "device-2", "runtime-2", time.Hour)
	if err != nil {
		t.Fatalf("create second ticket: %v", err)
	}
	_ = ticket2

	_, err = repo.ConsumeWithValidation(context.Background(), raw, "runtime-2", "device-2")
	if err != ErrTicketRevoked {
		t.Errorf("wrong user ticket should return ErrTicketRevoked, got: %v", err)
	}
}

func TestBootstrapTicket_RevokeUserTickets(t *testing.T) {
	db := newTestTicketDB(t)
	repo := NewBootstrapTicketRepository(db)

	for i := 0; i < 3; i++ {
		_, _, err := repo.Create(context.Background(), "user-1", "device-1", "runtime-1", time.Hour)
		if err != nil {
			t.Fatalf("create ticket %d: %v", i, err)
		}
	}

	affected, err := repo.RevokeUserTickets(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("revoke user tickets: %v", err)
	}
	if affected != 3 {
		t.Errorf("should revoke 3 tickets, got: %d", affected)
	}
}

func TestBootstrapTicket_ExpireOldTickets(t *testing.T) {
	db := newTestTicketDB(t)
	repo := NewBootstrapTicketRepository(db)

	_, ticket, err := repo.Create(context.Background(), "user-1", "device-1", "runtime-1", time.Hour)
	if err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	db.Model(&BootstrapTicket{}).Where("id = ?", ticket.ID).Update("expires_at", time.Now().Add(-time.Hour).UTC().Format(time.RFC3339))

	affected, err := repo.ExpireOldTickets(context.Background())
	if err != nil {
		t.Fatalf("expire old tickets: %v", err)
	}
	if affected != 1 {
		t.Errorf("should expire 1 ticket, got: %d", affected)
	}
}
