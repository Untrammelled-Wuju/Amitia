package plugincontract

import (
	"errors"
	"testing"
)

func TestParseIdentityAcceptsRealShapes(t *testing.T) {
	cases := []struct {
		kind IdentityKind
		raw  string
	}{
		{IdentityKindService, "channel.provider"},
		{IdentityKindService, "example.service"},
		{IdentityKindService, "com.amitia/emote/ai"},
		{IdentityKindService, "mock-go-runtime"},
		{IdentityKindProvider, "com.amitia/emote/channel.provider"},
		{IdentityKindProvider, "extprov_5f3c9a"},
		{IdentityKindProvider, "builtin.android_native/tls"},
		{IdentityKindProvider, "com.amitia/emote/emote-runtime/ai"},
		{IdentityKindCapability, "com.amitia/emote/emote-runtime/ai"},
		{IdentityKindInstance, "extprov_5f3c9a/provinst_1"},
		{IdentityKindInstance, "mesh-task-instance-7/worker"},
		{IdentityKindCapability, "plugin/namespace/name"},
	}
	for _, item := range cases {
		parsed, err := ParseIdentity(item.kind, item.raw)
		if err != nil {
			t.Fatalf("%s %q: expected valid, got %v", item.kind, item.raw, err)
		}
		if parsed.String() != item.raw {
			t.Fatalf("%s %q: canonical form changed to %q", item.kind, item.raw, parsed.String())
		}
		if parsed.Kind() != item.kind {
			t.Fatalf("%s %q: kind lost", item.kind, item.raw)
		}
	}
}

func TestParseIdentityRejectsMalformed(t *testing.T) {
	cases := []struct {
		kind   IdentityKind
		raw    string
		reason string
	}{
		{IdentityKindService, "", "empty"},
		{IdentityKindService, " media.transcribe", "surrounding whitespace"},
		{IdentityKindService, "Media.Transcribe", "uppercase"},
		{IdentityKindService, "media..transcribe", "empty inner segment"},
		{IdentityKindService, "a//b", "empty middle segment"},
		{IdentityKindService, "/media", "leading separator"},
		{IdentityKindService, "media.", "trailing separator"},
		{IdentityKindService, "media transcribe", "space"},
		{IdentityKindService, "媒体.转写", "non ascii"},
		{IdentityKindProvider, "a/b/c/d/e", "too many provider segments"},
		{IdentityKindCapability, "a/b/c/d/e", "capability needs three to four segments"},
		{IdentityKindInstance, "onlyone", "instance needs provider prefix"},
		{IdentityKindCapability, "plugin/name", "capability needs three segments"},
		{IdentityKind("unknown"), "a.b", "unknown kind"},
	}
	for _, item := range cases {
		if _, err := ParseIdentity(item.kind, item.raw); err == nil {
			t.Fatalf("%s %q (%s): expected rejection", item.kind, item.raw, item.reason)
		} else if !errors.Is(err, ErrInvalidIdentity) {
			t.Fatalf("%s %q: error must wrap ErrInvalidIdentity, got %v", item.kind, item.raw, err)
		}
	}
}

func TestBuildIdentities(t *testing.T) {
	serviceID, err := BuildServiceID("Channel", " Provider ")
	if err != nil {
		t.Fatalf("build service id: %v", err)
	}
	if serviceID != "channel.provider" {
		t.Fatalf("unexpected service id %q", serviceID)
	}
	providerID, err := BuildProviderID(ModuleRef{PluginID: "com.amitia", ModuleID: "emote"}, serviceID)
	if err != nil {
		t.Fatalf("build provider id: %v", err)
	}
	if providerID != "com.amitia/emote/channel.provider" {
		t.Fatalf("unexpected provider id %q", providerID)
	}
	instanceID, err := BuildInstanceID(providerID, "DESKTOP-1")
	if err != nil {
		t.Fatalf("build instance id: %v", err)
	}
	if instanceID != "com.amitia/emote/channel.provider/desktop-1" {
		t.Fatalf("unexpected instance id %q", instanceID)
	}
	if _, err := BuildServiceID("Channel", ""); !errors.Is(err, ErrInvalidIdentity) {
		t.Fatalf("empty name must be rejected, got %v", err)
	}
	if _, err := BuildProviderID(ModuleRef{PluginID: "com.amitia", ModuleID: ""}, serviceID); !errors.Is(err, ErrInvalidIdentity) {
		t.Fatalf("empty module id must be rejected, got %v", err)
	}
}

func TestDetectDuplicateIdentities(t *testing.T) {
	if err := DetectDuplicateIdentities(IdentityKindService, []string{"a.b", "c.d", "a.b"}); err == nil {
		t.Fatalf("duplicate must be reported")
	} else {
		var duplicated *DuplicateIdentityError
		if !errors.As(err, &duplicated) {
			t.Fatalf("expected DuplicateIdentityError, got %T", err)
		}
		if len(duplicated.Duplicates) != 1 || duplicated.Duplicates[0] != "a.b" {
			t.Fatalf("unexpected duplicates %v", duplicated.Duplicates)
		}
		if !errors.Is(err, ErrInvalidIdentity) {
			t.Fatalf("duplicate error must wrap ErrInvalidIdentity")
		}
	}
	if err := DetectDuplicateIdentities(IdentityKindService, []string{"a.b", "c.d"}); err != nil {
		t.Fatalf("distinct ids must pass, got %v", err)
	}
}
