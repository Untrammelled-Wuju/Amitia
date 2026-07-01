package expression

import (
	"testing"
	"time"

	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/proactive"
	"github.com/u-ai/backend/internal/realtime"
)

func TestAcceptance_TTSVoiceToWeChat_SwitchAndLease(t *testing.T) {
	proactive.GlobalLeaseManager.Reset()
	plan := interaction.ExpressionPlan{
		ID: "acc-plan-1",
		EmotionPresentation: []interaction.EmotionPresentation{
			{Kind: "joy", Intensity: 0.7},
		},
		Tones: []interaction.ExpressionTone{interaction.ExpressionToneWarm},
	}
	vp := MapExpressionToVoice(plan)
	if vp.EmotionTier != VoiceEmotionPositive {
		t.Fatalf("expected positive voice tier for joy, got %s", vp.EmotionTier)
	}
	lease := realtime.AcquireChannelLease("char-acc-1", "conv-acc-1", "voice", "corr-tts-1", proactive.PriorityNormal, 10*time.Second)
	if lease == nil {
		t.Fatal("expected non-nil voice lease")
	}
	if lease.ChannelGroup != "voice" {
		t.Fatalf("expected voice channel group, got %s", lease.ChannelGroup)
	}
	audioReq, traceBytes := BuildAudioRequestWithTrace(vp, "test")
	if audioReq == nil || traceBytes == nil {
		t.Fatal("expected non-nil audio request and trace")
	}
	event := realtime.SwitchChannel("char-acc-1", "voice", "wechat", "tts_complete")
	if event.ToChannel != "wechat" {
		t.Fatalf("expected switch to wechat, got %s", event.ToChannel)
	}
	belief := realtime.GetOrCreateChannelBelief("char-acc-1")
	if belief.GetActiveChannel() != "wechat" {
		t.Fatalf("expected active channel wechat after switch, got %s", belief.GetActiveChannel())
	}
	if belief.GetActiveGroup() != realtime.ChannelGroupText {
		t.Fatalf("expected text group after switch from voice, got %s", belief.GetActiveGroup())
	}
}

func TestAcceptance_VoiceToQQ_FeatureComparison(t *testing.T) {
	proactive.GlobalLeaseManager.Reset()
	plan1 := interaction.ExpressionPlan{
		ID: "acc-plan-2",
		EmotionPresentation: []interaction.EmotionPresentation{
			{Kind: "care", Intensity: 0.6},
		},
	}
	vp1 := MapExpressionToVoice(plan1)
	plan2 := interaction.ExpressionPlan{
		ID: "acc-plan-3",
		EmotionPresentation: []interaction.EmotionPresentation{
			{Kind: "sadness", Intensity: 0.4},
		},
	}
	vp2 := MapExpressionToVoice(plan2)
	if vp1.EmotionTier == vp2.EmotionTier {
		t.Fatal("expected different voice tiers for care vs sadness")
	}
	if vp1.Speed <= vp2.Speed {
		t.Fatal("expected caring speed > negative speed")
	}
	realtime.SwitchChannel("char-acc-2", "voice", "qq", "user_channel_switch")
	belief := realtime.GetOrCreateChannelBelief("char-acc-2")
	if belief.GetActiveChannel() != "qq" {
		t.Fatalf("expected active channel qq, got %s", belief.GetActiveChannel())
	}
	qqPolicy := GetChannelPolicy(ChannelQQ)
	lease := realtime.AcquireChannelLease("char-acc-2", "conv-acc-2", "qq", "corr-qq-1", proactive.PriorityNormal, 10*time.Second)
	if lease.ChannelGroup != "text" {
		t.Fatalf("expected text channel group for qq lease, got %s", lease.ChannelGroup)
	}
	constraints := NewChannelRenderConstraints(ChannelQQ)
	_ = qqPolicy
	_ = constraints
}

func TestAcceptance_WebToQQ_ScopeAndStateVersion(t *testing.T) {
	proactive.GlobalLeaseManager.Reset()
	plan := interaction.ExpressionPlan{
		ID: "acc-plan-4",
		EmotionPresentation: []interaction.EmotionPresentation{
			{Kind: "humor", Intensity: 0.8},
		},
	}
	vp := MapExpressionToVoice(plan)
	if vp.EmotionTier != VoiceEmotionHumorous {
		t.Fatalf("expected humorous tier, got %s", vp.EmotionTier)
	}
	realtime.SwitchChannel("char-acc-3", "web", "qq", "user_moved_to_qq")
	belief := realtime.GetOrCreateChannelBelief("char-acc-3")
	stateVersion := GetChannelStateVersion(ChannelQQ)
	if stateVersion != "qq-state-v1" {
		t.Fatalf("expected qq-state-v1, got %s", stateVersion)
	}
	webPlan := RenderWithChannel(ChannelWeb, plan)
	qqPlan := RenderWithChannel(ChannelQQ, plan)
	if webPlan.Version != qqPlan.Version {
		t.Fatal("expected same plan version across channels")
	}
	_ = belief
	_ = stateVersion
}

func TestAcceptance_DedupAcrossChannels(t *testing.T) {
	proactive.GlobalLeaseManager.Reset()
	proactive.GlobalDedupManager.Reset()
	corrID := "corr-acc-dedup-1"
	proactive.GlobalDedupManager.RecordDelivery(corrID, "char-acc-4", "conv-acc-4", "web", "hello")
	proactive.GlobalDedupManager.MarkSent(corrID, "web")
	if !proactive.GlobalDedupManager.IsDuplicate(corrID, "web") {
		t.Fatal("expected web record to be marked duplicate")
	}
	if proactive.GlobalDedupManager.IsDuplicate(corrID, "wechat") {
		t.Fatal("expected wechat not to be marked duplicate for different channel")
	}
	if !proactive.GlobalDedupManager.HasSentAnyChannel(corrID) {
		t.Fatal("expected HasSentAnyChannel to return true")
	}
	seen := map[string]bool{"web": true}
	channels := proactive.DeliverableChannels("all", seen)
	foundWeb := false
	for _, ch := range channels {
		if ch == "web" {
			foundWeb = true
			break
		}
	}
	if foundWeb {
		t.Fatal("web should be excluded from deliverable channels since already seen")
	}
}

func TestAcceptance_LeaseScopeCoversChannelGroup(t *testing.T) {
	proactive.GlobalLeaseManager.Reset()
	lease1 := realtime.AcquireChannelLease("char-acc-5", "conv-acc-5", "web", "corr-scope-1", proactive.PriorityNormal, 10*time.Second)
	lease2 := realtime.AcquireChannelLease("char-acc-5", "conv-acc-5", "wechat", "corr-scope-2", proactive.PriorityNormal, 10*time.Second)
	allLeases := proactive.GlobalLeaseManager.GetActiveLeases("char-acc-5")
	if len(allLeases) < 2 {
		t.Fatalf("expected at least 2 active leases, got %d", len(allLeases))
	}
	groupLeases := proactive.GetActiveLeasesForGroup("char-acc-5", "text")
	if len(groupLeases) < 2 {
		t.Fatalf("expected at least 2 text group leases, got %d", len(groupLeases))
	}
	voiceLeases := proactive.GetActiveLeasesForGroup("char-acc-5", "voice")
	if len(voiceLeases) != 0 {
		t.Fatalf("expected 0 voice group leases, got %d", len(voiceLeases))
	}
	_ = lease1
	_ = lease2
}

func TestAcceptance_NeutralVoiceFallbackOnUnsupported(t *testing.T) {
	plan := interaction.ExpressionPlan{
		ID: "acc-plan-fallback",
		EmotionPresentation: []interaction.EmotionPresentation{
			{Kind: "joy", Intensity: 0.9},
		},
	}
	vp := MapExpressionToVoiceSafe(plan, false)
	if vp.EmotionTier != VoiceEmotionNeutral {
		t.Fatalf("expected neutral tier when voice unsupported, got %s", vp.EmotionTier)
	}
	if vp.Trace.FallbackReason != "channel_unsupported" {
		t.Fatalf("expected fallback reason channel_unsupported, got %s", vp.Trace.FallbackReason)
	}
	if vp.Intensity != 0.0 {
		t.Fatalf("expected zero intensity for fallback, got %f", vp.Intensity)
	}
}
