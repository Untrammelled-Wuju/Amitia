package javascript_main

import (
	"errors"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/jsonrpc"
)

func TestReportUndecodableFrameDeliversOriginalFailure(t *testing.T) {
	host := &PluginHost{instanceID: "test-instance", pending: map[string]chan *jsonrpc.Response{}}
	ch := make(chan *jsonrpc.Response, 1)
	host.pending["host-11"] = ch
	frame := []byte(`{"jsonrpc":"2.0","id":"host-11","error":{"code":{"unexpected":true},"message":"插件目录请求失败: upstream_error HTTP 500"}}`)
	decodeErr := errors.New("jsonrpc: decode response: json: cannot unmarshal object into Go struct field Error.error.code of type jsonrpc.ErrorCode")
	host.reportUndecodableFrame(frame, decodeErr)

	select {
	case resp := <-ch:
		if resp == nil || resp.Error == nil {
			t.Fatal("expected error response for undecodable frame")
		}
		if !strings.Contains(resp.Error.Message, decodeErr.Error()) {
			t.Fatalf("expected original decode error preserved, got %q", resp.Error.Message)
		}
		if !strings.Contains(resp.Error.Message, string(frame)) {
			t.Fatalf("expected original frame preserved, got %q", resp.Error.Message)
		}
		if resp.Error.Code != jsonrpc.ErrCodeProtocol {
			t.Fatalf("expected %q, got %q", jsonrpc.ErrCodeProtocol, resp.Error.Code)
		}
	default:
		t.Fatal("undecodable frame was dropped without reporting")
	}

	host.pendingMu.Lock()
	_, stillPending := host.pending["host-11"]
	host.pendingMu.Unlock()
	if stillPending {
		t.Fatal("expected pending request to be released")
	}
}

func TestReportUndecodableFrameWithoutIDDoesNotPanic(t *testing.T) {
	host := &PluginHost{instanceID: "test-instance", pending: map[string]chan *jsonrpc.Response{}}
	host.reportUndecodableFrame([]byte(`{"jsonrpc":"2.0","method":"runtime.event"`), errors.New("jsonrpc: decode: unexpected end of JSON input"))
	if len(host.pending) != 0 {
		t.Fatalf("expected no pending entries, got %d", len(host.pending))
	}
}
