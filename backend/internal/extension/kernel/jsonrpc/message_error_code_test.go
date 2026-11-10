package jsonrpc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeEnvelopeAcceptsNumericErrorCode(t *testing.T) {
	raw := []byte(`{"jsonrpc":"2.0","id":"host-7","error":{"code":-32603,"message":"插件目录请求失败: upstream_error HTTP 500 {\"msg\":\"读取插件市场分类: dial tcp 127.0.0.1:3306: connect: connection refused\"}"}}`)
	env, err := DecodeEnvelope(raw)
	if err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if env.Kind != KindError {
		t.Fatalf("expected error envelope, got %s", env.Kind)
	}
	if env.Response == nil || env.Response.Error == nil {
		t.Fatal("expected error payload")
	}
	if got := string(env.Response.Error.Code); got != "-32603" {
		t.Fatalf("expected numeric code preserved as text, got %q", got)
	}
	if !strings.Contains(env.Response.Error.Message, "读取插件市场分类") {
		t.Fatalf("expected upstream message preserved, got %q", env.Response.Error.Message)
	}
	if !strings.Contains(env.Response.Error.Message, "connection refused") {
		t.Fatalf("expected upstream cause preserved, got %q", env.Response.Error.Message)
	}
	if got := env.Response.ID.String(); got != "host-7" {
		t.Fatalf("expected request id preserved, got %q", got)
	}
}

func TestDecodeEnvelopeAcceptsStringErrorCode(t *testing.T) {
	raw := []byte(`{"jsonrpc":"2.0","id":"host-8","error":{"code":"timeout","message":"tool execution timed out"}}`)
	env, err := DecodeEnvelope(raw)
	if err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if env.Kind != KindError || env.Response == nil || env.Response.Error == nil {
		t.Fatal("expected error envelope")
	}
	if env.Response.Error.Code != ErrCodeTimeout {
		t.Fatalf("expected %q, got %q", ErrCodeTimeout, env.Response.Error.Code)
	}
}

func TestErrorCodeUnmarshalRejectsNonScalar(t *testing.T) {
	var code ErrorCode
	if err := json.Unmarshal([]byte(`{"value":-32603}`), &code); err == nil {
		t.Fatal("expected invalid error code to be reported")
	} else if !strings.Contains(err.Error(), `{"value":-32603}`) {
		t.Fatalf("expected original payload in error, got %v", err)
	}
}

func TestErrorCodeMarshalStaysString(t *testing.T) {
	encoded, err := json.Marshal(Error{Code: "-32603", Message: "runtime failure"})
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	if !strings.Contains(string(encoded), `"code":"-32603"`) {
		t.Fatalf("expected string encoded code, got %s", string(encoded))
	}
}
