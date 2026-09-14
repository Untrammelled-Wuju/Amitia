// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestScFrameRoundTripForEvent(t *testing.T) {
	frame, err := scEncodeEvent(scMsgTypeFullClient, scSerializationJSON, scCompressionNone, scEventStartSession, "session-1", []byte(`{"x":1}`))
	if err != nil {
		t.Fatalf("encode event frame: %v", err)
	}
	decoded, err := scDecodeFrame(frame)
	if err != nil {
		t.Fatalf("decode event frame: %v", err)
	}
	if decoded.MessageType != scMsgTypeFullClient {
		t.Fatalf("unexpected message type: %#x", decoded.MessageType)
	}
	if !decoded.HasEvent || decoded.Event != scEventStartSession {
		t.Fatalf("unexpected event: %d hasEvent=%v", decoded.Event, decoded.HasEvent)
	}
	if decoded.SessionID != "session-1" {
		t.Fatalf("unexpected session id: %q", decoded.SessionID)
	}
	if string(decoded.Payload) != `{"x":1}` {
		t.Fatalf("unexpected payload: %s", string(decoded.Payload))
	}
}

func TestScFrameRoundTripForGzipAudio(t *testing.T) {
	pcm := make([]byte, 640)
	for i := range pcm {
		pcm[i] = byte(i % 251)
	}
	frame, err := scEncodeAudioPacket(scMsgTypeAudioOnlyClient, scFlagLastNoSeq, scSerializationRaw, scCompressionGZIP, pcm)
	if err != nil {
		t.Fatalf("encode audio frame: %v", err)
	}
	decoded, err := scDecodeFrame(frame)
	if err != nil {
		t.Fatalf("decode audio frame: %v", err)
	}
	if decoded.MessageType != scMsgTypeAudioOnlyClient {
		t.Fatalf("unexpected message type: %#x", decoded.MessageType)
	}
	if decoded.HasEvent {
		t.Fatalf("audio-only frame must not carry an event")
	}
	payload, err := decoded.decodedPayload()
	if err != nil {
		t.Fatalf("decode gzip payload: %v", err)
	}
	if len(payload) != len(pcm) {
		t.Fatalf("unexpected payload length: %d", len(payload))
	}
	for i := range pcm {
		if payload[i] != pcm[i] {
			t.Fatalf("payload mismatch at %d", i)
		}
	}
}

func TestScEncodeEventRejectsMissingSessionID(t *testing.T) {
	if _, err := scEncodeEvent(scMsgTypeFullClient, scSerializationJSON, scCompressionNone, scEventStartSession, "", []byte(`{}`)); err == nil {
		t.Fatal("expected error when session id is missing")
	}
}

func TestCascadeVoiceJSONParserStreamsInstructionThenText(t *testing.T) {
	parser := &cascadeVoiceJSONParser{}
	var text string
	for _, ch := range []string{
		`{"interaction_mode":"NORMAL","speech_`,
		`instruction":"温柔一点","speech_text":"你好`,
		`呀，今天过得怎么样？","user_affect":{`,
		`"primary_emotion":"neutral","secondary_emotion":"none","intensity":0,"stress":0,"need":"none","advice_wanted":false,"openness":0.5,"severity":0,"possible_concealment":false,"confidence":0.5,"evidence":["none"]}}`,
	} {
		fragment, _, err := parser.Feed(ch)
		if err != nil {
			t.Fatalf("feed parser: %v", err)
		}
		text += fragment
	}
	if parser.Instruction() != "温柔一点" {
		t.Fatalf("unexpected instruction: %q", parser.Instruction())
	}
	reply, err := parser.Finalize()
	if err != nil {
		t.Fatalf("finalize parser: %v", err)
	}
	if reply.SpeechText != "你好呀，今天过得怎么样？" {
		t.Fatalf("unexpected speech text: %q", reply.SpeechText)
	}
	if text != reply.SpeechText {
		t.Fatalf("streamed text mismatch: %q vs %q", text, reply.SpeechText)
	}
	if reply.InteractionMode != "NORMAL" {
		t.Fatalf("unexpected interaction mode: %q", reply.InteractionMode)
	}
	if reply.UserAffect.PrimaryEmotion != "neutral" {
		t.Fatalf("unexpected user affect: %+v", reply.UserAffect)
	}
}

func TestCascadeVoiceJSONParserStreamsTextBeforeInstruction(t *testing.T) {
	parser := &cascadeVoiceJSONParser{}
	var text string
	for _, chunk := range []string{
		`{"interaction_mode":"NORMAL","speech_text":"你`,
		`好","speech_instruction":"温柔","user_affect":{`,
		`"primary_emotion":"neutral","secondary_emotion":"none","intensity":0,"stress":0,"need":"none","advice_wanted":false,"openness":0.5,"severity":0,"possible_concealment":false,"confidence":0.5,"evidence":["none"]}}`,
	} {
		fragment, _, err := parser.Feed(chunk)
		if err != nil {
			t.Fatalf("feed parser: %v", err)
		}
		text += fragment
	}
	reply, err := parser.Finalize()
	if err != nil {
		t.Fatalf("finalize parser: %v", err)
	}
	if text != "你好" || reply.SpeechInstruction != "温柔" {
		t.Fatalf("unexpected text-first result: text=%q reply=%+v", text, reply)
	}
}

func TestCascadeTurnControllerCommitsAfterSilence(t *testing.T) {
	controller := newCascadeTurnController(defaultCascadeTurnThresholds())
	start := time.Now()
	controller.OnSpeechStart()
	controller.OnPartial("今天天气不错", start)
	controller.OnSpeechEnd(start.Add(200 * time.Millisecond))
	controller.OnFinal("今天天气不错。", start.Add(200*time.Millisecond))

	if decision := controller.Decide(start.Add(250 * time.Millisecond)); decision.Type != cascadeTurnHold {
		t.Fatalf("expected hold before the gap threshold, got %s", decision.Type)
	}
	decision := controller.Decide(start.Add(900 * time.Millisecond))
	if decision.Type != cascadeTurnCommit {
		t.Fatalf("expected commit after silence, got %s", decision.Type)
	}
	if decision.Text != "今天天气不错。" {
		t.Fatalf("unexpected committed text: %q", decision.Text)
	}
	if controller.HasPending() {
		t.Fatal("pending text must be cleared after commit")
	}
}

func TestCascadeTurnControllerCommitsStablePartialBeforeFinal(t *testing.T) {
	controller := newCascadeTurnController(defaultCascadeTurnThresholds())
	start := time.Now()
	controller.OnSpeechStart()
	controller.OnPartial("今天天气不错。", start.Add(100*time.Millisecond))
	controller.OnSpeechEnd(start.Add(300 * time.Millisecond))
	decision := controller.Decide(start.Add(800 * time.Millisecond))
	if decision.Type != cascadeTurnCommit || decision.Text != "今天天气不错。" || !decision.Partial {
		t.Fatalf("expected stable partial commit, got %+v", decision)
	}
	controller.OnFinal("今天天气不错。", start.Add(1000*time.Millisecond))
	if duplicate := controller.Decide(start.Add(1500 * time.Millisecond)); duplicate.Type == cascadeTurnCommit {
		t.Fatalf("late final triggered duplicate commit: %+v", duplicate)
	}
}

func TestCascadeTurnControllerAsksCompletionForIncompleteTurn(t *testing.T) {
	controller := newCascadeTurnController(defaultCascadeTurnThresholds())
	start := time.Now()
	controller.OnSpeechStart()
	controller.OnSpeechEnd(start)
	controller.OnFinal("今天我想说", start)

	if decision := controller.Decide(start.Add(700 * time.Millisecond)); decision.Type != cascadeTurnHold {
		t.Fatalf("expected hold inside the hold window, got %s", decision.Type)
	}
	decision := controller.Decide(start.Add(1400 * time.Millisecond))
	if decision.Type != cascadeTurnBackchannel {
		t.Fatalf("expected backchannel for incomplete turn, got %s", decision.Type)
	}
	decision = controller.Decide(start.Add(2400 * time.Millisecond))
	if decision.Type != cascadeTurnCompletionCheck {
		t.Fatalf("expected completion check, got %s", decision.Type)
	}
}

func TestCascadeTTSTextBufferSplitsOnPunctuation(t *testing.T) {
	buffer := &cascadeTTSTextBuffer{}
	chunks := buffer.Push("你好")
	if len(chunks) != 1 || chunks[0] != "你好" {
		t.Fatalf("expected the first short chunk to be released, got %v", chunks)
	}
	chunks = buffer.Push("，今天怎么样？")
	if len(chunks) != 1 || chunks[0] != "，今天怎么样？" {
		t.Fatalf("unexpected chunks: %v", chunks)
	}
	if tail := buffer.Flush(); len(tail) != 0 {
		t.Fatalf("expected empty flush, got %v", tail)
	}
}

func TestServeCascadeCallFailsWithoutGenerator(t *testing.T) {
	SetCascadeVoiceProvider(nil, nil)
	t.Cleanup(func() { SetCascadeVoiceProvider(nil, nil) })

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/api/realtime/v2/session", nil)

	serveCascadeCall(c, cascadeCallParams{CallID: "call-1", SessionID: "session-1"})

	if recorder.Code != 503 {
		t.Fatalf("expected 503 without generator, got %d", recorder.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["code"] != float64(503) {
		t.Fatalf("unexpected response body: %v", body)
	}
}

func TestCascadeVoiceReadinessRequiresGenerator(t *testing.T) {
	SetCascadeVoiceProvider(nil, nil)
	t.Cleanup(func() { SetCascadeVoiceProvider(nil, nil) })

	if err := CascadeVoiceReadiness(); err == nil {
		t.Fatal("expected readiness to fail without a generator")
	}
}

func TestCascadeAudioEnergyDetectsPCMVoice(t *testing.T) {
	silent := make([]byte, 6400)
	if energy := cascadeAudioEnergy(silent); energy != 0 {
		t.Fatalf("expected zero energy for silence, got %v", energy)
	}

	voiced := make([]byte, 6400)
	for index := 0; index+1 < len(voiced); index += 2 {
		voiced[index] = 0x40
		voiced[index+1] = 0x00
	}
	if energy := cascadeAudioEnergy(voiced); energy < cascadeBargeInEnergyThreshold {
		t.Fatalf("expected voiced energy above threshold, got %v", energy)
	}
}

func TestSetCascadeVoiceProviderAdaptsTurns(t *testing.T) {
	var captured CascadeVoiceGenerationRequest
	SetCascadeVoiceProvider(func(ctx context.Context, req CascadeVoiceGenerationRequest, onDelta func(string) error) error {
		captured = req
		return onDelta("delta")
	}, nil)
	t.Cleanup(func() {
		SetCascadeVoiceProvider(nil, nil)
		SetCascadeRollingSummarizer(nil)
	})

	generator, summarizer, _ := cascadeVoiceGeneratorSnapshot()
	if generator == nil {
		t.Fatal("expected generator to be registered")
	}
	if summarizer != nil {
		t.Fatal("expected no summarizer without an explicit registration")
	}
	req := cascadeVoiceGenerationRequest{
		SystemPrompt:   "sys",
		History:        []cascadeVoiceTurn{{UserText: "在吗", SpeechText: "在的"}},
		RollingSummary: "之前聊了晚饭",
		UserText:       "今天吃什么",
	}
	var deltas []string
	if err := generator(context.Background(), req, func(delta string) error {
		deltas = append(deltas, delta)
		return nil
	}); err != nil {
		t.Fatalf("generator returned error: %v", err)
	}
	if captured.SystemPrompt != "sys" || captured.UserText != "今天吃什么" {
		t.Fatalf("unexpected captured request: %+v", captured)
	}
	if captured.RollingSummary != "之前聊了晚饭" {
		t.Fatalf("unexpected captured rolling summary: %q", captured.RollingSummary)
	}
	if len(captured.History) != 1 || captured.History[0].UserText != "在吗" || captured.History[0].SpeechText != "在的" {
		t.Fatalf("unexpected captured history: %+v", captured.History)
	}
	if len(deltas) != 1 || deltas[0] != "delta" {
		t.Fatalf("unexpected deltas: %v", deltas)
	}
}

func TestSetCascadeRollingSummarizer(t *testing.T) {
	SetCascadeRollingSummarizer(func(ctx context.Context, existing, raw string) (string, error) {
		return existing + "|" + raw, nil
	})
	t.Cleanup(func() { SetCascadeRollingSummarizer(nil) })

	_, summarizer, _ := cascadeVoiceGeneratorSnapshot()
	if summarizer == nil {
		t.Fatal("expected summarizer to be registered")
	}
	summary, err := summarizer(context.Background(), "旧", "新")
	if err != nil {
		t.Fatalf("summarizer returned error: %v", err)
	}
	if summary != "旧|新" {
		t.Fatalf("unexpected summary: %q", summary)
	}
}

func TestCascadeVoiceExpressionPlanMapsEmotionKeywords(t *testing.T) {
	plan := cascadeVoiceExpressionPlanFromInstruction("声音放轻一点，带一点关心")
	if plan.PrimaryEmotion != "concern" {
		t.Fatalf("expected concern, got %q", plan.PrimaryEmotion)
	}
	if plan.Pace > 0.45 {
		t.Fatalf("expected slower pace, got %v", plan.Pace)
	}
	if plan.Warmth < 0.6 {
		t.Fatalf("expected warmer tone, got %v", plan.Warmth)
	}

	angry := cascadeVoiceExpressionPlanFromInstruction("语气更硬，明显不满")
	if angry.PrimaryEmotion != "anger" || angry.Emphasis != "firm" {
		t.Fatalf("unexpected anger plan: %+v", angry)
	}

	realization := cascadeVoiceExpressionPlanFromInstruction("恍然大悟，哦音轻微拉长并上扬")
	if realization.PrimaryEmotion != "surprise" || realization.PauseBeforeMS < 160 || realization.Ending != "slightly_up" {
		t.Fatalf("unexpected realization plan: %+v", realization)
	}

	laugh := cascadeVoiceExpressionPlanFromInstruction("被逗笑，带自然轻笑")
	if laugh.PrimaryEmotion != "joy" || laugh.Laugh < 0.70 || laugh.Smile < 0.78 {
		t.Fatalf("unexpected laugh plan: %+v", laugh)
	}
	laughInstruction := cascadeMapTTSInstruction("自然口语", laugh)
	if !containsSubstring(laughInstruction, "轻笑") {
		t.Fatalf("laugh instruction was not forwarded: %q", laughInstruction)
	}
}

func TestCascadeVoiceReactionRulesCoverCatalog(t *testing.T) {
	for _, want := range []string{
		"中性、平静",
		"倾听、承接",
		"思考、犹豫",
		"没听清、疑惑",
		"惊讶、震惊",
		"恍然大悟",
		"突然想起、明白确认",
		"开心、被逗笑",
		"兴奋",
		"得意、俏皮",
		"害羞、撒娇",
		"调情",
		"温柔关心、心疼",
		"感动、释然",
		"无奈、疲惫",
		"难过、委屈",
		"失望",
		"生气、烦躁",
		"吃醋、冷淡、冲突",
		"担心、焦虑、害怕",
		"心虚、抱歉、孤独",
		"严肃、认真",
		"嫌弃、放松",
		"留空间、结束话题",
		"哦～",
		"哦。",
		"哈哈",
		"噗",
	} {
		if !containsSubstring(cascadeVoiceReactionRules, want) {
			t.Fatalf("cascade reaction rules missing %q", want)
		}
	}
	prompt := buildCascadeVoiceSystemPrompt("角色", "风格", "", "")
	if !containsSubstring(prompt, cascadeVoiceReactionRules) {
		t.Fatal("cascade system prompt must include complete reaction rules")
	}
}

func TestCascadeMapTTSInstructionKeepsSourceAndTail(t *testing.T) {
	plan := cascadeVoiceExpressionPlanFromInstruction("语速稍慢，开心一点")
	instruction := cascadeMapTTSInstruction("角色固定风格", plan)
	if instruction == "" {
		t.Fatal("expected non-empty instruction")
	}
	if !containsSubstring(instruction, "角色固定风格") || !containsSubstring(instruction, "语速稍慢，开心一点") {
		t.Fatalf("instruction lost configured or source directive: %q", instruction)
	}
}

func TestCascadeBaseTTSInstructionUsesPreloadedEmotion(t *testing.T) {
	instruction := cascadeBaseTTSInstruction("角色固定风格", "声音放轻，语速稍慢")
	if !containsSubstring(instruction, "角色固定风格") || !containsSubstring(instruction, "声音放轻") {
		t.Fatalf("unexpected base tts instruction: %q", instruction)
	}
}

func TestCascadePlaybackTrackerComputesPartialDelivery(t *testing.T) {
	tracker := newCascadePlaybackTracker(7)
	tracker.SetTurnIndex(3)
	tracker.SetGeneratedText("第一句。第二句。第三句。")
	tracker.AddAudioBytes(48000)
	tracker.UpdateProgress(500, 1000)

	if tracker.TurnIndex() != 3 {
		t.Fatalf("unexpected turn index: %d", tracker.TurnIndex())
	}
	record, delivered := tracker.Delivery("call-1", true)
	if !record.Interrupted {
		t.Fatal("expected interrupted record")
	}
	if record.DeliveredRatio <= 0 || record.DeliveredRatio >= 1 {
		t.Fatalf("expected partial ratio, got %v", record.DeliveredRatio)
	}
	if delivered == "" {
		t.Fatal("expected a partial delivered text")
	}
	if len([]rune(delivered)) >= len([]rune("第一句。第二句。第三句。")) {
		t.Fatalf("expected delivered text to be truncated, got %q", delivered)
	}
	if !containsSubstring("第一句。第二句。第三句。", delivered) {
		t.Fatalf("delivered text must be a prefix of the generated text, got %q", delivered)
	}
}

func TestCascadePlaybackTrackerFullDeliveryWhenNotInterrupted(t *testing.T) {
	tracker := newCascadePlaybackTracker(8)
	tracker.SetGeneratedText("完整说完了。")
	tracker.AddAudioBytes(48000)
	record, delivered := tracker.Delivery("call-2", false)
	if record.DeliveredRatio != 1 {
		t.Fatalf("expected full ratio, got %v", record.DeliveredRatio)
	}
	if delivered != "完整说完了。" {
		t.Fatalf("unexpected delivered text: %q", delivered)
	}
}

func TestCascadeUserSignalSnapshot(t *testing.T) {
	call := &cascadeCall{}
	start := time.Now().Add(-2 * time.Second)
	call.beginUserSignal(start)
	call.observeAudioEnergy(0.04)
	call.observeAudioEnergy(0.09)
	call.endUserSignal(time.Now())
	call.completeUserUtterance("今天有点累")
	signals := call.signalSnapshot()
	if signals.UtteranceDurationMS < 1500 {
		t.Fatalf("unexpected utterance duration: %d", signals.UtteranceDurationMS)
	}
	if signals.VolumeRMSMean <= 0 || signals.SpeechRateCharsPerSec <= 0 {
		t.Fatalf("signals were not aggregated: %+v", signals)
	}
}

func TestCascadeVoiceJSONParserRequiresUserAffect(t *testing.T) {
	parser := &cascadeVoiceJSONParser{}
	_, _, err := parser.Feed(`{"interaction_mode":"NORMAL","speech_instruction":"自然","speech_text":"你好"}`)
	if err != nil {
		t.Fatalf("feed parser: %v", err)
	}
	if _, err := parser.Finalize(); err == nil {
		t.Fatal("expected user_affect to be required")
	}
}

func TestCascadeContextCompilerCompactsAndSummarizes(t *testing.T) {
	compiler := newCascadeContextCompiler()
	for i := 0; i < 20; i++ {
		index := compiler.AddUser("用户第 " + strconv.Itoa(i) + " 句")
		compiler.SetAssistantAt(index, "助手第 "+strconv.Itoa(i)+" 句")
	}
	turns, rolling := compiler.Snapshot()
	if len(turns) != 12 {
		t.Fatalf("expected the recent window to hold 12 turns, got %d", len(turns))
	}
	if rolling == "" {
		t.Fatal("expected compacted rolling context")
	}
	if turns[0].UserText != "用户第 8 句" {
		t.Fatalf("unexpected oldest recent turn: %q", turns[0].UserText)
	}

	existing, raw, count, ok := compiler.SummaryCandidate(6)
	if !ok || count < 6 || raw == "" {
		t.Fatalf("expected a summary candidate, got ok=%v count=%d raw=%q", ok, count, raw)
	}
	compiler.ApplySemanticSummary("压缩后的摘要", raw, count)
	_, rollingAfter := compiler.Snapshot()
	if !containsSubstring(rollingAfter, "压缩后的摘要") {
		t.Fatalf("expected the semantic summary in the rendered context, got %q", rollingAfter)
	}
	if existing != "" {
		t.Fatalf("expected no pre-existing summary, got %q", existing)
	}
}

func containsSubstring(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
