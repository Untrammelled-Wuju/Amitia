package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExternalTextEntrypointsUseUnifiedEntry(t *testing.T) {
	streamHandler := readGuardFile(t, "stream_handler.go")
	if strings.Contains(streamHandler, "chatSvc.ProcessMessage") {
		t.Fatal("WebChatSendStream must not call chatSvc.ProcessMessage directly")
	}
	if !strings.Contains(streamHandler, "h.unifiedEntry.Handle") {
		t.Fatal("WebChatSendStream must call UnifiedEntry.Handle")
	}

	webHandler := readGuardFile(t, "webchat_handler.go")
	if !strings.Contains(webHandler, "h.unifiedEntry.Handle") {
		t.Fatal("WebChatSend must call UnifiedEntry.Handle")
	}

	agentService := readGuardFile(t, "..", "agent", "service.go")
	if strings.Contains(agentService, "chatSvc.ProcessMessage") {
		t.Fatal("Agent webhook must not call chatSvc.ProcessMessage directly")
	}
	if !strings.Contains(agentService, "s.unifiedEntry.Handle") {
		t.Fatal("Agent webhook must call UnifiedEntry.Handle")
	}

	serverRouter := readGuardFile(t, "..", "..", "cmd", "server", "router.go")
	chatRouter := readGuardFile(t, "..", "chat", "router.go")
	chatHandler := readGuardFile(t, "..", "chat", "handler.go")
	if !strings.Contains(serverRouter, "agent.RegisterAgentRouter(apiGroup, ctx, services.UnifiedEntry)") {
		t.Fatal("Agent router must be registered with services.UnifiedEntry")
	}
	if !strings.Contains(serverRouter, "system.RegisterSystemRouter(apiGroup, ctx, services.Chat, services.UnifiedEntry") {
		t.Fatal("System router must be registered with services.UnifiedEntry")
	}
	if !strings.Contains(serverRouter, "chat.RegisterChatRouter(apiGroup, ctx, services.Chat, services.UnifiedEntry)") &&
		!strings.Contains(serverRouter, "chat.RegisterChatRouterWithDelivery(apiGroup, ctx, services.Chat, services.UnifiedEntry") {
		t.Fatal("Chat router must be registered with services.UnifiedEntry")
	}
	if !strings.Contains(chatRouter, "func RegisterChatRouter(r *gin.RouterGroup, ctx *app.AppContext, svc Service, entry *interaction.UnifiedEntry)") {
		t.Fatal("Chat router must accept UnifiedEntry")
	}
	if !strings.Contains(chatHandler, "h.unifiedEntry.Handle") {
		t.Fatal("Chat handler must call UnifiedEntry.Handle")
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
