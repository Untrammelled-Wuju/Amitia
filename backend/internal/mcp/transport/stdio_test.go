package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/mcp/protocol"
)

func TestStdioTransportStartsWithoutShellAndIsolatesEnvironment(t *testing.T) {
	stderr := make(chan string, 2)
	target := NewStdio(StdioConfig{
		Command:     os.Args[0],
		Args:        []string{"-test.run=TestStdioHelperProcess"},
		Environment: map[string]string{"GO_WANT_MCP_HELPER": "1", "MCP_TEST_SECRET": "allowed"},
		OnStderr:    func(line string) { stderr <- line },
	})
	if err := target.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	request, _ := protocol.Request(1, "environment/read", nil)
	if err := target.Send(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	select {
	case response := <-target.Receive():
		var result map[string]any
		if err := json.Unmarshal(response.Result, &result); err != nil {
			t.Fatal(err)
		}
		if result["allowed"] != "allowed" || result["modelKeyPresent"] != false {
			t.Fatalf("environment was not isolated: %#v", result)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("missing stdio response")
	}
	select {
	case line := <-stderr:
		if !strings.Contains(line, "helper diagnostic") {
			t.Fatalf("unexpected stderr: %s", line)
		}
	case <-time.After(time.Second):
		t.Fatal("missing stderr diagnostic")
	}
	if err := target.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if target.State() != StateStopped {
		t.Fatalf("unexpected final state: %s", target.State())
	}
}

func TestStdioTransportRejectsShellAndMissingCommand(t *testing.T) {
	for _, command := range []string{"cmd.exe", "powershell.exe", "pwsh.exe", "sh"} {
		target := NewStdio(StdioConfig{Command: command})
		if err := target.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "MCP_STDIO_COMMAND_NOT_ALLOWED") {
			t.Fatalf("expected shell rejection for %s, got %v", command, err)
		}
	}
	missing := NewStdio(StdioConfig{Command: "amitia-command-that-does-not-exist"})
	if err := missing.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "command not found") {
		t.Fatalf("expected missing command error, got %v", err)
	}
}

func TestStdioTransportRejectsOversizedOutput(t *testing.T) {
	target := NewStdio(StdioConfig{Command: os.Args[0], Args: []string{"-test.run=TestStdioHelperProcess"}, Environment: map[string]string{"GO_WANT_MCP_HELPER": "1", "MCP_HELPER_OVERSIZE": "1"}, MaxMessageBytes: 128})
	if err := target.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	request, _ := protocol.Request(1, "oversize", nil)
	if err := target.Send(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && target.State() != StateError {
		time.Sleep(10 * time.Millisecond)
	}
	if target.State() != StateError {
		t.Fatalf("expected protocol output failure, got %s", target.State())
	}
	_ = target.Close(context.Background())
}

func TestStdioHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_MCP_HELPER") != "1" {
		return
	}
	fmt.Fprintln(os.Stderr, "helper diagnostic")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		message, err := protocol.Decode(scanner.Bytes(), 4<<20)
		if err != nil {
			os.Exit(2)
		}
		var result any = map[string]any{"ok": true}
		if message.Method == "environment/read" {
			_, modelKeyPresent := os.LookupEnv("MODEL_API_KEY")
			result = map[string]any{"allowed": os.Getenv("MCP_TEST_SECRET"), "modelKeyPresent": modelKeyPresent}
		}
		if os.Getenv("MCP_HELPER_OVERSIZE") == "1" {
			result = map[string]any{"value": strings.Repeat("x", 1024)}
		}
		response, _ := protocol.Response(message.ID, result)
		data, _ := protocol.Encode(response, 4<<20)
		fmt.Println(string(data))
	}
	os.Exit(0)
}
