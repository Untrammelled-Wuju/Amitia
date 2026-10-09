package ioshostbridge

import (
	"bufio"
	"encoding/json"
	"io"
	"testing"
)

func testClient(t *testing.T, replies func(map[string]any) map[string]any) *Client {
	t.Helper()
	read, hostWrite := io.Pipe()
	hostRead, write := io.Pipe()
	c, err := NewClient(read, write, "generation-0000000001")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { read.Close(); write.Close(); hostRead.Close(); hostWrite.Close() })
	go func() {
		scanner := bufio.NewScanner(hostRead)
		for scanner.Scan() {
			var request map[string]any
			if json.Unmarshal(scanner.Bytes(), &request) != nil {
				return
			}
			reply := replies(request)
			if reply == nil {
				hostWrite.Close()
				return
			}
			json.NewEncoder(hostWrite).Encode(reply)
		}
	}()
	return c
}

func TestHostBridgeOriginalGenerationAndAcknowledgement(t *testing.T) {
	var sequence []string
	c := testClient(t, func(request map[string]any) map[string]any {
		sequence = append(sequence, request["id"].(string))
		if request["schemaVersion"] != float64(1) || request["generation"] != "generation-0000000001" || request["method"] != "secret.set" {
			t.Error("invalid original request")
		}
		return map[string]any{"schemaVersion": 1, "id": request["id"], "generation": request["generation"], "result": map[string]any{"ok": true}}
	})
	for range 2 {
		var result struct {
			OK bool `json:"ok"`
		}
		if err := c.Call("secret.set", map[string]string{"key": "key", "data": "aGVsbG8="}, &result); err != nil || !result.OK {
			t.Fatalf("missing ack: %v", err)
		}
	}
	if len(sequence) != 2 || sequence[0] != "1" || sequence[1] != "2" {
		t.Fatal("request identifiers reused")
	}
	if err := c.Call("native.execute", nil, &struct{}{}); err == nil {
		t.Fatal("unexpected method permitted")
	}
}

func TestHostBridgeRejectsLateOrMalformedResponse(t *testing.T) {
	for _, name := range []string{"generation", "id", "schema", "missing", "conflict", "disconnect"} {
		t.Run(name, func(t *testing.T) {
			c := testClient(t, func(request map[string]any) map[string]any {
				response := map[string]any{"schemaVersion": 1, "id": request["id"], "generation": request["generation"], "result": map[string]any{"ok": true}}
				switch name {
				case "generation":
					response["generation"] = "previous-generation"
				case "id":
					response["id"] = "previous-request"
				case "schema":
					response["schemaVersion"] = 2
				case "missing":
					delete(response, "result")
				case "conflict":
					response["error"] = "denied"
				case "disconnect":
					return nil
				}
				return response
			})
			if err := c.Call("identity.get", map[string]any{}, &struct{}{}); err == nil {
				t.Fatal("invalid response accepted")
			}
			if err := c.Call("identity.get", map[string]any{}, &struct{}{}); err == nil {
				t.Fatal("invalid channel recovered silently")
			}
		})
	}
}

func TestHostBridgeRequiredWithoutDescriptorsFailsClosed(t *testing.T) {
	t.Setenv("AMITIA_IOS_HOST_BRIDGE_REQUIRED", "true")
	for _, key := range []string{"READ_FD", "WRITE_FD", "GENERATION"} {
		t.Setenv("AMITIA_IOS_HOST_BRIDGE_"+key, "")
	}
	if c, err := FromEnvironment(); err == nil || c != nil {
		t.Fatal("required host bridge silently absent")
	}
}
