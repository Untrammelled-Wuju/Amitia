package matrices

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"testing"
)

func TestSmokePseudoHostExact(t *testing.T) {
	bin := "../../../go/cmd/mock-game-plugin/mock-game-plugin.exe"
	t.Logf("BINARY=%s", bin)

	ctx := context.Background()

	cmd := exec.CommandContext(ctx, bin)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("stderr: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Drain stderr
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				t.Logf("STDERR: %q", string(buf[:n]))
			}
			if err != nil {
				return
			}
		}
	}()

	br := bufio.NewReader(stdout)

	// Read first frame (Hello)
	var length uint32
	if err := binary.Read(br, binary.BigEndian, &length); err != nil {
		t.Fatalf("read hello header: %v", err)
	}
	helloBuf := make([]byte, length)
	if _, err := io.ReadFull(br, helloBuf); err != nil {
		t.Fatalf("read hello payload: %v", err)
	}
	t.Logf("HELLO_LEN=%d", length)

	var helloEnv map[string]any
	_ = json.Unmarshal(helloBuf, &helloEnv)
	helloID, _ := helloEnv["id"].(string)
	t.Logf("HELLO_ID=%s", helloID)

	// Send Hello response with correct requestID
	helloResp := fmt.Sprintf(`{"protocol":"amitia-game-host/1","capabilities":["realtime_control","state_streaming","event_streaming","custom_rpc","host_api","shared_control","multi_service"],"rpcNamespaces":["mock.core","mock.state","mock.data","mock.control","mock.security","mock.fault"]}`)
	respEnv := map[string]any{
		"protocol":  "amitia-game-host/1",
		"type":      "response",
		"requestId": helloID,
		"payload":   json.RawMessage(helloResp),
	}
	respBytes, _ := json.Marshal(respEnv)
	if err := binary.Write(stdin, binary.BigEndian, uint32(len(respBytes))); err != nil {
		t.Fatalf("write resp header: %v", err)
	}
	if _, err := stdin.Write(respBytes); err != nil {
		t.Fatalf("write resp payload: %v", err)
	}
	t.Logf("WROTE_HELLO_RESP=%d bytes", len(respBytes))

	// Now read subsequent frames (OnReady: secret.acquire, sink.register, mock.ready)
	for i := 0; i < 10; i++ {
		var flen uint32
		if err := binary.Read(br, binary.BigEndian, &flen); err != nil {
			t.Logf("FRAME_%d_HEADER_ERR=%v", i, err)
			break
		}
		fbuf := make([]byte, flen)
		if _, err := io.ReadFull(br, fbuf); err != nil {
			t.Logf("FRAME_%d_PAYLOAD_ERR=%v", i, err)
			break
		}
		var fenv map[string]any
		_ = json.Unmarshal(fbuf, &fenv)
		fType, _ := fenv["type"].(string)
		fMethod, _ := fenv["method"].(string)
		fID, _ := fenv["id"].(string)
		t.Logf("FRAME_%d type=%s method=%s id=%s len=%d", i, fType, fMethod, fID, flen)

		// If it's a request, respond
		if fType == "request" {
			respAgain := map[string]any{
				"protocol":  "amitia-game-host/1",
				"type":      "response",
				"requestId": fID,
				"payload":   json.RawMessage(`{"stub":true}`),
			}
			rb, _ := json.Marshal(respAgain)
			if err := binary.Write(stdin, binary.BigEndian, uint32(len(rb))); err != nil {
				t.Logf("FRAME_%d RESP_HEADER_ERR=%v", i, err)
				break
			}
			if _, err := stdin.Write(rb); err != nil {
				t.Logf("FRAME_%d RESP_PAYLOAD_ERR=%v", i, err)
				break
			}
			t.Logf("FRAME_%d RESPONDED", i)
		}

		if fType == "notification" && fMethod == "mock.ready" {
			t.Logf("GOT_MOCK_READY")
			break
		}
	}

	stdin.Close()
	cmd.Process.Kill()
	cmd.Wait()
}
