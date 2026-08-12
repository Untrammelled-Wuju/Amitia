package root

import "testing"

func TestMapRootStatusFromResult_Full(t *testing.T) {
	result := map[string]any{
		"platformSupported":   true,
		"rootFramework":       "Magisk",
		"rootManagerDetected": true,
		"suBinaryDetected":    true,
		"authorizationState":  "granted",
		"rootAvailable":       true,
		"backend":             "su",
		"state":               "authorized",
	}

	status := MapRootStatusFromResult(result)

	if !status.PlatformSupported {
		t.Fatal("expected PlatformSupported true")
	}
	if status.RootFramework != "Magisk" {
		t.Fatalf("expected Magisk, got %s", status.RootFramework)
	}
	if !status.RootManagerDetected {
		t.Fatal("expected RootManagerDetected true")
	}
	if !status.SUBinaryDetected {
		t.Fatal("expected SUBinaryDetected true")
	}
	if status.Authorization != AuthorizationGranted {
		t.Fatalf("expected granted, got %s", status.Authorization)
	}
	if !status.RootAvailable {
		t.Fatal("expected RootAvailable true")
	}
	if status.Backend != "su" {
		t.Fatalf("expected su, got %s", status.Backend)
	}
	if status.State != "authorized" {
		t.Fatalf("expected authorized, got %s", status.State)
	}
}

func TestMapRootStatusFromResult_Empty(t *testing.T) {
	status := MapRootStatusFromResult(map[string]any{})

	if status.PlatformSupported {
		t.Fatal("expected PlatformSupported false")
	}
	if status.RootFramework != "" {
		t.Fatalf("expected empty, got %s", status.RootFramework)
	}
	if status.State != "" {
		t.Fatalf("expected empty, got %s", status.State)
	}
}

func TestMapRootStatusFromResult_NonAndroid(t *testing.T) {
	result := map[string]any{
		"platformSupported": false,
		"state":             "unsupported",
	}

	status := MapRootStatusFromResult(result)

	if status.PlatformSupported {
		t.Fatal("expected PlatformSupported false")
	}
	if status.State != "unsupported" {
		t.Fatalf("expected unsupported, got %s", status.State)
	}
}

func TestDeriveRootState_FromState(t *testing.T) {
	status := RootStatus{State: "authorized"}
	state := DeriveRootState(status)
	if state != "authorized" {
		t.Fatalf("expected authorized, got %s", state)
	}
}

func TestDeriveRootState_Unsupported(t *testing.T) {
	status := RootStatus{PlatformSupported: false}
	state := DeriveRootState(status)
	if state != RootStateUnsupported {
		t.Fatalf("expected unsupported, got %s", state)
	}
}

func TestDeriveRootState_NotRooted(t *testing.T) {
	status := RootStatus{
		PlatformSupported:   true,
		SUBinaryDetected:    false,
		RootManagerDetected: false,
	}
	state := DeriveRootState(status)
	if state != RootStateNotRooted {
		t.Fatalf("expected not_rooted, got %s", state)
	}
}

func TestDeriveRootState_AuthorizationRequired(t *testing.T) {
	status := RootStatus{
		PlatformSupported: true,
		SUBinaryDetected:  true,
		Authorization:     AuthorizationRequired,
	}
	state := DeriveRootState(status)
	if state != RootStateAuthorizationRequired {
		t.Fatalf("expected authorization_required, got %s", state)
	}
}

func TestDeriveRootState_Authorized(t *testing.T) {
	status := RootStatus{
		PlatformSupported: true,
		SUBinaryDetected:  true,
		Authorization:     AuthorizationGranted,
	}
	state := DeriveRootState(status)
	if state != RootStateAuthorized {
		t.Fatalf("expected authorized, got %s", state)
	}
}

func TestDeriveRootState_Denied(t *testing.T) {
	status := RootStatus{
		PlatformSupported: true,
		SUBinaryDetected:  true,
		Authorization:     AuthorizationDenied,
	}
	state := DeriveRootState(status)
	if state != RootStateDenied {
		t.Fatalf("expected denied, got %s", state)
	}
}

func TestDeriveRootState_Unknown(t *testing.T) {
	status := RootStatus{
		PlatformSupported: true,
		SUBinaryDetected:  true,
		Authorization:     AuthorizationUnknown,
	}
	state := DeriveRootState(status)
	if state != RootStateAuthorizationUnknown {
		t.Fatalf("expected authorization_unknown, got %s", state)
	}
}

func TestDeriveRootState_Unavailable(t *testing.T) {
	status := RootStatus{
		PlatformSupported: true,
		SUBinaryDetected:  true,
	}
	state := DeriveRootState(status)
	if state != RootStateUnavailable {
		t.Fatalf("expected unavailable, got %s", state)
	}
}
