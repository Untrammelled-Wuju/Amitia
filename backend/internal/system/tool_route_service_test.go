package system

import (
	"testing"

	"github.com/u-ai/backend/internal/requestidentity"
)

func TestSystemTime_FallbackWhenNoTemporalService(t *testing.T) {
	svc := &service{}
	result := svc.systemTimeResult(map[string]interface{}{})
	if result["source"] != "system-fallback" {
		t.Errorf("expected fallback source, got %v", result["source"])
	}
	if result["timezoneConfirmed"] != false {
		t.Errorf("expected timezoneConfirmed=false, got %v", result["timezoneConfirmed"])
	}
}

func TestSystemTime_ForcesCanonicalUser(t *testing.T) {
	result := (&service{}).systemTimeResult(map[string]interface{}{
		"userId": "random-user-abc",
	})
	_, hasUserLocal := result["userLocal"]
	if hasUserLocal {
		t.Error("fallback should not contain userLocal")
	}
}

func TestSystemTime_IgnoresBodyUserId(t *testing.T) {
	_ = requestidentity.DefaultUserID
	result := (&service{}).systemTimeResult(map[string]interface{}{
		"userId":      "attacker-id",
		"characterId": "char-1",
	})
	if result["source"] != "system-fallback" {
		t.Errorf("expected fallback, got %v", result["source"])
	}
}
