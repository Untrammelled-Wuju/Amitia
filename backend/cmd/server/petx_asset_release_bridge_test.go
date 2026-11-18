package main

import (
	"context"
	"strings"
	"testing"
)

func TestPetxAssetReleaseBridgeRejectsMissingResource(t *testing.T) {
	bridge := newPetxAssetReleaseBridge(nil, nil, nil, nil, nil, "space", "device")
	_, err := bridge.AttachPluginResource(context.Background(), "com.example/pet", "pet", 1, map[string]any{"contributionKind": "pet_resource"})
	if err == nil || !strings.Contains(err.Error(), "resource reference is required") {
		t.Fatalf("missing resource reference error = %v", err)
	}
}
