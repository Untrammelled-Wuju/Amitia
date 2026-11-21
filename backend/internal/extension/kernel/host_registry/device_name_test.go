package host_registry

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

func newDeviceNameTestRegistry(t *testing.T) *Registry {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "kernel.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE kernel_devices (
		device_id TEXT PRIMARY KEY,
		space_id TEXT NOT NULL,
		platform TEXT NOT NULL DEFAULT '',
		label TEXT NOT NULL DEFAULT '',
		reported_name TEXT NOT NULL DEFAULT '',
		pending_name TEXT NOT NULL DEFAULT '',
		name_revision INTEGER NOT NULL DEFAULT 0,
		trust_state TEXT NOT NULL DEFAULT 'pending',
		created_at TEXT NOT NULL,
		trusted_at TEXT,
		last_seen_at TEXT NOT NULL,
		revision INTEGER NOT NULL DEFAULT 1
	)`)
	if err != nil {
		t.Fatal(err)
	}
	return NewRegistry(db)
}

func TestEnsureDeviceUsesSystemNameAsFirstLabel(t *testing.T) {
	registry := newDeviceNameTestRegistry(t)
	record, err := registry.EnsureDevice(t.Context(), DeviceRecord{
		SpaceID:  runtimeidentity.ParseSpaceID("space-a"),
		DeviceID: runtimeidentity.ParseDeviceID("device-a"),
		Label:    " MacBook Pro ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.Label != "MacBook Pro" || record.ReportedName != "MacBook Pro" || record.NameRevision != 1 {
		t.Fatalf("首次命名没有使用设备自报名: %+v", record)
	}
}

func TestReportDeviceNameRequiresConfirmation(t *testing.T) {
	registry := newDeviceNameTestRegistry(t)
	space := runtimeidentity.ParseSpaceID("space-a")
	device := runtimeidentity.ParseDeviceID("device-a")
	if _, err := registry.EnsureDevice(t.Context(), DeviceRecord{SpaceID: space, DeviceID: device, Label: "MacBook Pro"}); err != nil {
		t.Fatal(err)
	}
	reported, err := registry.ReportDeviceName(t.Context(), device, "红米K50")
	if err != nil {
		t.Fatal(err)
	}
	if reported.Label != "MacBook Pro" {
		t.Fatalf("未确认的改名改动了中心显示名: %+v", reported)
	}
	if reported.PendingName != "红米K50" || reported.ReportedName != "红米K50" {
		t.Fatalf("改名没有进入待确认状态: %+v", reported)
	}
	confirmed, err := registry.ConfirmDeviceName(t.Context(), space, device)
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.Label != "红米K50" || confirmed.PendingName != "" || confirmed.NameRevision != 2 {
		t.Fatalf("确认后设备名不正确: %+v", confirmed)
	}
}

func TestReportDeviceNameClearsPendingWhenReverted(t *testing.T) {
	registry := newDeviceNameTestRegistry(t)
	space := runtimeidentity.ParseSpaceID("space-a")
	device := runtimeidentity.ParseDeviceID("device-a")
	if _, err := registry.EnsureDevice(t.Context(), DeviceRecord{SpaceID: space, DeviceID: device, Label: "MacBook Pro"}); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.ReportDeviceName(t.Context(), device, "Old Name"); err != nil {
		t.Fatal(err)
	}
	reverted, err := registry.ReportDeviceName(t.Context(), device, "MacBook Pro")
	if err != nil {
		t.Fatal(err)
	}
	if reverted.PendingName != "" {
		t.Fatalf("设备改回原名称后仍保留待确认: %+v", reverted)
	}
	if _, err := registry.ConfirmDeviceName(t.Context(), space, device); err != ErrDeviceNameNotPending {
		t.Fatalf("没有待确认改名时应拒绝确认, got %v", err)
	}
}

func TestConfirmDeviceNameRejectsOtherSpace(t *testing.T) {
	registry := newDeviceNameTestRegistry(t)
	device := runtimeidentity.ParseDeviceID("device-a")
	if _, err := registry.EnsureDevice(t.Context(), DeviceRecord{SpaceID: runtimeidentity.ParseSpaceID("space-a"), DeviceID: device, Label: "MacBook Pro"}); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.ReportDeviceName(t.Context(), device, "红米K50"); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.ConfirmDeviceName(t.Context(), runtimeidentity.ParseSpaceID("space-b"), device); err != ErrDeviceOwnedByOther {
		t.Fatalf("其他空间不能确认本空间设备改名, got %v", err)
	}
}

func TestNormalizeDeviceNameBoundsAndStripsControlCharacters(t *testing.T) {
	if got := NormalizeDeviceName("  MacBook\u0000  Pro\n"); got != "MacBook Pro" {
		t.Fatalf("控制字符或重复空白未归一: %q", got)
	}
	long := make([]rune, 0, MaxDeviceNameRunes+20)
	for i := 0; i < MaxDeviceNameRunes+20; i++ {
		long = append(long, '名')
	}
	if got := NormalizeDeviceName(string(long)); len([]rune(got)) != MaxDeviceNameRunes {
		t.Fatalf("设备名长度未截断: %d", len([]rune(got)))
	}
	if got := NormalizeDeviceName("   "); got != "" {
		t.Fatalf("空白设备名应为空: %q", got)
	}
}
