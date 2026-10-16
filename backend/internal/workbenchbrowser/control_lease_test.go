package workbenchbrowser

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/browser"
)

func TestAgentControlLeaseExclusiveAndRevocation(t *testing.T) {
	session, tab := browser.BrowserSessionID("control-session"), browser.BrowserTabID("control-tab")
	ref := ExposeAgentTab("lease-space", "lease-conversation", session, tab)
	if ref == "" {
		t.Fatal("cannot expose tab")
	}
	defer RevokeAgentTab(ref)
	if _, ok := acquireAgentControl(ref, "owner-a", session, tab); !ok {
		t.Fatal("first owner denied")
	}
	if !hasAgentControl(ref, "owner-a", session, tab) {
		t.Fatal("lease not visible")
	}
	if _, ok := acquireAgentControl(ref, "owner-b", session, tab); ok {
		t.Fatal("two operators on one tab")
	}
	if releaseAgentControl(ref, "owner-b", session, tab) {
		t.Fatal("foreign owner released lease")
	}
	if !releaseAgentControl(ref, "owner-a", session, tab) {
		t.Fatal("owner release failed")
	}
	if _, ok := acquireAgentControl(ref, "owner-b", session, tab); !ok {
		t.Fatal("new owner denied")
	}
	RevokeAgentTab(ref)
	if currentAgentControl(session, tab) {
		t.Fatal("revoked agent reference retained a lease")
	}
}

func TestControlLeaseExpirationAndWaitCancellation(t *testing.T) {
	session, tab := browser.BrowserSessionID("expire-session"), browser.BrowserTabID("expire-tab")
	if _, ok := acquireAgentControl("expire-ref", "operator", session, tab); !ok {
		t.Fatal("acquire failed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := WaitForAgentTabControl(ctx, session, tab); !errors.Is(err, context.Canceled) {
		t.Fatalf("want cancellation, got %v", err)
	}
	agentControlLeases.Lock()
	lease := agentControlLeases.items[agentControlTabKey(session, tab)]
	lease.deadline = time.Now().Add(-time.Second)
	agentControlLeases.items[agentControlTabKey(session, tab)] = lease
	agentControlLeases.Unlock()
	if err := WaitForAgentTabControl(context.Background(), session, tab); err != nil {
		t.Fatalf("expired lease blocks Agent: %v", err)
	}
	if currentAgentControl(session, tab) {
		t.Fatal("expired lease still active")
	}
}

func TestAgentToolAndUserLeaseCannotOverlap(t *testing.T) {
	session, tab := browser.BrowserSessionID("inflight-session"), browser.BrowserTabID("inflight-tab")
	done, err := BeginAgentTabMutation(context.Background(), session, tab)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := acquireAgentControl("ref", "user", session, tab); ok {
		t.Fatal("user preempted in-flight Agent tool")
	}
	done()
	if _, ok := acquireAgentControl("ref", "user", session, tab); !ok {
		t.Fatal("user cannot acquire after tool completion")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if finish, err := BeginAgentTabMutation(ctx, session, tab); !errors.Is(err, context.DeadlineExceeded) {
		if finish != nil {
			finish()
		}
		t.Fatalf("Agent ran through manual lease, error=%v", err)
	}
	releaseAgentControl("ref", "user", session, tab)
	finish, err := BeginAgentTabMutation(context.Background(), session, tab)
	if err != nil {
		t.Fatal(err)
	}
	finish()
}

func TestManualTabMutationIsSingleWriter(t *testing.T) {
	session, tab := browser.BrowserSessionID("exclusive-manual-session"), browser.BrowserTabID("exclusive-manual-tab")
	if _, ok := acquireAgentControl("exclusive-ref", "owner-one", session, tab); !ok {
		t.Fatal("owner could not acquire lease")
	}
	defer releaseAgentControl("exclusive-ref", "owner-one", session, tab)
	finish, ok := BeginManualTabMutation("exclusive-ref", "owner-one", agentControlLeaseToken("exclusive-ref", "owner-one", session, tab), session, tab)
	if !ok {
		t.Fatal("first writer denied")
	}
	if second, allowed := BeginManualTabMutation("exclusive-ref", "owner-one", agentControlLeaseToken("exclusive-ref", "owner-one", session, tab), session, tab); allowed {
		second()
		finish()
		t.Fatal("same observer obtained concurrent write access")
	}
	if _, ok := acquireAgentControl("exclusive-ref", "owner-two", session, tab); ok {
		finish()
		t.Fatal("second owner acquired during an active write")
	}
	if !releaseAgentControl("exclusive-ref", "owner-one", session, tab) {
		finish()
		t.Fatal("release failed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if agentFinish, err := BeginAgentTabMutation(ctx, session, tab); !errors.Is(err, context.DeadlineExceeded) {
		if agentFinish != nil {
			agentFinish()
		}
		finish()
		t.Fatalf("Agent entered while manual operation was active: %v", err)
	}
	finish()
	finish()
	if agentFinish, err := BeginAgentTabMutation(context.Background(), session, tab); err != nil {
		t.Fatal(err)
	} else {
		agentFinish()
	}
}

func TestAgentTabMutationsSerializeAndRespectCancellation(t *testing.T) {
	session, tab := browser.BrowserSessionID("serialized-session"), browser.BrowserTabID("serialized-tab")
	finish, err := BeginAgentTabMutation(context.Background(), session, tab)
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	otherTab, err := BeginAgentTabMutation(context.Background(), session, browser.BrowserTabID("different-tab"))
	if err != nil {
		t.Fatalf("unrelated tab blocked: %v", err)
	}
	otherTab()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	if second, err := BeginAgentTabMutation(ctx, session, tab); !errors.Is(err, context.DeadlineExceeded) {
		if second != nil {
			second()
		}
		t.Fatalf("second agent mutated same tab, err=%v", err)
	}
	if _, granted := acquireAgentControl("ref-serialized", "owner-serialized", session, tab); granted {
		t.Fatal("manual control acquired during active agent operation")
	}
	finish()
	finish()
	next, err := BeginAgentTabMutation(context.Background(), session, tab)
	if err != nil {
		t.Fatal(err)
	}
	next()
	if _, granted := acquireAgentControl("ref-serialized", "owner-serialized", session, tab); !granted {
		t.Fatal("agent completion did not release manual control gate")
	}
	defer releaseAgentControl("ref-serialized", "owner-serialized", session, tab)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel2()
	if blocked, err := BeginAgentTabMutation(ctx2, session, tab); !errors.Is(err, context.DeadlineExceeded) {
		if blocked != nil {
			blocked()
		}
		t.Fatalf("manual lease did not block agent: %v", err)
	}
}

func TestAgentTabMutationConcurrentSingleWriter(t *testing.T) {
	session, tab := browser.BrowserSessionID("single-agent-session"), browser.BrowserTabID("single-agent-tab")
	first, err := BeginAgentTabMutation(context.Background(), session, tab)
	if err != nil {
		t.Fatal(err)
	}
	acquired := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go func() {
		done, err := BeginAgentTabMutation(ctx, session, tab)
		if err == nil {
			done()
		}
		acquired <- err
	}()
	select {
	case err := <-acquired:
		first()
		t.Fatalf("concurrent agent entered before first completed: %v", err)
	case <-time.After(40 * time.Millisecond):
	}
	first()
	if err := <-acquired; err != nil {
		t.Fatalf("waiting agent did not resume: %v", err)
	}
}

func TestControlLeaseFencingRejectsLateOwnerReleaseAndRenew(t *testing.T) {
	session := browser.BrowserSessionID("fencing-session")
	tab := browser.BrowserTabID("fencing-tab")
	_, oldToken, ok := acquireAgentControlWithToken("ref", "same-owner", session, tab)
	if !ok || len(oldToken) < 24 {
		t.Fatal("first lease was not generated")
	}
	_, nextToken, ok := acquireAgentControlWithToken("ref", "same-owner", session, tab)
	if !ok || nextToken == oldToken {
		t.Fatal("reacquiring a lease did not rotate its generation")
	}
	if releaseAgentControlWithToken("ref", "same-owner", oldToken, session, tab) {
		t.Fatal("stale release revoked newer lease")
	}
	if _, renewed := renewAgentControl("ref", "same-owner", oldToken, session, tab); renewed {
		t.Fatal("stale renewal extended newer lease")
	}
	if token := agentControlLeaseToken("ref", "same-owner", session, tab); token != nextToken {
		t.Fatal("lease token changed after stale operation")
	}
	if !currentAgentControl(session, tab) {
		t.Fatal("stale release removed current control")
	}
	if _, renewed := renewAgentControl("ref", "same-owner", nextToken, session, tab); !renewed {
		t.Fatal("active lease cannot renew")
	}
	if releaseAgentControlWithToken("ref", "another-owner", nextToken, session, tab) {
		t.Fatal("foreign lease revoked")
	}
	if !releaseAgentControlWithToken("ref", "same-owner", nextToken, session, tab) {
		t.Fatal("current owner cannot release")
	}
	if _, renewed := renewAgentControl("ref", "same-owner", nextToken, session, tab); renewed {
		t.Fatal("released lease renewed")
	}
	finish, err := BeginAgentTabMutation(context.Background(), session, tab)
	if err != nil {
		t.Fatal(err)
	}
	finish()
}

func TestControlLeaseExpiredGenerationCannotBeRenewed(t *testing.T) {
	session := browser.BrowserSessionID("expired-generation-session")
	tab := browser.BrowserTabID("expired-generation-tab")
	_, token, ok := acquireAgentControlWithToken("ref", "owner", session, tab)
	if !ok {
		t.Fatal("cannot acquire")
	}
	key := agentControlTabKey(session, tab)
	agentControlLeases.Lock()
	lease := agentControlLeases.items[key]
	lease.deadline = time.Now().Add(-time.Second)
	agentControlLeases.items[key] = lease
	agentControlLeases.Unlock()
	if _, ok := renewAgentControl("ref", "owner", token, session, tab); ok {
		t.Fatal("expired generation was renewed")
	}
	_, rotated, ok := acquireAgentControlWithToken("ref", "owner", session, tab)
	if !ok || rotated == token {
		t.Fatal("expired generation not rotated")
	}
	defer releaseAgentControlWithToken("ref", "owner", rotated, session, tab)
	if releaseAgentControlWithToken("ref", "owner", token, session, tab) {
		t.Fatal("expired generation released active lease")
	}
}

func TestManualMutationRequiresCurrentGenerationToken(t *testing.T) {
	session := browser.BrowserSessionID("manual-fenced-session")
	tab := browser.BrowserTabID("manual-fenced-tab")
	ref, owner := "fenced-ref", "fenced-owner"
	_, oldToken, ok := acquireAgentControlWithToken(ref, owner, session, tab)
	if !ok {
		t.Fatal("first generation unavailable")
	}
	if finish, permitted := BeginManualTabMutation(ref, owner, "", session, tab); permitted {
		finish()
		t.Fatal("missing lease token accepted")
	}
	_, newToken, ok := acquireAgentControlWithToken(ref, owner, session, tab)
	if !ok || oldToken == newToken {
		t.Fatal("lease generation not rotated")
	}
	if finish, permitted := BeginManualTabMutation(ref, owner, oldToken, session, tab); permitted {
		finish()
		t.Fatal("late mutation under old lease accepted")
	}
	if finish, permitted := BeginManualTabMutation(ref, "other-owner", newToken, session, tab); permitted {
		finish()
		t.Fatal("foreign owner used lease")
	}
	finish, permitted := BeginManualTabMutation(ref, owner, newToken, session, tab)
	if !permitted {
		t.Fatal("current generation mutation denied")
	}
	if releaseAgentControlWithToken(ref, owner, oldToken, session, tab) {
		t.Fatal("old lease released new generation")
	}
	if !releaseAgentControlWithToken(ref, owner, newToken, session, tab) {
		t.Fatal("new lease release denied")
	}
	if next, granted := BeginManualTabMutation(ref, owner, newToken, session, tab); granted {
		next()
		t.Fatal("released lease reused")
	}
	finish()
	next, err := BeginAgentTabMutation(context.Background(), session, tab)
	if err != nil {
		t.Fatal(err)
	}
	next()
}
