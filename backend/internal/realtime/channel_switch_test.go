package realtime

import (
	"testing"
	"time"

	"github.com/u-ai/backend/internal/proactive"
)

func TestResolveChannelGroup(t *testing.T) {
	if resolveChannelGroup("wechat") != ChannelGroupText {
		t.Fatal("expected wechat to be text group")
	}
	if resolveChannelGroup("qq") != ChannelGroupText {
		t.Fatal("expected qq to be text group")
	}
	if resolveChannelGroup("web") != ChannelGroupText {
		t.Fatal("expected web to be text group")
	}
	if resolveChannelGroup("voice") != ChannelGroupVoice {
		t.Fatal("expected voice to be voice group")
	}
	if resolveChannelGroup("tts") != ChannelGroupVoice {
		t.Fatal("expected tts to be voice group")
	}
	if resolveChannelGroup("unknown") != ChannelGroupAll {
		t.Fatal("expected unknown to be all group")
	}
}

func TestChannelBelief_GetOrCreate(t *testing.T) {
	belief := GetOrCreateChannelBelief("char-chan-1")
	if belief.ActiveChannel != "web" {
		t.Fatalf("expected default channel web, got %s", belief.ActiveChannel)
	}
	if belief.ActiveGroup != ChannelGroupText {
		t.Fatalf("expected default group text, got %s", belief.ActiveGroup)
	}
	belief2 := GetOrCreateChannelBelief("char-chan-1")
	if belief != belief2 {
		t.Fatal("expected same belief from cache")
	}
}

func TestSwitchChannel_UpdatesBelief(t *testing.T) {
	belief := GetOrCreateChannelBelief("char-chan-2")
	event := SwitchChannel("char-chan-2", "web", "voice", "user_request")
	if event.FromChannel != "web" || event.ToChannel != "voice" {
		t.Fatalf("expected switch web->voice, got %s->%s", event.FromChannel, event.ToChannel)
	}
	if event.FromGroup != ChannelGroupText || event.ToGroup != ChannelGroupVoice {
		t.Fatalf("expected group text->voice, got %s->%s", event.FromGroup, event.ToGroup)
	}
	if belief.GetActiveChannel() != "voice" {
		t.Fatalf("expected active channel voice, got %s", belief.GetActiveChannel())
	}
	if belief.GetActiveGroup() != ChannelGroupVoice {
		t.Fatalf("expected active group voice, got %s", belief.GetActiveGroup())
	}
	if len(belief.SwitchHistory) < 1 {
		t.Fatal("expected at least 1 switch in history")
	}
}

func TestSwitchChannel_HistoryCapped(t *testing.T) {
	belief := GetOrCreateChannelBelief("char-chan-3")
	for i := 0; i < 25; i++ {
		SwitchChannel("char-chan-3", "web", "qq", "test")
		SwitchChannel("char-chan-3", "qq", "web", "test")
	}
	belief.mu.RLock()
	hlen := len(belief.SwitchHistory)
	belief.mu.RUnlock()
	if hlen > 20 {
		t.Fatalf("expected history capped at 20, got %d", hlen)
	}
}

func TestAcquireChannelLease(t *testing.T) {
	proactive.GlobalLeaseManager.Reset()
	lease := AcquireChannelLease("char-lease-1", "conv-1", "voice", "corr-voice-1", proactive.PriorityNormal, 10*time.Second)
	if lease == nil {
		t.Fatal("expected non-nil lease")
	}
	if lease.ChannelGroup != "voice" {
		t.Fatalf("expected channel group voice, got %s", lease.ChannelGroup)
	}
	if proactive.GlobalLeaseManager.CountActive("char-lease-1") != 1 {
		t.Fatalf("expected 1 active lease, got %d", proactive.GlobalLeaseManager.CountActive("char-lease-1"))
	}
}

func TestCancelLowPriorityLeasesOnUserInput(t *testing.T) {
	proactive.GlobalLeaseManager.Reset()
	AcquireChannelLease("char-lp-1", "conv-1", "web", "corr-lp-1", proactive.PriorityLow, 30*time.Second)
	AcquireChannelLease("char-lp-1", "conv-1", "web", "corr-lp-2", proactive.PriorityNormal, 30*time.Second)
	cancelled := CancelLowPriorityLeasesOnUserInput("char-lp-1")
	if cancelled != 1 {
		t.Fatalf("expected 1 cancelled low-priority lease, got %d", cancelled)
	}
	active := proactive.GlobalLeaseManager.CountActive("char-lp-1")
	if active != 1 {
		t.Fatalf("expected 1 active lease remaining, got %d", active)
	}
}

func TestCancelLeasesForChannelGroup(t *testing.T) {
	proactive.GlobalLeaseManager.Reset()
	AcquireChannelLease("char-cg-1", "conv-1", "web", "corr-cg-1", proactive.PriorityNormal, 30*time.Second)
	AcquireChannelLease("char-cg-1", "conv-1", "voice", "corr-cg-2", proactive.PriorityNormal, 30*time.Second)
	cancelled := CancelLeasesForChannelGroup("char-cg-1", ChannelGroupVoice)
	if cancelled != 1 {
		t.Fatalf("expected 1 cancelled voice lease, got %d", cancelled)
	}
	active := proactive.GlobalLeaseManager.CountActive("char-cg-1")
	if active != 1 {
		t.Fatalf("expected 1 active lease remaining, got %d", active)
	}
}

func TestHasActiveLeaseForChannel(t *testing.T) {
	proactive.GlobalLeaseManager.Reset()
	if HasActiveLeaseForChannel("char-has-1", "web") {
		t.Fatal("expected no active lease initially")
	}
	AcquireChannelLease("char-has-1", "conv-1", "web", "corr-has-1", proactive.PriorityNormal, 30*time.Second)
	if !HasActiveLeaseForChannel("char-has-1", "web") {
		t.Fatal("expected active lease for web")
	}
	if HasActiveLeaseForChannel("char-has-1", "voice") {
		t.Fatal("expected no active lease for voice")
	}
}
