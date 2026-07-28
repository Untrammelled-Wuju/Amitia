package desktop

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func makeMenuDef(id, extID string) DesktopContributionDefinition {
	return DesktopContributionDefinition{
		ContributionID:  id,
		ExtensionID:     extID,
		ModuleID:        "test-module",
		DesktopType:     DesktopTypeMenuItem,
		ContractID:      "app.menu.item",
		ContractVersion: 1,
		Target:          "app.menu.file.extensions",
		Label:           LocalizedText{Default: "Test Menu"},
		Action:          DesktopActionBinding{ActionType: "host_action", TargetID: "action-" + id},
	}
}

func makeTrayDef(id, extID string) DesktopContributionDefinition {
	return DesktopContributionDefinition{
		ContributionID:  id,
		ExtensionID:     extID,
		ModuleID:        "test-module",
		DesktopType:     DesktopTypeTrayItem,
		ContractID:      "app.tray.item",
		ContractVersion: 1,
		Target:          "tray.quick_actions",
		Label:           LocalizedText{Default: "Test Tray"},
		Action:          DesktopActionBinding{ActionType: "host_action", TargetID: "tray-" + id},
	}
}

func makeShortcutDef(id, extID, accel string) DesktopContributionDefinition {
	return DesktopContributionDefinition{
		ContributionID:  id,
		ExtensionID:     extID,
		ModuleID:        "test-module",
		DesktopType:     DesktopTypeAppShortcut,
		ContractID:      "app.shortcut.application",
		ContractVersion: 1,
		Target:          "window.focused",
		Label:           LocalizedText{Default: "Test Shortcut"},
		Action:          DesktopActionBinding{ActionType: "host_action", TargetID: "sc-" + id},
		Shortcut:        &DesktopShortcutDefinition{Accelerator: accel},
	}
}

func TestNormalizeAccelerator(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"lowercase_single", "ctrl+shift+a", "Ctrl+Shift+A"},
		{"already_normalized", "Ctrl+Shift+A", "Ctrl+Shift+A"},
		{"cmdorcontrol", "CmdOrCtrl+P", "CmdOrCtrl+P"},
		{"control_alias", "control+shift+a", "Ctrl+Shift+A"},
		{"uppercase_input", "CTRL+SHIFT+A", "Ctrl+Shift+A"},
		{"cmd_alias", "cmd+p", "Cmd+P"},
		{"command_alias", "command+p", "Cmd+P"},
		{"commandorcontrol_alias", "commandorcontrol+p", "CmdOrCtrl+P"},
		{"option_alias", "option+p", "Alt+P"},
		{"alt_alias", "alt+p", "Alt+P"},
		{"super_alias", "super+p", "Super+P"},
		{"meta_alias", "meta+p", "Super+P"},
		{"single_char_uppercase", "ctrl+a", "Ctrl+A"},
		{"function_key_kept", "ctrl+f1", "Ctrl+f1"},
		{"spaces_trimmed", "ctrl + shift + a", "Ctrl+Shift+A"},
		{"order_preserved", "shift+ctrl+a", "Shift+Ctrl+A"},
		{"three_parts", "cmdorctrl+shift+p", "CmdOrCtrl+Shift+P"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeAccelerator(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeAccelerator(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateAccelerator(t *testing.T) {
	tests := []struct {
		name         string
		accel        string
		wantValid    bool
		wantReserved bool
	}{
		{"empty", "", false, false},
		{"only_modifier", "Ctrl", false, false},
		{"modifiers_only", "Ctrl+Shift", false, false},
		{"no_modifier_single_key", "A", false, false},
		{"invalid_key", "Ctrl+xyz", false, false},
		{"valid_simple", "Ctrl+P", true, false},
		{"valid_cmdorctrl", "CmdOrCtrl+Shift+P", true, false},
		{"valid_alt_function", "Alt+F1", true, false},
		{"valid_three_mods", "CmdOrCtrl+Alt+Shift+D", true, false},
		{"reserved_quit", "CmdOrCtrl+Q", false, true},
		{"reserved_close", "CmdOrCtrl+W", false, true},
		{"reserved_new", "CmdOrCtrl+N", false, true},
		{"reserved_devtools", "CmdOrCtrl+Shift+I", false, true},
		{"reserved_devtools_alt", "CmdOrCtrl+Alt+I", false, true},
		{"reserved_altf4", "Alt+F4", false, true},
		{"reserved_reload", "CmdOrCtrl+R", false, true},
		{"reserved_commandorcontrol_h", "CommandOrControl+Shift+H", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := ValidateAccelerator(tt.accel)
			if r.Valid != tt.wantValid {
				t.Errorf("ValidateAccelerator(%q) Valid = %v, want %v, reason=%s", tt.accel, r.Valid, tt.wantValid, r.Reason)
			}
			if r.IsReserved != tt.wantReserved {
				t.Errorf("ValidateAccelerator(%q) IsReserved = %v, want %v", tt.accel, r.IsReserved, tt.wantReserved)
			}
			if r.Valid {
				if r.Normalized == "" {
					t.Errorf("ValidateAccelerator(%q) valid but Normalized empty", tt.accel)
				}
			}
		})
	}
}

func TestValidateAccelerator_NormalizedField(t *testing.T) {
	r := ValidateAccelerator("ctrl+shift+p")
	if !r.Valid {
		t.Fatalf("expected valid, reason=%s", r.Reason)
	}
	if r.Normalized != "Ctrl+Shift+P" {
		t.Errorf("Normalized = %q, want %q", r.Normalized, "Ctrl+Shift+P")
	}
}

func TestIsShortcutReserved(t *testing.T) {
	if !IsShortcutReserved("CmdOrCtrl+Q") {
		t.Error("CmdOrCtrl+Q should be reserved")
	}
	if !IsShortcutReserved("CmdOrCtrl+W") {
		t.Error("CmdOrCtrl+W should be reserved")
	}
	if IsShortcutReserved("Ctrl+P") {
		t.Error("Ctrl+P should not be reserved")
	}
	if IsShortcutReserved("Ctrl+Shift+K") {
		t.Error("Ctrl+Shift+K should not be reserved")
	}
}

func TestAcceleratorsConflict(t *testing.T) {
	if !AcceleratorsConflict("Ctrl+P", "ctrl+p") {
		t.Error("expected conflict for same accel different case")
	}
	if !AcceleratorsConflict("Ctrl+P", "control+p") {
		t.Error("expected conflict for control alias")
	}
	if !AcceleratorsConflict("CmdOrCtrl+P", "cmdorctrl+p") {
		t.Error("expected conflict for cmdorctrl alias")
	}
	if AcceleratorsConflict("Ctrl+P", "Ctrl+Shift+P") {
		t.Error("expected no conflict for different accels")
	}
	if AcceleratorsConflict("Ctrl+P", "Ctrl+A") {
		t.Error("expected no conflict for different keys")
	}
}

func TestConflictResolver_DetectShortcutConflict(t *testing.T) {
	cr := NewConflictResolver()
	existing := makeShortcutDef("sc-1", "ext-1", "Ctrl+P")
	newDef := makeShortcutDef("sc-2", "ext-2", "Ctrl+P")
	record := cr.DetectShortcutConflict(&existing, &newDef)
	if record == nil {
		t.Fatal("expected conflict record, got nil")
	}
	if record.Type != ConflictTypeShortcut {
		t.Errorf("Type = %s, want %s", record.Type, ConflictTypeShortcut)
	}
	if record.Severity != ConflictSeverityBlock {
		t.Errorf("Severity = %s, want %s", record.Severity, ConflictSeverityBlock)
	}
	if record.Accelerator != "Ctrl+P" {
		t.Errorf("Accelerator = %s, want Ctrl+P", record.Accelerator)
	}
	if record.ExistingContribID != "sc-1" {
		t.Errorf("ExistingContribID = %s, want sc-1", record.ExistingContribID)
	}
	if record.NewContribID != "sc-2" {
		t.Errorf("NewContribID = %s, want sc-2", record.NewContribID)
	}
}

func TestConflictResolver_DetectShortcutConflict_None(t *testing.T) {
	cr := NewConflictResolver()
	existing := makeShortcutDef("sc-1", "ext-1", "Ctrl+P")
	newDef := makeShortcutDef("sc-2", "ext-2", "Ctrl+Shift+P")
	record := cr.DetectShortcutConflict(&existing, &newDef)
	if record != nil {
		t.Errorf("expected nil for different accelerators, got %v", record)
	}
}

func TestConflictResolver_DetectShortcutConflict_NilShortcut(t *testing.T) {
	cr := NewConflictResolver()
	menu := makeMenuDef("m-1", "ext-1")
	def := makeShortcutDef("sc-1", "ext-1", "Ctrl+P")
	if record := cr.DetectShortcutConflict(&menu, &def); record != nil {
		t.Errorf("expected nil when existing shortcut nil, got %v", record)
	}
	if record := cr.DetectShortcutConflict(&def, &menu); record != nil {
		t.Errorf("expected nil when new shortcut nil, got %v", record)
	}
}

func TestConflictResolver_DetectMenuIDConflict(t *testing.T) {
	cr := NewConflictResolver()
	existing := makeMenuDef("m-1", "ext-1")
	newSameID := makeMenuDef("m-1", "ext-2")
	record := cr.DetectMenuIDConflict(&existing, &newSameID)
	if record == nil {
		t.Fatal("expected conflict for same contribution id")
	}
	if record.Type != ConflictTypeMenuID {
		t.Errorf("Type = %s, want %s", record.Type, ConflictTypeMenuID)
	}
	if record.Severity != ConflictSeverityBlock {
		t.Errorf("Severity = %s, want %s", record.Severity, ConflictSeverityBlock)
	}
}

func TestConflictResolver_DetectActionIDConflict(t *testing.T) {
	cr := NewConflictResolver()
	existing := makeMenuDef("m-1", "ext-1")
	existing.Action.TargetID = "shared-action"
	newDef := makeMenuDef("m-2", "ext-2")
	newDef.Action.TargetID = "shared-action"
	record := cr.DetectMenuIDConflict(&existing, &newDef)
	if record == nil {
		t.Fatal("expected conflict for same action target id")
	}
	if record.Type != ConflictTypeActionID {
		t.Errorf("Type = %s, want %s", record.Type, ConflictTypeActionID)
	}
}

func TestConflictResolver_DetectMenuIDConflict_None(t *testing.T) {
	cr := NewConflictResolver()
	existing := makeMenuDef("m-1", "ext-1")
	newDef := makeMenuDef("m-2", "ext-2")
	if record := cr.DetectMenuIDConflict(&existing, &newDef); record != nil {
		t.Errorf("expected nil for different ids and target ids, got %v", record)
	}
}

func TestConflictResolver_ResolveAndList(t *testing.T) {
	cr := NewConflictResolver()
	existing := makeShortcutDef("sc-1", "ext-1", "Ctrl+P")
	newDef := makeShortcutDef("sc-2", "ext-2", "Ctrl+P")
	record := cr.DetectShortcutConflict(&existing, &newDef)

	unresolved := cr.ListUnresolved()
	if len(unresolved) != 1 {
		t.Errorf("unresolved count = %d, want 1", len(unresolved))
	}

	if err := cr.Resolve(record.ConflictID, "manual"); err != nil {
		t.Fatalf("Resolve error: %v", err)
	}

	got, ok := cr.Get(record.ConflictID)
	if !ok {
		t.Fatal("conflict not found after resolve")
	}
	if !got.Resolved {
		t.Error("expected resolved = true")
	}
	if got.Resolution != "manual" {
		t.Errorf("Resolution = %s, want manual", got.Resolution)
	}
	if got.ResolvedAt == nil {
		t.Error("expected non-nil ResolvedAt")
	}
	if len(cr.ListUnresolved()) != 0 {
		t.Error("expected 0 unresolved after resolve")
	}
	if len(cr.ListAll()) != 1 {
		t.Errorf("ListAll count = %d, want 1", len(cr.ListAll()))
	}
}

func TestConflictResolver_ResolveInvalid(t *testing.T) {
	cr := NewConflictResolver()
	err := cr.Resolve("nonexistent", "x")
	if !errors.Is(err, ErrInvalidConflictResolution) {
		t.Errorf("expected ErrInvalidConflictResolution, got %v", err)
	}
}

func TestConflictResolver_GetNotFound(t *testing.T) {
	cr := NewConflictResolver()
	if _, ok := cr.Get("nonexistent"); ok {
		t.Error("expected false for nonexistent conflict")
	}
}

func TestConflictResolver_ClearByExtension(t *testing.T) {
	cr := NewConflictResolver()
	existing := makeShortcutDef("sc-1", "ext-1", "Ctrl+P")
	newDef := makeShortcutDef("sc-2", "ext-2", "Ctrl+P")
	cr.DetectShortcutConflict(&existing, &newDef)
	if len(cr.ListAll()) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(cr.ListAll()))
	}
	cr.ClearByExtension("ext-1")
	if len(cr.ListAll()) != 0 {
		t.Errorf("expected 0 conflicts after clear, got %d", len(cr.ListAll()))
	}
}

func TestConflictResolver_ClearByExtension_BothSides(t *testing.T) {
	cr := NewConflictResolver()
	existing := makeShortcutDef("sc-1", "ext-1", "Ctrl+P")
	newDef := makeShortcutDef("sc-2", "ext-2", "Ctrl+P")
	cr.DetectShortcutConflict(&existing, &newDef)
	cr.ClearByExtension("ext-2")
	if len(cr.ListAll()) != 0 {
		t.Errorf("expected 0 conflicts after clearing new side, got %d", len(cr.ListAll()))
	}
}

func TestConflictResolver_ListAllEmpty(t *testing.T) {
	cr := NewConflictResolver()
	if len(cr.ListAll()) != 0 {
		t.Errorf("expected 0 conflicts, got %d", len(cr.ListAll()))
	}
	if len(cr.ListUnresolved()) != 0 {
		t.Errorf("expected 0 unresolved, got %d", len(cr.ListUnresolved()))
	}
}

func TestDesktopHost_RegisterMenuContribution(t *testing.T) {
	h := NewDesktopHost()
	ctx := context.Background()
	def := makeMenuDef("m-1", "ext-1")
	resolved, err := h.RegisterContribution(ctx, def)
	if err != nil {
		t.Fatalf("RegisterContribution error: %v", err)
	}
	if resolved.Status != ContributionStatusRegistered {
		t.Errorf("Status = %s, want %s", resolved.Status, ContributionStatusRegistered)
	}
	if resolved.EffectiveLabel != "Test Menu" {
		t.Errorf("EffectiveLabel = %s, want Test Menu", resolved.EffectiveLabel)
	}
	if resolved.Generation <= 0 {
		t.Errorf("Generation = %d, want > 0", resolved.Generation)
	}
	if resolved.ResolvedAt.IsZero() {
		t.Error("ResolvedAt should not be zero")
	}
	got, ok := h.GetContribution("m-1")
	if !ok {
		t.Fatal("GetContribution returned false")
	}
	if got.Definition.ContributionID != "m-1" {
		t.Errorf("ContributionID = %s, want m-1", got.Definition.ContributionID)
	}
}

func TestDesktopHost_RegisterTrayContribution(t *testing.T) {
	h := NewDesktopHost()
	ctx := context.Background()
	def := makeTrayDef("t-1", "ext-1")
	resolved, err := h.RegisterContribution(ctx, def)
	if err != nil {
		t.Fatalf("RegisterContribution error: %v", err)
	}
	if resolved.Status != ContributionStatusRegistered {
		t.Errorf("Status = %s, want %s", resolved.Status, ContributionStatusRegistered)
	}
	if resolved.EffectiveLabel != "Test Tray" {
		t.Errorf("EffectiveLabel = %s, want Test Tray", resolved.EffectiveLabel)
	}
}

func TestDesktopHost_RegisterShortcutContribution(t *testing.T) {
	h := NewDesktopHost()
	ctx := context.Background()
	def := makeShortcutDef("s-1", "ext-1", "Ctrl+P")
	resolved, err := h.RegisterContribution(ctx, def)
	if err != nil {
		t.Fatalf("RegisterContribution error: %v", err)
	}
	if resolved.Status != ContributionStatusRegistered {
		t.Errorf("Status = %s, want %s", resolved.Status, ContributionStatusRegistered)
	}
	if resolved.EffectiveLabel != "Test Shortcut" {
		t.Errorf("EffectiveLabel = %s, want Test Shortcut", resolved.EffectiveLabel)
	}
	res, ok := h.GetContribution("s-1")
	if !ok {
		t.Fatal("GetContribution returned false")
	}
	if res.Definition.Shortcut == nil {
		t.Fatal("Shortcut should not be nil")
	}
	if res.Definition.Shortcut.Accelerator != "Ctrl+P" {
		t.Errorf("Accelerator = %s, want Ctrl+P", res.Definition.Shortcut.Accelerator)
	}
}

func TestDesktopHost_RegisterDuplicateID(t *testing.T) {
	h := NewDesktopHost()
	ctx := context.Background()
	def := makeMenuDef("m-1", "ext-1")
	if _, err := h.RegisterContribution(ctx, def); err != nil {
		t.Fatalf("first RegisterContribution error: %v", err)
	}
	_, err := h.RegisterContribution(ctx, def)
	if !errors.Is(err, ErrContributionExists) {
		t.Errorf("expected ErrContributionExists, got %v", err)
	}
	if !strings.Contains(err.Error(), "m-1") {
		t.Errorf("error message should contain id m-1, got %s", err.Error())
	}
}

func TestDesktopHost_RegisterInvalidDefinition(t *testing.T) {
	h := NewDesktopHost()
	ctx := context.Background()
	tests := []struct {
		name string
		def  DesktopContributionDefinition
	}{
		{"empty_contribution_id", DesktopContributionDefinition{ExtensionID: "ext", ModuleID: "mod"}},
		{"empty_extension_id", DesktopContributionDefinition{ContributionID: "c", ModuleID: "mod"}},
		{"empty_module_id", DesktopContributionDefinition{ContributionID: "c", ExtensionID: "ext"}},
		{"empty_desktop_type", DesktopContributionDefinition{ContributionID: "c", ExtensionID: "ext", ModuleID: "mod", ContractID: "app.menu.item", ContractVersion: 1, Target: "app.menu.file.extensions"}},
		{"empty_contract_id", DesktopContributionDefinition{ContributionID: "c", ExtensionID: "ext", ModuleID: "mod", DesktopType: DesktopTypeMenuItem, ContractVersion: 1, Target: "app.menu.file.extensions"}},
		{"zero_contract_version", DesktopContributionDefinition{ContributionID: "c", ExtensionID: "ext", ModuleID: "mod", DesktopType: DesktopTypeMenuItem, ContractID: "app.menu.item", Target: "app.menu.file.extensions"}},
		{"empty_target", DesktopContributionDefinition{ContributionID: "c", ExtensionID: "ext", ModuleID: "mod", DesktopType: DesktopTypeMenuItem, ContractID: "app.menu.item", ContractVersion: 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := h.RegisterContribution(ctx, tt.def)
			if !errors.Is(err, ErrInvalidDefinition) {
				t.Errorf("expected ErrInvalidDefinition, got %v", err)
			}
		})
	}
}

func TestDesktopHost_RegisterInvalidActionType(t *testing.T) {
	h := NewDesktopHost()
	ctx := context.Background()
	def := makeMenuDef("m-1", "ext-1")
	def.Action = DesktopActionBinding{ActionType: "invalid_type"}
	_, err := h.RegisterContribution(ctx, def)
	if !errors.Is(err, ErrInvalidActionType) {
		t.Errorf("expected ErrInvalidActionType, got %v", err)
	}
}

func TestDesktopHost_RegisterForbiddenActionType(t *testing.T) {
	h := NewDesktopHost()
	ctx := context.Background()
	def := makeMenuDef("m-1", "ext-1")
	def.Action = DesktopActionBinding{ActionType: "raw_ipc"}
	_, err := h.RegisterContribution(ctx, def)
	if !errors.Is(err, ErrForbiddenActionType) {
		t.Errorf("expected ErrForbiddenActionType, got %v", err)
	}
}

func TestDesktopHost_RegisterInvalidTarget(t *testing.T) {
	h := NewDesktopHost()
	ctx := context.Background()
	def := makeMenuDef("m-1", "ext-1")
	def.Target = "invalid.target"
	_, err := h.RegisterContribution(ctx, def)
	if !errors.Is(err, ErrInvalidMenuTarget) {
		t.Errorf("expected ErrInvalidMenuTarget, got %v", err)
	}
}

func TestDesktopHost_RegisterShortcutConflict(t *testing.T) {
	h := NewDesktopHost()
	ctx := context.Background()
	def1 := makeShortcutDef("s-1", "ext-1", "Ctrl+P")
	if _, err := h.RegisterContribution(ctx, def1); err != nil {
		t.Fatalf("first register error: %v", err)
	}
	def2 := makeShortcutDef("s-2", "ext-2", "Ctrl+P")
	resolved, err := h.RegisterContribution(ctx, def2)
	if err != nil {
		t.Fatalf("second register error: %v", err)
	}
	if resolved.Status != ContributionStatusConflict {
		t.Errorf("Status = %s, want %s", resolved.Status, ContributionStatusConflict)
	}
	if !strings.Contains(resolved.ConflictReason, "already used") {
		t.Errorf("ConflictReason = %s, want contains 'already used'", resolved.ConflictReason)
	}
	if !strings.Contains(resolved.ConflictReason, "s-1") {
		t.Errorf("ConflictReason = %s, want contains 's-1'", resolved.ConflictReason)
	}
	conflicts := h.GetConflicts().ListAll()
	if len(conflicts) == 0 {
		t.Error("expected conflict records in resolver")
	}
	if _, ok := h.GetContribution("s-1"); !ok {
		t.Error("first contribution should still exist")
	}
	if _, ok := h.GetContribution("s-2"); !ok {
		t.Error("second contribution should still exist despite conflict")
	}
}

func TestDesktopHost_RegisterShortcutConflict_DifferentCase(t *testing.T) {
	h := NewDesktopHost()
	ctx := context.Background()
	def1 := makeShortcutDef("s-1", "ext-1", "Ctrl+P")
	h.RegisterContribution(ctx, def1)
	def2 := makeShortcutDef("s-2", "ext-2", "ctrl+p")
	resolved, err := h.RegisterContribution(ctx, def2)
	if err != nil {
		t.Fatalf("register error: %v", err)
	}
	if resolved.Status != ContributionStatusConflict {
		t.Errorf("Status = %s, want %s (case-insensitive conflict)", resolved.Status, ContributionStatusConflict)
	}
}

func TestDesktopHost_RegisterReservedShortcut(t *testing.T) {
	h := NewDesktopHost()
	ctx := context.Background()
	def := makeShortcutDef("s-1", "ext-1", "CmdOrCtrl+Q")
	resolved, err := h.RegisterContribution(ctx, def)
	if err != nil {
		t.Fatalf("register error: %v", err)
	}
	if resolved.Status != ContributionStatusConflict {
		t.Errorf("Status = %s, want %s", resolved.Status, ContributionStatusConflict)
	}
	if !strings.Contains(resolved.ConflictReason, "reserved shortcut") {
		t.Errorf("ConflictReason = %s, want contains 'reserved shortcut'", resolved.ConflictReason)
	}
	conflicts := h.GetConflicts().ListAll()
	if len(conflicts) == 0 {
		t.Error("expected conflict record for reserved shortcut")
	}
}

func TestDesktopHost_RegisterReservedShortcut_AltF4(t *testing.T) {
	h := NewDesktopHost()
	ctx := context.Background()
	def := makeShortcutDef("s-1", "ext-1", "Alt+F4")
	resolved, err := h.RegisterContribution(ctx, def)
	if err != nil {
		t.Fatalf("register error: %v", err)
	}
	if resolved.Status != ContributionStatusConflict {
		t.Errorf("Status = %s, want %s", resolved.Status, ContributionStatusConflict)
	}
}

func TestDesktopHost_RegisterInvalidShortcut(t *testing.T) {
	h := NewDesktopHost()
	ctx := context.Background()
	def := makeShortcutDef("s-1", "ext-1", "xyz")
	resolved, err := h.RegisterContribution(ctx, def)
	if err != nil {
		t.Fatalf("register error: %v", err)
	}
	if resolved.Status != ContributionStatusUnsupported {
		t.Errorf("Status = %s, want %s", resolved.Status, ContributionStatusUnsupported)
	}
	if resolved.ConflictReason == "" {
		t.Error("expected non-empty ConflictReason")
	}
}

func TestDesktopHost_RegisterModifierOnlyShortcut(t *testing.T) {
	h := NewDesktopHost()
	ctx := context.Background()
	def := makeShortcutDef("s-1", "ext-1", "Ctrl+Shift")
	resolved, err := h.RegisterContribution(ctx, def)
	if err != nil {
		t.Fatalf("register error: %v", err)
	}
	if resolved.Status != ContributionStatusUnsupported {
		t.Errorf("Status = %s, want %s", resolved.Status, ContributionStatusUnsupported)
	}
}

func TestDesktopHost_RegisterActionIDConflict(t *testing.T) {
	h := NewDesktopHost()
	ctx := context.Background()
	def1 := makeMenuDef("m-1", "ext-1")
	def1.Action.TargetID = "shared-action"
	if _, err := h.RegisterContribution(ctx, def1); err != nil {
		t.Fatalf("first register error: %v", err)
	}
	def2 := makeMenuDef("m-2", "ext-2")
	def2.Action.TargetID = "shared-action"
	resolved, err := h.RegisterContribution(ctx, def2)
	if err != nil {
		t.Fatalf("second register error: %v", err)
	}
	if resolved.Status != ContributionStatusConflict {
		t.Errorf("Status = %s, want %s", resolved.Status, ContributionStatusConflict)
	}
	if !strings.Contains(resolved.ConflictReason, "conflict") {
		t.Errorf("ConflictReason = %s, want contains 'conflict'", resolved.ConflictReason)
	}
}

func TestDesktopHost_UnregisterContribution(t *testing.T) {
	h := NewDesktopHost()
	ctx := context.Background()
	def := makeShortcutDef("s-1", "ext-1", "Ctrl+P")
	if _, err := h.RegisterContribution(ctx, def); err != nil {
		t.Fatalf("register error: %v", err)
	}
	if err := h.UnregisterContribution("s-1"); err != nil {
		t.Fatalf("unregister error: %v", err)
	}
	if _, ok := h.GetContribution("s-1"); ok {
		t.Error("expected contribution to be removed")
	}
	if len(h.ListContributions()) != 0 {
		t.Errorf("ListContributions count = %d, want 0", len(h.ListContributions()))
	}
	if len(h.ListResources()) != 0 {
		t.Errorf("resources count = %d, want 0 after unregister", len(h.ListResources()))
	}
}

func TestDesktopHost_UnregisterNotFound(t *testing.T) {
	h := NewDesktopHost()
	err := h.UnregisterContribution("nonexistent")
	if !errors.Is(err, ErrContributionNotFound) {
		t.Errorf("expected ErrContributionNotFound, got %v", err)
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should contain id, got %s", err.Error())
	}
}

func TestDesktopHost_UnregisterReleasesShortcut(t *testing.T) {
	h := NewDesktopHost()
	ctx := context.Background()
	def1 := makeShortcutDef("s-1", "ext-1", "Ctrl+P")
	if _, err := h.RegisterContribution(ctx, def1); err != nil {
		t.Fatalf("first register error: %v", err)
	}
	if err := h.UnregisterContribution("s-1"); err != nil {
		t.Fatalf("unregister error: %v", err)
	}
	def2 := makeShortcutDef("s-2", "ext-2", "Ctrl+P")
	resolved, err := h.RegisterContribution(ctx, def2)
	if err != nil {
		t.Fatalf("re-register same accel error: %v", err)
	}
	if resolved.Status != ContributionStatusRegistered {
		t.Errorf("Status = %s, want %s after unregistering previous", resolved.Status, ContributionStatusRegistered)
	}
}

func TestDesktopHost_UnregisterByExtension(t *testing.T) {
	h := NewDesktopHost()
	ctx := context.Background()
	h.RegisterContribution(ctx, makeMenuDef("m-1", "ext-1"))
	h.RegisterContribution(ctx, makeMenuDef("m-2", "ext-1"))
	h.RegisterContribution(ctx, makeMenuDef("m-3", "ext-2"))
	count := h.UnregisterByExtension("ext-1")
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
	if _, ok := h.GetContribution("m-1"); ok {
		t.Error("m-1 should be removed")
	}
	if _, ok := h.GetContribution("m-2"); ok {
		t.Error("m-2 should be removed")
	}
	if _, ok := h.GetContribution("m-3"); !ok {
		t.Error("m-3 should remain")
	}
	if len(h.ListByExtension("ext-1")) != 0 {
		t.Errorf("ListByExtension(ext-1) count = %d, want 0", len(h.ListByExtension("ext-1")))
	}
}

func TestDesktopHost_BuildSnapshot(t *testing.T) {
	h := NewDesktopHost()
	ctx := context.Background()
	h.RegisterContribution(ctx, makeMenuDef("m-1", "ext-1"))
	h.RegisterContribution(ctx, makeMenuDef("m-2", "ext-1"))
	h.RegisterContribution(ctx, makeTrayDef("t-1", "ext-1"))
	h.RegisterContribution(ctx, makeShortcutDef("s-1", "ext-1", "Ctrl+P"))
	sortCtx := SortContext{}
	snapshot := h.BuildSnapshot(sortCtx)
	if snapshot == nil {
		t.Fatal("expected snapshot, got nil")
	}
	if len(snapshot.Contributions) != 4 {
		t.Errorf("Contributions count = %d, want 4", len(snapshot.Contributions))
	}
	if len(snapshot.MenuTree) == 0 {
		t.Error("expected non-empty menu tree")
	}
	menuItems, ok := snapshot.MenuTree["app.menu.file.extensions"]
	if !ok {
		t.Fatal("expected menu tree entry for app.menu.file.extensions")
	}
	if len(menuItems) < 2 {
		t.Errorf("menu items count = %d, want >= 2", len(menuItems))
	}
	if len(snapshot.TrayTree) == 0 {
		t.Error("expected non-empty tray tree")
	}
	trayItems, ok := snapshot.TrayTree["tray.quick_actions"]
	if !ok {
		t.Fatal("expected tray tree entry for tray.quick_actions")
	}
	if len(trayItems) != 1 {
		t.Errorf("tray items count = %d, want 1", len(trayItems))
	}
	if len(snapshot.Shortcuts) != 1 {
		t.Errorf("Shortcuts count = %d, want 1", len(snapshot.Shortcuts))
	}
	if snapshot.Hash == "" {
		t.Error("expected non-empty hash")
	}
	if snapshot.Generation <= 0 {
		t.Errorf("Generation = %d, want > 0", snapshot.Generation)
	}
	if snapshot.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestDesktopHost_BuildSnapshotWithConflicts(t *testing.T) {
	h := NewDesktopHost()
	ctx := context.Background()
	h.RegisterContribution(ctx, makeShortcutDef("s-1", "ext-1", "Ctrl+P"))
	h.RegisterContribution(ctx, makeShortcutDef("s-2", "ext-2", "Ctrl+P"))
	snap := h.BuildSnapshot(SortContext{})
	if len(snap.Conflicts) == 0 {
		t.Error("expected conflicts in snapshot")
	}
	if len(snap.Shortcuts) != 1 {
		t.Errorf("Shortcuts count = %d, want 1 (only registered)", len(snap.Shortcuts))
	}
	if len(snap.Contributions) != 2 {
		t.Errorf("Contributions count = %d, want 2", len(snap.Contributions))
	}
}

func TestDesktopHost_BuildSnapshotEmpty(t *testing.T) {
	h := NewDesktopHost()
	snap := h.BuildSnapshot(SortContext{})
	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if len(snap.Contributions) != 0 {
		t.Errorf("Contributions count = %d, want 0", len(snap.Contributions))
	}
	if len(snap.MenuTree) != 0 {
		t.Errorf("MenuTree count = %d, want 0", len(snap.MenuTree))
	}
	if snap.Hash == "" {
		t.Error("expected non-empty hash even for empty snapshot")
	}
}

func TestDesktopHost_GetSnapshot(t *testing.T) {
	h := NewDesktopHost()
	if h.GetSnapshot() != nil {
		t.Error("expected nil snapshot before BuildSnapshot")
	}
	h.RegisterContribution(context.Background(), makeMenuDef("m-1", "ext-1"))
	snap := h.BuildSnapshot(SortContext{})
	if h.GetSnapshot() == nil {
		t.Error("expected non-nil snapshot after BuildSnapshot")
	}
	if h.GetSnapshot() != snap {
		t.Error("GetSnapshot should return same snapshot pointer")
	}
}

func TestDesktopHost_ListContributions(t *testing.T) {
	h := NewDesktopHost()
	ctx := context.Background()
	h.RegisterContribution(ctx, makeMenuDef("m-1", "ext-1"))
	h.RegisterContribution(ctx, makeMenuDef("m-2", "ext-2"))
	list := h.ListContributions()
	if len(list) != 2 {
		t.Errorf("ListContributions count = %d, want 2", len(list))
	}
	byExt := h.ListByExtension("ext-1")
	if len(byExt) != 1 {
		t.Errorf("ListByExtension count = %d, want 1", len(byExt))
	}
	byTarget := h.ListByTarget("app.menu.file.extensions")
	if len(byTarget) != 2 {
		t.Errorf("ListByTarget count = %d, want 2", len(byTarget))
	}
	byTargetOther := h.ListByTarget("invalid.target")
	if len(byTargetOther) != 0 {
		t.Errorf("ListByTarget(invalid) count = %d, want 0", len(byTargetOther))
	}
}

func TestDesktopHost_MaxItemsPerExtension(t *testing.T) {
	h := NewDesktopHost()
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		def := makeTrayDef(fmt.Sprintf("t-%d", i), "ext-1")
		if _, err := h.RegisterContribution(ctx, def); err != nil {
			t.Fatalf("register %d error: %v", i, err)
		}
	}
	def := makeTrayDef("t-overflow", "ext-1")
	_, err := h.RegisterContribution(ctx, def)
	if !errors.Is(err, ErrTooManyItems) {
		t.Errorf("expected ErrTooManyItems, got %v", err)
	}
	defOther := makeTrayDef("t-other", "ext-2")
	if _, err := h.RegisterContribution(ctx, defOther); err != nil {
		t.Errorf("other extension should not be limited, got error: %v", err)
	}
}

func TestDesktopHost_Resources(t *testing.T) {
	h := NewDesktopHost()
	ctx := context.Background()
	h.RegisterContribution(ctx, makeMenuDef("m-1", "ext-1"))
	h.RegisterContribution(ctx, makeMenuDef("m-2", "ext-2"))
	resources := h.ListResources()
	if len(resources) != 2 {
		t.Errorf("ListResources count = %d, want 2", len(resources))
	}
	byExt := h.ListResourcesByExtension("ext-1")
	if len(byExt) != 1 {
		t.Errorf("ListResourcesByExtension count = %d, want 1", len(byExt))
	}
	h.UnregisterContribution("m-1")
	if len(h.ListResources()) != 1 {
		t.Errorf("after unregister, ListResources count = %d, want 1", len(h.ListResources()))
	}
}

func TestDesktopHost_ResourcesNotTrackedForConflicted(t *testing.T) {
	h := NewDesktopHost()
	ctx := context.Background()
	h.RegisterContribution(ctx, makeShortcutDef("s-1", "ext-1", "Ctrl+P"))
	h.RegisterContribution(ctx, makeShortcutDef("s-2", "ext-2", "Ctrl+P"))
	if len(h.ListResources()) != 1 {
		t.Errorf("ListResources count = %d, want 1 (only registered)", len(h.ListResources()))
	}
}

type deniedPermissionChecker struct{}

func (deniedPermissionChecker) Check(context.Context, string, string) (bool, error) {
	return false, nil
}

func TestDesktopHost_GlobalShortcutWaitsForPermission(t *testing.T) {
	host := NewDesktopHost()
	host.SetPermissionChecker(deniedPermissionChecker{})
	definition := makeShortcutDef("global-permission", "ext-permission", "CmdOrCtrl+Alt+P")
	definition.DesktopType = DesktopTypeGlobalShortcut
	definition.ContractID = "app.shortcut.global"
	definition.Target = "global"
	definition.Shortcut.Global = true
	resolved, err := host.RegisterContribution(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Status != ContributionStatusPendingPermission {
		t.Fatalf("expected pending_permission, got %s", resolved.Status)
	}
}

func TestDesktopHostRecordsApplyResult(t *testing.T) {
	host := NewDesktopHost()
	host.RecordApplyReport(DesktopApplyReport{Generation: 7, Hash: "snapshot-hash", Success: true})
	report, ok := host.GetApplyReport(7)
	if !ok || !report.Success || report.AppliedAt.IsZero() {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestDesktopContractRegistry_Builtins(t *testing.T) {
	r := NewDesktopContractRegistry()
	if !r.IsTargetAllowed("app.menu.item", 1, "app.menu.file.extensions") {
		t.Error("expected target allowed for menu item")
	}
	if r.IsTargetAllowed("app.menu.item", 1, "invalid.target") {
		t.Error("expected target not allowed")
	}
	if r.MaxItemsPerExtension("app.menu.item", 1) != 20 {
		t.Errorf("MaxItemsPerExtension = %d, want 20", r.MaxItemsPerExtension("app.menu.item", 1))
	}
	if r.MaxItemsPerExtension("app.tray.item", 1) != 5 {
		t.Errorf("MaxItemsPerExtension = %d, want 5", r.MaxItemsPerExtension("app.tray.item", 1))
	}
	contracts := r.ListContracts()
	if len(contracts) == 0 {
		t.Error("expected builtin contracts")
	}
}

func TestDesktopContractRegistry_NotFound(t *testing.T) {
	r := NewDesktopContractRegistry()
	ctx := context.Background()
	if r.IsTargetAllowed("nonexistent", 1, "any") {
		t.Error("expected false for nonexistent contract")
	}
	if r.MaxItemsPerExtension("nonexistent", 1) != 0 {
		t.Error("expected 0 for nonexistent contract")
	}
	if _, err := r.GetContract(ctx, "nonexistent", 1); !errors.Is(err, ErrContractNotFound) {
		t.Errorf("expected ErrContractNotFound, got %v", err)
	}
}

func TestDesktopContractRegistry_GetContractVersionNotFound(t *testing.T) {
	r := NewDesktopContractRegistry()
	ctx := context.Background()
	if _, err := r.GetContract(ctx, "app.menu.item", 99); !errors.Is(err, ErrContractVersionNotFound) {
		t.Errorf("expected ErrContractVersionNotFound, got %v", err)
	}
}

func TestDesktopContractRegistry_RegisterContract(t *testing.T) {
	r := NewDesktopContractRegistry()
	ctx := context.Background()
	def := DesktopContractDefinition{
		ContractID:     "custom.contract",
		Version:        1,
		DesktopType:    DesktopTypeMenuItem,
		AllowedTargets: []string{"custom.target"},
		Status:         ContractStatusActive,
		MaxItemsPerExt: 10,
	}
	if err := r.RegisterContract(ctx, def); err != nil {
		t.Fatalf("RegisterContract error: %v", err)
	}
	if !r.IsTargetAllowed("custom.contract", 1, "custom.target") {
		t.Error("expected target allowed for custom contract")
	}
	if r.MaxItemsPerExtension("custom.contract", 1) != 10 {
		t.Errorf("MaxItemsPerExtension = %d, want 10", r.MaxItemsPerExtension("custom.contract", 1))
	}
}

func TestDesktopContractRegistry_RegisterContract_InvalidID(t *testing.T) {
	r := NewDesktopContractRegistry()
	ctx := context.Background()
	def := DesktopContractDefinition{Version: 1}
	if err := r.RegisterContract(ctx, def); !errors.Is(err, ErrInvalidContractID) {
		t.Errorf("expected ErrInvalidContractID, got %v", err)
	}
}

func TestDesktopContractRegistry_RegisterContract_ZeroVersion(t *testing.T) {
	r := NewDesktopContractRegistry()
	ctx := context.Background()
	def := DesktopContractDefinition{ContractID: "test", Version: 0}
	if err := r.RegisterContract(ctx, def); !errors.Is(err, ErrInvalidContractID) {
		t.Errorf("expected ErrInvalidContractID, got %v", err)
	}
}

func TestActionBinding_Validate(t *testing.T) {
	tests := []struct {
		name      string
		action    DesktopActionBinding
		wantError error
	}{
		{"empty_action_type", DesktopActionBinding{ActionType: ""}, ErrInvalidActionType},
		{"invalid_type", DesktopActionBinding{ActionType: "invalid"}, ErrInvalidActionType},
		{"forbidden_raw_ipc", DesktopActionBinding{ActionType: "raw_ipc"}, ErrForbiddenActionType},
		{"forbidden_electron", DesktopActionBinding{ActionType: "electron_call"}, ErrForbiddenActionType},
		{"forbidden_shell", DesktopActionBinding{ActionType: "shell"}, ErrForbiddenActionType},
		{"forbidden_raw_http", DesktopActionBinding{ActionType: "raw_http"}, ErrForbiddenActionType},
		{"forbidden_raw_sql", DesktopActionBinding{ActionType: "raw_sql"}, ErrForbiddenActionType},
		{"forbidden_file_path", DesktopActionBinding{ActionType: "file_path_open"}, ErrForbiddenActionType},
		{"valid_host_action", DesktopActionBinding{ActionType: "host_action"}, nil},
		{"valid_tool_invoke", DesktopActionBinding{ActionType: "tool_invoke"}, nil},
		{"valid_workflow", DesktopActionBinding{ActionType: "workflow_execute"}, nil},
		{"valid_task", DesktopActionBinding{ActionType: "task_enqueue"}, nil},
		{"valid_command", DesktopActionBinding{ActionType: "extension_command"}, nil},
		{"valid_navigation", DesktopActionBinding{ActionType: "navigation"}, nil},
		{"valid_dialog", DesktopActionBinding{ActionType: "dialog_open"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.action.Validate()
			if tt.wantError == nil {
				if err != nil {
					t.Errorf("expected nil error, got %v", err)
				}
			} else {
				if !errors.Is(err, tt.wantError) {
					t.Errorf("expected %v, got %v", tt.wantError, err)
				}
			}
		})
	}
}

func TestLocalizedText_Get(t *testing.T) {
	lt := LocalizedText{
		Default:      "Hello",
		Translations: map[string]string{"zh": "你好", "ja": "こんにちは"},
	}
	if got := lt.Get("zh"); got != "你好" {
		t.Errorf("Get(zh) = %s, want 你好", got)
	}
	if got := lt.Get("ja"); got != "こんにちは" {
		t.Errorf("Get(ja) = %s, want こんにちは", got)
	}
	if got := lt.Get("en"); got != "Hello" {
		t.Errorf("Get(en) = %s, want Hello (default)", got)
	}
	empty := LocalizedText{Default: "Default"}
	if got := empty.Get("zh"); got != "Default" {
		t.Errorf("Get(zh) with no translations = %s, want Default", got)
	}
}
