package workbenchbrowser

import (
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/browser"
)

func TestAgentReferenceScopeAndRevocation(t *testing.T) {
	session := browser.BrowserSessionID("browser-session")
	tab := browser.BrowserTabID("browser-tab")
	ref := ExposeAgentTab("space-one", "conversation-one", session, tab)
	if len(ref) != 32 {
		t.Fatalf("invalid opaque reference length: %d", len(ref))
	}
	if id, tid, allowed := ResolveAgentTab("space-one", "conversation-one", ref); !allowed || id != session || tid != tab {
		t.Fatal("owner cannot resolve valid agent tab")
	}
	if _, _, allowed := ResolveAgentTab("space-two", "conversation-one", ref); allowed {
		t.Fatal("cross-space browser observation")
	}
	if _, _, allowed := ResolveAgentTab("space-one", "conversation-two", ref); allowed {
		t.Fatal("cross-conversation browser observation")
	}
	if _, _, allowed := ResolveAgentTab("space-one", "conversation-one", strings.Repeat("x", 32)); allowed {
		t.Fatal("unregistered reference accepted")
	}
	for _, unsafe := range []string{"../", strings.Repeat("!", 32), strings.Repeat("/", 32)} {
		if _, _, allowed := ResolveAgentTab("space-one", "conversation-one", unsafe); allowed {
			t.Fatalf("invalid browser reference accepted: %q", unsafe)
		}
	}
	RevokeAgentTab(ref)
	if _, _, allowed := ResolveAgentTab("space-one", "conversation-one", ref); allowed {
		t.Fatal("revoked reference accepted")
	}
}

func TestAgentReferenceExpirationAndBadArguments(t *testing.T) {
	if ExposeAgentTab("", "c", "s", "t") != "" || ExposeAgentTab("space", "../bad", "s", "t") != "" {
		t.Fatal("invalid agent tab was exposed")
	}
	ref := ExposeAgentTab("space", "conversation", "session", "tab")
	if ref == "" {
		t.Fatal("agent reference allocation failed")
	}
	agentTabs.Lock()
	record := agentTabs.items[ref]
	record.expires = time.Now().Add(-time.Second)
	agentTabs.items[ref] = record
	agentTabs.Unlock()
	if _, _, allowed := ResolveAgentTab("space", "conversation", ref); allowed {
		t.Fatal("expired reference still accessible")
	}
	RevokeAgentTab(ref)
}
