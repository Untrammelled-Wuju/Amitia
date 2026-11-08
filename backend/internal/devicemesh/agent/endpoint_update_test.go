package agent

import (
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestUpdateEndpointRejectsRequestWithoutActiveCredential(t *testing.T) {
	handler := NewLocalHandler(t.TempDir(), runtimeidentity.PlatformAndroid)
	_, err := handler.UpdateEndpoint(t.Context(), endpointUpdateRequest{
		CloudBaseURL: "https://172.16.176.114:54796",
		Fingerprint:  "a0f155357f00de773ad26652201a872f311ecc987aca4da743864742045084a9",
		CoreID:       "space-a",
	})
	var failure *BindingError
	if !errors.As(err, &failure) || failure.Status != 409 || failure.Code != "pairing_required" {
		t.Fatalf("unexpected failure: %v", err)
	}
}

func TestUpdateEndpointRejectsDifferentCoreWithoutChangingCredential(t *testing.T) {
	handler := NewLocalHandler(t.TempDir(), runtimeidentity.PlatformAndroid)
	credential := &StoredCredential{
		CloudBaseUrl: "https://10.0.0.2:54796",
		CredentialID: "credential-a",
		Credential:   "credential-secret",
		SpaceID:      "space-a",
		DeviceID:     "device-a",
		RuntimeID:    "runtime-a",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	if err := handler.credStore.SaveCredential(credential); err != nil {
		t.Fatal(err)
	}
	_, err := handler.UpdateEndpoint(t.Context(), endpointUpdateRequest{
		CloudBaseURL: "https://172.16.176.114:54796",
		Fingerprint:  "a0f155357f00de773ad26652201a872f311ecc987aca4da743864742045084a9",
		CoreID:       "space-b",
	})
	var failure *BindingError
	if !errors.As(err, &failure) || failure.Status != 409 || failure.Code != "core_identity_changed" {
		t.Fatalf("unexpected failure: %v", err)
	}
	stored, err := handler.credStore.LoadCredential()
	if err != nil || stored == nil || stored.CloudBaseUrl != credential.CloudBaseUrl || stored.SpaceID != credential.SpaceID {
		t.Fatalf("credential changed: %+v %v", stored, err)
	}
}

func TestUpdateEndpointRejectsInvalidAddress(t *testing.T) {
	handler := NewLocalHandler(t.TempDir(), runtimeidentity.PlatformAndroid)
	_, err := handler.UpdateEndpoint(t.Context(), endpointUpdateRequest{
		CloudBaseURL: "file:///tmp/center",
		CoreID:       "space-a",
	})
	var failure *BindingError
	if !errors.As(err, &failure) || failure.Status != 400 || failure.Code != "invalid_provider_endpoint" {
		t.Fatalf("unexpected failure: %v", err)
	}
}

func TestUpdateEndpointRejectsCoreMode(t *testing.T) {
	handler := NewLocalHandler(t.TempDir(), runtimeidentity.PlatformAndroid)
	handler.SetAgentMode(false)
	_, err := handler.UpdateEndpoint(t.Context(), endpointUpdateRequest{
		CloudBaseURL: "https://172.16.176.114:54796",
		CoreID:       "space-b",
	})
	var failure *BindingError
	if !errors.As(err, &failure) || failure.Status != 409 || failure.Code != "mesh.agent_mode_required" {
		t.Fatalf("unexpected failure: %v", err)
	}
}
