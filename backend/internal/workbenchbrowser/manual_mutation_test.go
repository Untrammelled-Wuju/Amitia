package workbenchbrowser

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/browser"
)

func TestManualMutationBlocksAgentAfterLeaseRelease(t *testing.T) {
	session, tab := browser.BrowserSessionID("manual-session"), browser.BrowserTabID("manual-tab")
	if _, ok := acquireAgentControl("manual-ref", "manual-owner", session, tab); !ok {
		t.Fatal("acquire failed")
	}
	finish, ok := BeginManualTabMutation("manual-ref", "manual-owner", agentControlLeaseToken("manual-ref", "manual-owner", session, tab), session, tab)
	if !ok {
		t.Fatal("manual input refused")
	}
	if !releaseAgentControl("manual-ref", "manual-owner", session, tab) {
		t.Fatal("lease release failed")
	}
	if _, ok := acquireAgentControl("new-ref", "new-owner", session, tab); ok {
		t.Fatal("second user preempted ongoing manual input")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if done, err := BeginAgentTabMutation(ctx, session, tab); !errors.Is(err, context.DeadlineExceeded) {
		if done != nil {
			done()
		}
		t.Fatalf("agent overlapped manual input: %v", err)
	}
	finish()
	finish()
	done, err := BeginAgentTabMutation(context.Background(), session, tab)
	if err != nil {
		t.Fatal(err)
	}
	done()
}

func TestManualMutationRevocationCannotInterruptInFlightInput(t *testing.T) {
	session, tab := browser.BrowserSessionID("revoked-session"), browser.BrowserTabID("revoked-tab")
	ref := ExposeAgentTab("scope", "conversation", session, tab)
	if ref == "" {
		t.Fatal("reference failed")
	}
	if _, ok := acquireAgentControl(ref, "owner", session, tab); !ok {
		t.Fatal("lease failed")
	}
	finish, ok := BeginManualTabMutation(ref, "owner", agentControlLeaseToken(ref, "owner", session, tab), session, tab)
	if !ok {
		t.Fatal("manual action failed")
	}
	RevokeAgentTab(ref)
	if _, ok := BeginManualTabMutation(ref, "owner", agentControlLeaseToken(ref, "owner", session, tab), session, tab); ok {
		t.Fatal("revoked reference authorized new mutation")
	}
	if !currentAgentControl(session, tab) {
		t.Fatal("Agent can race already submitted mutation")
	}
	finish()
	if currentAgentControl(session, tab) {
		t.Fatal("manual operation leaked")
	}
}

func TestManualMutationExpiredLeaseAndInFlightAgent(t *testing.T) {
	session, tab := browser.BrowserSessionID("expiry-session"), browser.BrowserTabID("expiry-tab")
	if _, ok := BeginManualTabMutation("ref", "operator", agentControlLeaseToken("ref", "operator", session, tab), session, tab); ok {
		t.Fatal("no lease")
	}
	if _, ok := acquireAgentControl("ref", "operator", session, tab); !ok {
		t.Fatal("lease failed")
	}
	agentControlLeases.Lock()
	key := agentControlTabKey(session, tab)
	lease := agentControlLeases.items[key]
	lease.deadline = time.Now().Add(-time.Second)
	agentControlLeases.items[key] = lease
	agentControlLeases.Unlock()
	if _, ok := BeginManualTabMutation("ref", "operator", agentControlLeaseToken("ref", "operator", session, tab), session, tab); ok {
		t.Fatal("expired lease accepted")
	}
	done, err := BeginAgentTabMutation(context.Background(), session, tab)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := acquireAgentControl("ref", "operator", session, tab); ok {
		t.Fatal("Agent preempted")
	}
	done()
}

func TestAgentMutationFinishIsIdempotent(t *testing.T) {
	session, tab := browser.BrowserSessionID("double-finish"), browser.BrowserTabID("same-tab")
	first, err := BeginAgentTabMutation(context.Background(), session, tab)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if second, err := BeginAgentTabMutation(ctx, session, tab); !errors.Is(err, context.DeadlineExceeded) {
		if second != nil {
			second()
		}
		first()
		t.Fatalf("concurrent Agent write entered: %v", err)
	}
	first()
	next, err := BeginAgentTabMutation(context.Background(), session, tab)
	if err != nil {
		t.Fatal(err)
	}
	first()
	if _, ok := acquireAgentControl("ref", "owner", session, tab); ok {
		next()
		t.Fatal("stale completion released newer Agent write")
	}
	next()
	if _, ok := acquireAgentControl("ref", "owner", session, tab); !ok {
		t.Fatal("Agent completion leaked")
	}
	releaseAgentControl("ref", "owner", session, tab)
}
