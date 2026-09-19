package main

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/nativebridge"
)

type recordingNativeBridge struct {
	request  nativebridge.Request
	response nativebridge.Response
	err      error
}

func (b *recordingNativeBridge) Execute(_ context.Context, req nativebridge.Request) (nativebridge.Response, error) {
	b.request = req
	return b.response, b.err
}

func (b *recordingNativeBridge) Health(context.Context) nativebridge.Health {
	return nativebridge.HealthReady
}

func TestWorkspaceSAFBridgeCallAddsRequestID(t *testing.T) {
	native := &recordingNativeBridge{
		response: nativebridge.Response{
			Status: "success",
			Result: map[string]any{
				"valid":             true,
				"readable":          true,
				"writable":          true,
				"providerAvailable": true,
				"rootExists":        true,
			},
		},
	}
	bridge := newWorkspaceSAFBridge(native)

	status, err := bridge.GrantStatus(context.Background(), "content://test/tree")
	if err != nil {
		t.Fatal(err)
	}
	if !status.Valid {
		t.Fatal("expected valid SAF grant")
	}
	if native.request.RequestId == "" {
		t.Fatal("expected SAF native bridge requestId")
	}
	if native.request.Operation != "workspace.saf.grant_status" {
		t.Fatalf("operation = %q", native.request.Operation)
	}
}
