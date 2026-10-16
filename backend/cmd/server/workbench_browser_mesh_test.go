package main

import (
	"net/http"
	"testing"

	"github.com/u-ai/backend/internal/workbenchbrowser"
)

func TestRemoteBrowserReadErrorHTTPStatus(t *testing.T) {
	cases := []struct {
		code   workbenchbrowser.RemoteReadErrorCode
		status int
	}{
		{workbenchbrowser.RemoteReadInvalidRequest, http.StatusBadRequest},
		{workbenchbrowser.RemoteReadCapabilityRevoked, http.StatusGone},
		{workbenchbrowser.RemoteReadRuntimeUnavailable, http.StatusServiceUnavailable},
		{workbenchbrowser.RemoteReadUnsupported, http.StatusNotImplemented},
		{workbenchbrowser.RemoteReadResourceExceeded, http.StatusBadGateway},
	}
	for _, item := range cases {
		if got := remoteBrowserReadHTTPStatus(item.code); got != item.status {
			t.Fatalf("code %s returned %d", item.code, got)
		}
	}
}
