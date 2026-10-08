package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestRealPairingPlatformMatrixRejectsDuplicateReverseAndSelfScans(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	for _, scenario := range []struct {
		name             string
		caller, provider runtimeidentity.Platform
	}{
		{"phone-phone", runtimeidentity.PlatformAndroid, runtimeidentity.PlatformAndroid},
		{"computer-phone", runtimeidentity.PlatformWindows, runtimeidentity.PlatformAndroid},
		{"phone-computer", runtimeidentity.PlatformAndroid, runtimeidentity.PlatformWindows},
		{"computer-computer", runtimeidentity.PlatformWindows, runtimeidentity.PlatformWindows},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			a, b := newThreeCoreFixture(t, "pairing-a", schemas, scenario.caller), newThreeCoreFixture(t, "pairing-b", schemas, scenario.provider)
			pairThreeCoreFixtures(t, a, b)
			scan := func(caller, provider *threeCoreFixture, expected int) {
				t.Helper()
				_, token, err := provider.pairing.CreateOffer(t.Context(), provider.device.DeviceID, time.Minute, true)
				if err != nil {
					t.Fatal(err)
				}
				payload, _ := json.Marshal(map[string]any{"endpoint": provider.endpoint, "offerToken": token, "label": caller.core})
				recorder := httptest.NewRecorder()
				request := httptest.NewRequest(http.MethodPost, "/internal/device-mesh/pairing/claim", bytes.NewReader(payload))
				request.RemoteAddr = "127.0.0.1:40000"
				request.Header.Set("Content-Type", "application/json")
				caller.router.ServeHTTP(recorder, request)
				var result struct {
					Message string `json:"message"`
				}
				_ = json.Unmarshal(recorder.Body.Bytes(), &result)
				if recorder.Code != expected || result.Message == "" {
					t.Fatalf("%s scans %s: status=%d expected=%d message=%s", caller.core, provider.core, recorder.Code, expected, result.Message)
				}
			}
			scan(a, b, http.StatusConflict)
			scan(b, a, http.StatusConflict)
			scan(a, a, http.StatusConflict)
			approvals, err := a.pairing.PendingApprovals(t.Context())
			if err != nil || len(approvals) != 0 {
				t.Fatal("反向重复扫码在对端创建了多余审批")
			}
			stored, err := a.local.LoadCredential()
			if err != nil || stored == nil || stored.SpaceID.String() != b.core {
				t.Fatal("拒绝重复扫码后原配对失效")
			}
		})
	}
}
