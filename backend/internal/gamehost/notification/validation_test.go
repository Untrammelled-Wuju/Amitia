package notification

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestValidateRoute_OK(t *testing.T) {
	cases := []RouteContext{
		{"p", "r", "s", 1},
		{"plugin.a.b", "runtime-123", "svc-main", 2},
		{"P", "R", "S", 3},
	}
	for _, c := range cases {
		if err := ValidateRoute(c); err != nil {
			t.Errorf("expected valid, got error: %v", err)
		}
	}
}

func TestValidateRoute_MissingFields(t *testing.T) {
	cases := []struct {
		name string
		ctx  RouteContext
	}{
		{"all empty", RouteContext{}},
		{"missing plugin", RouteContext{RuntimeID: "r", ServiceID: "s", Generation: 1}},
		{"missing runtime", RouteContext{PluginID: "p", ServiceID: "s", Generation: 1}},
		{"missing service", RouteContext{PluginID: "p", RuntimeID: "r", Generation: 1}},
		{"missing generation", RouteContext{PluginID: "p", RuntimeID: "r", ServiceID: "s"}},
	}
	for _, c := range cases {
		err := ValidateRoute(c.ctx)
		if err == nil {
			t.Errorf("%s: expected error", c.name)
			continue
		}
		if !domain.IsHostError(err, domain.ErrInvalidArgument) {
			t.Errorf("%s: expected invalid_argument, got %v", c.name, err)
		}
	}
}

func TestValidateRoute_ControlChars(t *testing.T) {
	cases := []struct {
		name string
		ctx  RouteContext
	}{
		{"bad plugin ID", RouteContext{"bad\x01id", "r", "s", 1}},
		{"bad runtime ID", RouteContext{"p", "bad\x01id", "s", 1}},
		{"bad service ID", RouteContext{"p", "r", "bad\x01id", 1}},
	}
	for _, c := range cases {
		err := ValidateRoute(c.ctx)
		if err == nil {
			t.Errorf("%s: expected error for control chars", c.name)
		}
	}
}

func TestValidateMethod_OK(t *testing.T) {
	cases := []string{
		"example.game.entity.updated",
		"vendor.event",
		"plugin.custom.notification",
		"a",
	}
	for _, m := range cases {
		if err := ValidateMethod(m); err != nil {
			t.Errorf("method %q expected valid, got error: %v", m, err)
		}
	}
}

func TestValidateMethod_Empty(t *testing.T) {
	if err := ValidateMethod(""); err == nil {
		t.Error("expected error for empty method")
	}
}

func TestValidateMethod_TooLong(t *testing.T) {
	b := strings.Repeat("a", maxMethodLength+1)
	if err := ValidateMethod(b); err == nil {
		t.Error("expected error for too long method")
	}
}

func TestValidateMethod_MaxLength(t *testing.T) {
	b := strings.Repeat("a", maxMethodLength)
	if err := ValidateMethod(b); err != nil {
		t.Errorf("max length method should be valid, got: %v", err)
	}
}

func TestValidateMetadata_OK(t *testing.T) {
	cases := []map[string]json.RawMessage{
		nil,
		{},
		{"a": []byte(`"v"`)},
		{"trace_id": []byte(`"abc"`), "extra": []byte(`{"k":1}`)},
	}
	for i, m := range cases {
		if err := ValidateMetadata(m); err != nil {
			t.Errorf("case %d metadata expected valid, got error: %v", i, err)
		}
	}
}

func TestValidateMetadata_BadKey(t *testing.T) {
	cases := []struct {
		name string
		m    map[string]json.RawMessage
	}{
		{"empty key", map[string]json.RawMessage{"": []byte(`"v"`)}},
		{"control char", map[string]json.RawMessage{"bad\x01key": []byte(`"v"`)}},
	}
	for _, c := range cases {
		if err := ValidateMetadata(c.m); err == nil {
			t.Errorf("%s: expected error for bad key", c.name)
		}
	}
}

func TestValidateMetadata_LongKey(t *testing.T) {
	longKey := strings.Repeat("k", maxMetadataKeyLength+1)
	m := map[string]json.RawMessage{longKey: []byte(`"v"`)}
	if err := ValidateMetadata(m); err == nil {
		t.Error("expected error for too long metadata key")
	}
}
