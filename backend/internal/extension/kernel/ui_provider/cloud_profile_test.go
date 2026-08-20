package ui_provider

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func newCloudProfileTestRegistry(t *testing.T) *Registry {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "ui-profile.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := NewSQLiteProfileStore(db)
	if err := store.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	r := NewRegistryWithBuiltins()
	if err := r.AttachStore(context.Background(), store); err != nil {
		t.Fatal(err)
	}
	return r
}

func schemaProvider(id string, placement Placement, fallback string, priority int) ProviderDefinition {
	return ProviderDefinition{
		ProviderID: id, ExtensionID: "test.cloud-ui", ModuleID: id,
		Capability: CapabilityConversationShell, Mode: ModeReplace, Priority: priority,
		Entries: map[string]Entry{
			"android": {Type: EntrySchemaRenderer, ContributionID: id + ".schema"},
		},
		FallbackProviderID: fallback,
		Placement:          placement,
		Enabled:            true,
	}
}

func TestScopedProfilesResolvePerUserAndDevice(t *testing.T) {
	ctx := context.Background()
	r := newCloudProfileTestRegistry(t)
	cloud := schemaProvider("test.cloud", PlacementCloud, "", 10)
	device := schemaProvider("test.device", PlacementDevice, "test.cloud", 20)
	device.DeviceRequirements = &DeviceRequirements{RequiredFeatures: []string{"native-ui"}}
	if err := r.Register(cloud); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(device); err != nil {
		t.Fatal(err)
	}

	user1 := ResolveContext{UserID: "user-1", Platform: "android"}
	if _, err := r.SetProfileForContext(ctx, user1, ProfileScopeUser, Profile{
		ProfileID: "user-1", Name: "User 1", Selections: map[Capability]string{CapabilityConversationShell: "test.cloud"},
	}, 0); err != nil {
		t.Fatal(err)
	}

	device1 := ResolveContext{
		UserID: "user-1", DeviceID: "phone-1", Platform: "android", DeviceOnline: true,
		DeviceCapabilities: []string{"native-ui"},
	}
	if _, err := r.SetProfileForContext(ctx, device1, ProfileScopeDevice, Profile{
		ProfileID: "phone-1", Name: "Phone 1", Selections: map[Capability]string{CapabilityConversationShell: "test.device"},
	}, 0); err != nil {
		t.Fatal(err)
	}

	if got := r.ResolveWithContext(ctx, CapabilityConversationShell, device1).Provider; got == nil || got.ProviderID != "test.device" {
		t.Fatalf("device override not resolved: %#v", got)
	}

	otherDevice := ResolveContext{UserID: "user-1", DeviceID: "phone-2", Platform: "android", DeviceOnline: true}
	if got := r.ResolveWithContext(ctx, CapabilityConversationShell, otherDevice).Provider; got == nil || got.ProviderID != "test.cloud" {
		t.Fatalf("user profile should apply to other device: %#v", got)
	}

	otherUser := ResolveContext{UserID: "user-2", DeviceID: "phone-1", Platform: "android", DeviceOnline: true, DeviceCapabilities: []string{"native-ui"}}
	if got := r.ResolveWithContext(ctx, CapabilityConversationShell, otherUser).Provider; got == nil || !got.Builtin {
		t.Fatalf("profiles must not leak between users: %#v", got)
	}

	offline := device1
	offline.DeviceOnline = false
	if got := r.ResolveWithContext(ctx, CapabilityConversationShell, offline).Provider; got == nil || got.ProviderID != "test.cloud" {
		t.Fatalf("offline device provider should fall back to cloud provider: %#v", got)
	}
}

func TestScopedProfileRevisionConflict(t *testing.T) {
	ctx := context.Background()
	r := newCloudProfileTestRegistry(t)
	rc := ResolveContext{UserID: "user-1", Platform: "android"}
	first, err := r.SetProfileForContext(ctx, rc, ProfileScopeUser, Profile{
		ProfileID: "user-1", Name: "User 1", Selections: map[Capability]string{},
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision != 1 {
		t.Fatalf("expected first revision 1, got %d", first.Revision)
	}
	_, err = r.SetProfileForContext(ctx, rc, ProfileScopeUser, Profile{
		ProfileID: "user-1", Name: "Stale", Selections: map[Capability]string{},
	}, 0)
	if !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("expected revision conflict, got %v", err)
	}
}

func TestProfileScopeKeyEscapesDelimiters(t *testing.T) {
	a := ProfileScope{UserID: "user|d=device", Platform: "android"}.Key()
	b := ProfileScope{UserID: "user", DeviceID: "device", Platform: "android"}.Key()
	if a == b {
		t.Fatalf("escaped profile scope keys collided: %q", a)
	}
}

func TestDeviceRequirementsFailClosedWhenMetadataMissing(t *testing.T) {
	ctx := context.Background()
	r := newCloudProfileTestRegistry(t)
	cloud := schemaProvider("test.cloud", PlacementCloud, "", 10)
	device := schemaProvider("test.device", PlacementDevice, "test.cloud", 20)
	device.DeviceRequirements = &DeviceRequirements{
		Architectures: []string{"arm64"}, MinAppVersion: "1.2.0", MinRuntimeVersion: "1.3.0",
	}
	if err := r.Register(cloud); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(device); err != nil {
		t.Fatal(err)
	}
	rc := ResolveContext{UserID: "user-1", DeviceID: "phone-1", Platform: "android", DeviceOnline: true}
	if _, err := r.SetProfileForContext(ctx, rc, ProfileScopeDevice, Profile{
		ProfileID: "phone-1", Name: "Phone 1", Selections: map[Capability]string{CapabilityConversationShell: "test.device"},
	}, 0); err != nil {
		t.Fatal(err)
	}
	if got := r.ResolveWithContext(ctx, CapabilityConversationShell, rc).Provider; got == nil || got.ProviderID != "test.cloud" {
		t.Fatalf("missing device metadata must fail closed to cloud fallback: %#v", got)
	}
	rc.Architecture = "arm64"
	rc.AppVersion = "1.2.0"
	rc.RuntimeVersion = "1.3.0"
	if got := r.ResolveWithContext(ctx, CapabilityConversationShell, rc).Provider; got == nil || got.ProviderID != "test.device" {
		t.Fatalf("complete compatible metadata should allow device provider: %#v", got)
	}
}

func TestRuntimeProfileIsIndependentFromDeviceAndPlatform(t *testing.T) {
	ctx := context.Background()
	r := newCloudProfileTestRegistry(t)
	cloud := schemaProvider("test.cloud", PlacementCloud, "", 10)
	cloud.Entries["windows"] = Entry{Type: EntrySchemaRenderer, ContributionID: "test.cloud.schema.windows"}
	if err := r.Register(cloud); err != nil {
		t.Fatal(err)
	}

	rc := ResolveContext{UserID: "user-1", DeviceID: "phone-1", Platform: "android", RuntimeProfile: "cloud-core"}
	saved, err := r.SetProfileForContext(ctx, rc, ProfileScopeRuntime, Profile{
		ProfileID: "cloud-runtime", Name: "Cloud Runtime", Selections: map[Capability]string{CapabilityConversationShell: "test.cloud"},
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Scope.DeviceID != "" || saved.Scope.Platform != "" || saved.Scope.RuntimeProfile != "cloud-core" {
		t.Fatalf("runtime scope must be user+runtime only: %#v", saved.Scope)
	}

	otherDevice := ResolveContext{UserID: "user-1", DeviceID: "desktop-2", Platform: "windows", RuntimeProfile: "cloud-core"}
	if got := r.ResolveWithContext(ctx, CapabilityConversationShell, otherDevice).Provider; got == nil || got.ProviderID != "test.cloud" {
		t.Fatalf("runtime override should follow the user across devices/platforms: %#v", got)
	}
}
