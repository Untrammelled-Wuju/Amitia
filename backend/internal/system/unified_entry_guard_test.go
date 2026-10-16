package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExternalTextEntrypointsUseOwnedExecution(t *testing.T) {
	serverRouter := readGuardFile(t, "..", "..", "cmd", "server", "router.go")
	services := readGuardFile(t, "..", "..", "cmd", "server", "services.go")
	chatRouter := readGuardFile(t, "..", "chat", "router.go")
	systemRouter := readGuardFile(t, "router.go")
	agentRouter := readGuardFile(t, "..", "agent", "router.go")
	for _, text := range []string{chatRouter, systemRouter, agentRouter} {
		for _, route := range []string{`POST("/chat"`, `POST("/web-chat/messages"`, `POST("/agent/webhook"`, `POST("/agent/test"`, `POST("/voice/turn"`} {
			if strings.Contains(text, route) {
				t.Fatalf("legacy entry registered: %s", route)
			}
		}
	}
	if strings.Contains(serverRouter, "RegisterVoiceEntryRouter") || strings.Contains(serverRouter, "RegisterChatRouter") {
		t.Fatal("server still registers legacy entry")
	}
	if !strings.Contains(serverRouter, "chat.RegisterModelRouter") {
		t.Fatal("model configuration routes were lost")
	}
	if !strings.Contains(services, "SetEntryExecutor") {
		t.Fatal("source entry does not use owned executor")
	}
}

func readGuardFile(t *testing.T, parts ...string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
