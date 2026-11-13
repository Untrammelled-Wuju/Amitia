// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package dataportability

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadRestoreTicketWithoutPendingRestoreIsInvalidNotIO(t *testing.T) {
	manager := NewStagingManager(t.TempDir())
	ticket, err := manager.ReadRestoreTicket()
	if err == nil || ticket != nil {
		t.Fatalf("missing ticket must fail: %+v %v", ticket, err)
	}
	if !errors.Is(err, ErrRestoreInvalid) {
		t.Fatalf("missing ticket must map to RESTORE_BACKUP_INVALID: %v", err)
	}
	if !errors.Is(err, ErrRestoreNotPending) {
		t.Fatalf("missing ticket must stay distinguishable from corrupt tickets: %v", err)
	}
	if strings.Contains(err.Error(), ".restore") || strings.Contains(err.Error(), string(os.PathSeparator)) {
		t.Fatalf("missing ticket must not expose local paths: %v", err)
	}
}

func TestReadRestoreTicketKeepsCorruptTicketCause(t *testing.T) {
	manager := NewStagingManager(t.TempDir())
	directory := filepath.Join(manager.BaseDir, ".restore")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "ticket.json"), []byte("{not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := manager.ReadRestoreTicket()
	if !errors.Is(err, ErrRestoreInvalid) {
		t.Fatalf("corrupt ticket must map to RESTORE_BACKUP_INVALID: %v", err)
	}
	if !strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("corrupt ticket must keep the original parse error: %v", err)
	}
}
