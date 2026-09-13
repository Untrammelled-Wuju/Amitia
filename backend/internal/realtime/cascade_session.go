// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	appLog "github.com/u-ai/backend/log"
)

const (
	cascadeBargeInEnergyThreshold = 0.08
	cascadeRollingSummaryMinLines = 6
)

type cascadeVoiceTurn struct {
	UserText   string
	SpeechText string
}

type cascadeVoiceGenerationRequest struct {
	UserID         string
	CharacterID    string
	SystemPrompt   string
	History        []cascadeVoiceTurn
	RollingSummary string
	UserText       string
}

type cascadeVoiceGenerator func(ctx context.Context, req cascadeVoiceGenerationRequest, onDelta func(string) error) error

type cascadeRollingSummarizer func(ctx context.Context, existingSummary, rawTurns string) (string, error)

type CascadeVoiceTurn struct {
	UserText   string
	SpeechText string
}

type CascadeVoiceGenerationRequest struct {
	UserID         string
	CharacterID    string
	SystemPrompt   string
	History        []CascadeVoiceTurn
	RollingSummary string
	UserText       string
}

var cascadeVoiceRegistry struct {
	mu         sync.RWMutex
	generator  cascadeVoiceGenerator
	summarizer cascadeRollingSummarizer
	readiness  func() error
}

func SetCascadeVoiceProvider(gen func(ctx context.Context, req CascadeVoiceGenerationRequest, onDelta func(string) error) error, readiness func() error) {
	cascadeVoiceRegistry.mu.Lock()
	defer cascadeVoiceRegistry.mu.Unlock()
	if gen == nil {
		cascadeVoiceRegistry.generator = nil
	} else {
		cascadeVoiceRegistry.generator = func(ctx context.Context, req cascadeVoiceGenerationRequest, onDelta func(string) error) error {
			history := make([]CascadeVoiceTurn, 0, len(req.History))
			for _, turn := range req.History {
				history = append(history, CascadeVoiceTurn{UserText: turn.UserText, SpeechText: turn.SpeechText})
			}
			return gen(ctx, CascadeVoiceGenerationRequest{UserID: req.UserID, CharacterID: req.CharacterID, SystemPrompt: req.SystemPrompt, History: history, RollingSummary: req.RollingSummary, UserText: req.UserText}, onDelta)
		}
	}
	cascadeVoiceRegistry.readiness = readiness
}

func SetCascadeRollingSummarizer(fn func(ctx context.Context, existingSummary, rawTurns string) (string, error)) {
	cascadeVoiceRegistry.mu.Lock()
	defer cascadeVoiceRegistry.mu.Unlock()
	cascadeVoiceRegistry.summarizer = fn
}

func cascadeVoiceGeneratorSnapshot() (cascadeVoiceGenerator, cascadeRollingSummarizer, func() error) {
	cascadeVoiceRegistry.mu.RLock()
	defer cascadeVoiceRegistry.mu.RUnlock()
	return cascadeVoiceRegistry.generator, cascadeVoiceRegistry.summarizer, cascadeVoiceRegistry.readiness
}

func CascadeVoiceReadiness() error {
	generator, _, readiness := cascadeVoiceGeneratorSnapshot()
	if generator == nil {
		return fmt.Errorf("实时语音生成器未配置")
	}
	if readiness == nil {
		return fmt.Errorf("实时语音模型未就绪")
	}
	if err := readiness(); err != nil {
		return err
	}
	if _, err := resolveCascadeASRConfig(); err != nil {
		return fmt.Errorf("实时语音 ASR 未配置完整: %w", err)
	}
	if _, err := resolveCascadeTTSConfig(""); err != nil {
		return fmt.Errorf("实时语音 TTS 未配置完整: %w", err)
	}
	return nil
}

type cascadeCallParams struct {
	CallID          string
	SessionID       string
	UserID          string
	CharacterID     string
	ConversationID  string
	CharacterName   string
	CharacterBase   string
	SpeakingStyle   string
	VoiceType       string
	Language        string
	VisualEndpoint  string
	VisualTicket    string
	Instruction     string
	DialogID        string
	DesktopPetPhase bool
	DesktopPet      *ContinuousVoiceSession
	Call            *RealtimeCallSession
}

type cascadeCall struct {
	params cascadeCallParams

	writeJSON func(any) error

	asr  *cascadeASRSession
	turn *cascadeTurnController

	asrCfg   cascadeSpeechConfig
	ttsCfg   cascadeSpeechConfig
	voiceID  string
	language string

	generator       cascadeVoiceGenerator
	summarizer      cascadeRollingSummarizer
	emotionProvider CascadeEmotionProvider
	emotionMu       sync.RWMutex
	emotionContext  *CascadeEmotionContext

	generation atomic.Uint64

	activeMu         sync.Mutex
	activeCancel     context.CancelFunc
	activeGeneration uint64

	compiler *cascadeContextCompiler

	trackerMu sync.Mutex
	trackers  map[uint64]*cascadePlaybackTracker

	summaryMu     sync.Mutex
	summaryActive bool

	interruptCount       atomic.Int64
	startedAt            time.Time
	userSpeaking         atomic.Bool
	bargeInFrames        int
	signalMu             sync.Mutex
	speechStartedAt      time.Time
	utteranceDuration    time.Duration
	energySum            float64
	energySamples        int
	lastAssistantEndedAt time.Time
	lastUtteranceChars   int

	asrTurnSequence atomic.Uint64

	visualContext func() string
	desktopPet    *ContinuousVoiceSession
	petSpeaking   atomic.Bool

	callCtx context.Context
}

func (s *cascadeCall) writeEvent(value map[string]any) {
	if s.writeJSON == nil {
		return
	}
	_ = s.writeJSON(value)
}

func (s *cascadeCall) emitPet(phase string) {
	if s.desktopPet == nil {
		return
	}
	emitDesktopPetVoice(context.Background(), s.desktopPet, phase)
}

func (s *cascadeCall) putTracker(gen uint64, tracker *cascadePlaybackTracker) {
	s.trackerMu.Lock()
	s.trackers[gen] = tracker
	s.trackerMu.Unlock()
}

func (s *cascadeCall) takeAllTrackers() []*cascadePlaybackTracker {
	s.trackerMu.Lock()
	defer s.trackerMu.Unlock()
	out := make([]*cascadePlaybackTracker, 0, len(s.trackers))
	for gen, tracker := range s.trackers {
		out = append(out, tracker)
		delete(s.trackers, gen)
	}
	return out
}

func (s *cascadeCall) getTracker(gen uint64) *cascadePlaybackTracker {
	s.trackerMu.Lock()
	defer s.trackerMu.Unlock()
	return s.trackers[gen]
}

func (s *cascadeCall) takeTracker(gen uint64) *cascadePlaybackTracker {
	s.trackerMu.Lock()
	defer s.trackerMu.Unlock()
	tracker := s.trackers[gen]
	delete(s.trackers, gen)
	return tracker
}

func (s *cascadeCall) commitDelivered(tracker *cascadePlaybackTracker, interrupted bool) {
	if tracker == nil {
		return
	}
	record, delivered := tracker.Delivery(s.params.CallID, interrupted)
	appLog.Info(fmt.Sprintf("cascade delivery call=%s turn=%s gen=%d interrupted=%v ratio=%.3f delivered_chars=%d generated_chars=%d",
		record.CallID, record.TurnID, record.GenerationID, record.Interrupted, record.DeliveredRatio, len([]rune(record.DeliveredText)), len([]rune(record.GeneratedText))))
	text := strings.TrimSpace(delivered)
	s.commitEmotion(tracker, text)
	if text == "" {
		return
	}
	s.compiler.SetAssistantAt(tracker.TurnIndex(), text)
}

func (s *cascadeCall) schedulePlaybackCommitFallback(gen uint64) {
	tracker := s.getTracker(gen)
	if tracker == nil {
		return
	}
	delay := time.Duration(tracker.EstimatedDurationMS()+750) * time.Millisecond
	if delay < time.Second {
		delay = time.Second
	}
	time.AfterFunc(delay, func() {
		tracker := s.takeTracker(gen)
		if tracker == nil {
			return
		}
		tracker.UpdateProgress(1<<60, 1<<60)
		s.markAssistantPlaybackEnded(time.Now())
		s.commitDelivered(tracker, false)
	})
}

func (s *cascadeCall) isGenerating() bool {
	s.activeMu.Lock()
	defer s.activeMu.Unlock()
	return s.activeCancel != nil
}

func (s *cascadeCall) interruptActive(reason string) {
	s.activeMu.Lock()
	cancel := s.activeCancel
	gen := s.activeGeneration
	s.activeCancel = nil
	s.activeGeneration = 0
	s.activeMu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	s.turn.MarkInterruptedTurn()
	s.interruptCount.Add(1)
	s.writeEvent(map[string]any{"event": "interrupted", "generationId": strconv.FormatUint(gen, 10), "reason": reason})
	s.emitPet("turn.interrupted")
	s.emitPet("listening.started")
	if gen != 0 {
		s.commitDelivered(s.takeTracker(gen), true)
	}
}

func (s *cascadeCall) handleSpeechStart() {
	now := time.Now()
	s.userSpeaking.Store(true)
	s.bargeInFrames = 0
	s.beginUserSignal(now)
	s.turn.OnSpeechStart()
	if s.isGenerating() {
		s.interruptActive("user_speech")
	}
}

func (s *cascadeCall) handleSpeechEnd() {
	now := time.Now()
	s.userSpeaking.Store(false)
	s.bargeInFrames = 0
	s.endUserSignal(now)
	s.turn.OnSpeechEnd(now)
}

func (s *cascadeCall) handleAudioChunk(pcm []byte) {
	if len(pcm) == 0 {
		return
	}
	energy := cascadeAudioEnergy(pcm)
	if s.userSpeaking.Load() {
		s.observeAudioEnergy(energy)
	}
	if s.isGenerating() {
		if !s.userSpeaking.Load() && energy >= cascadeBargeInEnergyThreshold {
			s.bargeInFrames++
			if s.bargeInFrames >= 3 {
				s.userSpeaking.Store(true)
				s.bargeInFrames = 0
				s.turn.OnSpeechStart()
				s.interruptActive("vad_interrupted")
			}
		} else {
			s.bargeInFrames = 0
		}
	} else {
		s.bargeInFrames = 0
	}
	if err := s.asr.SendPCM(pcm); err != nil {
		appLog.Info("cascade asr push failed:", err)
	}
}

func (s *cascadeCall) handleASREvent(event cascadeASREvent) {
	now := time.Now()
	if event.Final {
		s.turn.OnFinal(event.Text, now)
		return
	}
	s.turn.OnPartial(event.Text, now)
}

func (s *cascadeCall) commitUserTurn(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	s.interruptActive("new_user_turn")
	for _, stale := range s.takeAllTrackers() {
		s.commitDelivered(stale, false)
	}
	s.emitASRFinal(text)
	turnIndex := s.compiler.AddUser(text)
	s.startAssistantTurn(text, turnIndex)
	s.maybeSummarizeContext()
}

func (s *cascadeCall) maybeSummarizeContext() {
	if s.summarizer == nil {
		return
	}
	s.summaryMu.Lock()
	if s.summaryActive {
		s.summaryMu.Unlock()
		return
	}
	existing, raw, count, ok := s.compiler.SummaryCandidate(cascadeRollingSummaryMinLines)
	if !ok {
		s.summaryMu.Unlock()
		return
	}
	s.summaryActive = true
	s.summaryMu.Unlock()

	go func(existingSummary, candidateRaw string, consumed int) {
		defer func() {
			s.summaryMu.Lock()
			s.summaryActive = false
			s.summaryMu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		summary, err := s.summarizer(ctx, existingSummary, candidateRaw)
		if err != nil {
			appLog.Info("cascade rolling summary failed:", err)
			return
		}
		s.compiler.ApplySemanticSummary(summary, candidateRaw, consumed)
	}(existing, raw, count)
}

func (s *cascadeCall) handlePlaybackProgress(gen uint64, playedMS, receivedMS int64) {
	if gen == 0 {
		return
	}
	tracker := s.getTracker(gen)
	if tracker == nil {
		return
	}
	tracker.UpdateProgress(playedMS, receivedMS)
}

func (s *cascadeCall) handlePlaybackEnded(gen uint64) {
	if gen == 0 {
		return
	}
	tracker := s.takeTracker(gen)
	if tracker == nil {
		return
	}
	tracker.UpdateProgress(1<<60, 1<<60)
	s.markAssistantPlaybackEnded(time.Now())
	s.commitDelivered(tracker, false)
}

func (s *cascadeCall) emitASRFinal(text string) {
	sequence := s.asrTurnSequence.Add(1)
	eventID := makeVoiceWorkflowEventID("realtime-asr", fmt.Sprintf("%s\n%d\n%s", s.params.SessionID, sequence, text))
	payload := map[string]any{
		"transcript":     text,
		"eventId":        eventID,
		"sessionId":      s.params.SessionID,
		"callId":         s.params.CallID,
		"conversationId": s.params.ConversationID,
	}
	if visual, source, capturedAt, ok := s.currentVisual(); ok {
		payload["visualContext"] = visual
		payload["visualSource"] = string(source)
		payload["visualCapturedAt"] = capturedAt
	}
	s.writeEvent(map[string]any{"event": "asr_final", "data": payload})
}

func (s *cascadeCall) currentVisual() (string, VisualSourceType, time.Time, bool) {
	if s.params.Call == nil {
		return "", "", time.Time{}, false
	}
	return s.params.Call.LatestVisualContext(8 * time.Second)
}

func (s *cascadeCall) startAssistantTurn(userText string, turnIndex int) {
	gen := s.generation.Add(1)
	turnCtx, turnCancel := context.WithCancel(s.callCtx)

	s.activeMu.Lock()
	if s.activeCancel != nil {
		s.activeCancel()
	}
	s.activeCancel = turnCancel
	s.activeGeneration = gen
	s.activeMu.Unlock()

	tracker := newCascadePlaybackTracker(gen)
	tracker.SetTurnIndex(turnIndex)
	tracker.SetUserText(userText)
	s.completeUserUtterance(userText)
	tracker.SetUserSignals(s.signalSnapshot())
	s.putTracker(gen, tracker)

	s.writeEvent(map[string]any{"event": "thinking", "generationId": strconv.FormatUint(gen, 10)})

	go func() {
		defer func() {
			s.activeMu.Lock()
			if s.activeGeneration == gen {
				s.activeCancel = nil
				s.activeGeneration = 0
			}
			s.activeMu.Unlock()
			turnCancel()
		}()

		parser := &cascadeVoiceJSONParser{}
		buffer := &cascadeTTSTextBuffer{}
		startedThinkingAt := time.Now()
		var ttsMu sync.Mutex
		var tts *cascadeTTSSession
		var speaking atomic.Bool
		var firstDeltaAt atomic.Int64
		var instructionReadyAt atomic.Int64
		var ttsSessionAt atomic.Int64
		var firstAudioAt atomic.Int64

		startTTS := func() error {
			ttsMu.Lock()
			defer ttsMu.Unlock()
			if tts != nil {
				return nil
			}
			instruction := parser.Instruction()
			if strings.TrimSpace(instruction) == "" {
				return fmt.Errorf("speech_instruction is not ready")
			}
			plan := cascadeVoiceExpressionPlanFromInstruction(instruction)
			ttsInstruction := cascadeMapTTSInstruction(s.params.Instruction, plan)
			if voiceInstruction := s.emotionVoiceInstruction(); voiceInstruction != "" {
				ttsInstruction = voiceInstruction + "；" + ttsInstruction
			}
			session, err := newCascadeTTSSession(turnCtx, s.ttsCfg, s.voiceID, s.language, ttsInstruction, s.params.CallID, func(pcm []byte) {
				if len(pcm) == 0 {
					return
				}
				if s.generation.Load() != gen || turnCtx.Err() != nil {
					return
				}
				tracker.AddAudioBytes(len(pcm))
				if speaking.CompareAndSwap(false, true) {
					s.petSpeaking.Store(true)
					s.emitPet("speaking.started")
					s.writeEvent(map[string]any{"event": "speaking", "generationId": strconv.FormatUint(gen, 10)})
					if firstAudioAt.CompareAndSwap(0, time.Now().UnixNano()) {
						appLog.Infof(
							"cascade latency call=%s turn=%d first_delta_ms=%d instruction_ms=%d tts_session_ms=%d first_audio_ms=%d",
							s.params.CallID,
							turnIndex,
							cascadeElapsedMillis(startedThinkingAt, firstDeltaAt.Load()),
							cascadeElapsedMillis(startedThinkingAt, instructionReadyAt.Load()),
							cascadeElapsedMillis(startedThinkingAt, ttsSessionAt.Load()),
							cascadeElapsedMillis(startedThinkingAt, firstAudioAt.Load()),
						)
					}
				}
				s.writeEvent(map[string]any{"event": "audio", "generationId": strconv.FormatUint(gen, 10), "data": base64.StdEncoding.EncodeToString(pcm)})
			})
			if err != nil {
				return err
			}
			ttsSessionAt.CompareAndSwap(0, time.Now().UnixNano())
			tts = session
			return nil
		}
		sendChunks := func(chunks []string) error {
			for _, chunk := range chunks {
				if strings.TrimSpace(chunk) == "" {
					continue
				}
				ttsMu.Lock()
				session := tts
				ttsMu.Unlock()
				if session == nil {
					return fmt.Errorf("tts session is not ready")
				}
				if err := session.SendText(chunk); err != nil {
					return err
				}
			}
			return nil
		}

		turns, rolling := s.compiler.Snapshot()
		if len(turns) > 0 && turnIndex == len(turns)-1 {
			turns = turns[:len(turns)-1]
		}
		systemPrompt := buildCascadeVoiceSystemPrompt(s.params.CharacterBase, s.params.SpeakingStyle, s.visualPromptText(), rolling)
		if prompt := s.emotionPrompt(); prompt != "" {
			systemPrompt += "\n\n" + prompt
		}
		req := cascadeVoiceGenerationRequest{
			UserID:         s.params.UserID,
			CharacterID:    s.params.CharacterID,
			SystemPrompt:   systemPrompt,
			History:        turns,
			RollingSummary: rolling,
			UserText:       userText,
		}

		err := s.generator(turnCtx, req, func(delta string) error {
			if turnCtx.Err() != nil || s.generation.Load() != gen {
				return context.Canceled
			}
			fragment, instructionReady, feedErr := parser.Feed(delta)
			if feedErr != nil {
				return feedErr
			}
			firstDeltaAt.CompareAndSwap(0, time.Now().UnixNano())
			if instructionReady && instructionReadyAt.CompareAndSwap(0, time.Now().UnixNano()) {
				if startErr := startTTS(); startErr != nil {
					return startErr
				}
			}
			if fragment == "" {
				return nil
			}
			s.writeEvent(map[string]any{"event": "assistant_text", "generationId": strconv.FormatUint(gen, 10), "data": map[string]any{"text": fragment, "delta": true}})
			if startErr := startTTS(); startErr != nil {
				return startErr
			}
			return sendChunks(buffer.Push(fragment))
		})

		if err == nil {
			reply, finalErr := parser.Finalize()
			if finalErr != nil {
				err = finalErr
			} else {
				if s.generation.Load() == gen && turnCtx.Err() == nil {
					s.writeEvent(map[string]any{"event": "assistant_text", "generationId": strconv.FormatUint(gen, 10), "data": map[string]any{"text": reply.SpeechText, "delta": false, "interactionMode": reply.InteractionMode}})
				}
				if reply.InteractionMode == "WAIT" {
					if s.generation.Load() == gen && turnCtx.Err() == nil {
						s.writeEvent(map[string]any{"event": "listening"})
					}
				} else {
					if tts == nil {
						err = startTTS()
					}
					if err == nil {
						err = sendChunks(buffer.Flush())
					}
					if err == nil {
						err = tts.Finish()
					}
					if err == nil {
						err = tts.Wait(turnCtx)
					}
					if err == nil {
						tracker.SetGeneratedText(reply.SpeechText)
						tracker.SetUserAffect(reply.UserAffect)
						s.compiler.SetAssistantAt(turnIndex, reply.SpeechText)
					}
					if err == nil && s.generation.Load() == gen && turnCtx.Err() == nil {
						s.writeEvent(map[string]any{"event": "tts_ended", "generationId": strconv.FormatUint(gen, 10)})
						s.schedulePlaybackCommitFallback(gen)
					}
				}
			}
		}

		ttsMu.Lock()
		if tts != nil {
			if err != nil || turnCtx.Err() != nil {
				tts.Cancel()
			} else {
				tts.Close()
			}
		}
		ttsMu.Unlock()

		if speaking.Load() {
			s.petSpeaking.Store(false)
			s.emitPet("speaking.ended")
		}

		if err != nil && turnCtx.Err() == nil && s.generation.Load() == gen {
			appLog.Warn("cascade assistant turn failed:", err.Error())
			s.writeEvent(map[string]any{"event": "turn_error", "data": err.Error(), "generationId": strconv.FormatUint(gen, 10)})
			s.writeEvent(map[string]any{"event": "listening"})
		}
	}()
}

func (s *cascadeCall) visualPromptText() string {
	if s.visualContext == nil {
		return ""
	}
	return s.visualContext()
}

func (s *cascadeCall) speakBackchannel(text string) {
	text = strings.TrimSpace(text)
	if text == "" || s.isGenerating() {
		return
	}
	gen := s.generation.Add(1)
	s.writeEvent(map[string]any{"event": "backchannel", "text": text, "generationId": strconv.FormatUint(gen, 10)})
	go func() {
		ctx, cancel := context.WithTimeout(s.callCtx, 8*time.Second)
		defer cancel()
		var speaking atomic.Bool
		session, err := newCascadeTTSSession(ctx, s.ttsCfg, s.voiceID, s.language, cascadeMapTTSInstruction(s.params.Instruction, cascadeVoiceExpressionPlanFromInstruction("非常短促自然的倾听回应，不要像正式回答，不要抢话")), s.params.CallID, func(pcm []byte) {
			if len(pcm) == 0 || ctx.Err() != nil || s.generation.Load() != gen {
				return
			}
			if speaking.CompareAndSwap(false, true) {
				s.writeEvent(map[string]any{"event": "speaking", "generationId": strconv.FormatUint(gen, 10), "microReaction": true})
			}
			s.writeEvent(map[string]any{"event": "audio", "generationId": strconv.FormatUint(gen, 10), "microReaction": true, "data": base64.StdEncoding.EncodeToString(pcm)})
		})
		if err != nil {
			appLog.Info("cascade backchannel tts failed:", err)
			return
		}
		if err := session.SendText(text); err != nil {
			session.Cancel()
			return
		}
		if err := session.Finish(); err != nil {
			session.Cancel()
			return
		}
		if err := session.Wait(ctx); err != nil {
			session.Cancel()
			return
		}
		session.Close()
		if s.generation.Load() == gen && ctx.Err() == nil {
			s.writeEvent(map[string]any{"event": "tts_ended", "generationId": strconv.FormatUint(gen, 10), "microReaction": true})
			s.writeEvent(map[string]any{"event": "listening"})
		}
	}()
}

func serveCascadeCall(c *gin.Context, params cascadeCallParams) {
	generator, summarizer, readiness := cascadeVoiceGeneratorSnapshot()
	if generator == nil {
		c.JSON(503, gin.H{"code": 503, "message": "realtime voice generator is not configured"})
		return
	}
	if readiness != nil {
		if readyErr := readiness(); readyErr != nil {
			c.JSON(503, gin.H{"code": 503, "message": readyErr.Error()})
			return
		}
	}
	asrCfg, err := resolveCascadeASRConfig()
	if err != nil {
		c.JSON(503, gin.H{"code": 503, "message": err.Error()})
		return
	}
	ttsCfg, err := resolveCascadeTTSConfig(params.VoiceType)
	if err != nil {
		c.JSON(503, gin.H{"code": 503, "message": err.Error()})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(2 * 1024 * 1024)
	defer conn.Close()

	callCtx, cancelCall := context.WithCancel(c.Request.Context())
	defer cancelCall()

	var writeMu sync.Mutex
	writeJSON := func(value any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		if payload, ok := value.(map[string]any); ok {
			payload["event_id"] = uuid.NewString()
			payload["session_id"] = params.SessionID
			payload["user_id"] = params.UserID
			payload["character_id"] = params.CharacterID
		}
		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		writeErr := conn.WriteJSON(value)
		_ = conn.SetWriteDeadline(time.Time{})
		return writeErr
	}

	asr, err := newCascadeASRSession(callCtx, asrCfg)
	if err != nil {
		_ = writeJSON(map[string]any{"event": "error", "data": err.Error()})
		return
	}
	defer asr.Close()

	session := &cascadeCall{
		params:          params,
		writeJSON:       writeJSON,
		asr:             asr,
		turn:            newCascadeTurnController(defaultCascadeTurnThresholds()),
		asrCfg:          asrCfg,
		ttsCfg:          ttsCfg,
		voiceID:         cascadeTTSVoiceID(ttsCfg.VoiceType),
		language:        params.Language,
		generator:       generator,
		summarizer:      summarizer,
		emotionProvider: cascadeEmotionProviderSnapshot(),
		compiler:        newCascadeContextCompiler(),
		trackers:        make(map[uint64]*cascadePlaybackTracker),
		startedAt:       time.Now(),
		callCtx:         callCtx,
		desktopPet:      params.DesktopPet,
	}
	if strings.TrimSpace(session.language) == "" {
		session.language = "zh-CN"
	}
	session.loadEmotionContext(callCtx)

	if params.Call != nil && realtimeVisualAnalyzer != nil {
		visualPipeline := NewVisualPipeline(callCtx, params.Call, realtimeVisualAnalyzer)
		realtimeCallRegistry.Add(&callRuntime{call: params.Call, pipeline: visualPipeline})
		defer func() {
			realtimeCallRegistry.Remove(params.CallID)
		}()
		go func() {
			for {
				select {
				case <-callCtx.Done():
					return
				case update, ok := <-visualPipeline.Updates():
					if !ok {
						return
					}
					_ = writeJSON(map[string]any{"event": "vision.updated", "data": update})
				case visualErr, ok := <-visualPipeline.Errors():
					if !ok {
						return
					}
					_ = writeJSON(map[string]any{"event": "vision.status", "data": gin.H{"available": false, "message": visualErr.Error()}})
				}
			}
		}()
	}

	if err := writeJSON(map[string]any{
		"event": "connected",
		"data":  "ok",
		"call": map[string]any{
			"callId":         params.CallID,
			"sessionId":      params.SessionID,
			"visualEndpoint": params.VisualEndpoint,
			"visualTicket":   params.VisualTicket,
			"capabilities":   params.Call.Capabilities,
			"sources":        params.Call.Sources,
		},
	}); err != nil {
		return
	}

	session.emitPet("session.started")
	session.emitPet("listening.started")

	go func() {
		<-callCtx.Done()
		_ = conn.Close()
	}()

	go func() {
		for {
			select {
			case <-callCtx.Done():
				return
			case event, ok := <-asr.Events():
				if !ok {
					return
				}
				session.handleASREvent(event)
			case err, ok := <-asr.Errors():
				if !ok {
					return
				}
				if err != nil {
					_ = writeJSON(map[string]any{"event": "error", "data": err.Error()})
				}
				cancelCall()
				return
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-callCtx.Done():
				return
			case now := <-ticker.C:
				decision := session.turn.Decide(now)
				switch decision.Type {
				case cascadeTurnCommit:
					session.commitUserTurn(decision.Text)
				case cascadeTurnBackchannel, cascadeTurnCompletionCheck:
					session.speakBackchannel(decision.Text)
				}
			}
		}
	}()

	defer func() {
		session.interruptActive("call_end")
		if session.petSpeaking.Load() {
			session.petSpeaking.Store(false)
			session.emitPet("speaking.ended")
		}
		session.emitPet("session.ended")
	}()

	for {
		var message map[string]any
		if err := conn.ReadJSON(&message); err != nil {
			return
		}
		eventName, _ := message["event"].(string)
		switch eventName {
		case "audio":
			encoded, _ := message["data"].(string)
			if encoded == "" {
				continue
			}
			pcm, decodeErr := base64.StdEncoding.DecodeString(encoded)
			if decodeErr != nil || len(pcm) == 0 {
				continue
			}
			session.handleAudioChunk(pcm)
		case "speech_start":
			session.handleSpeechStart()
		case "speech_end":
			session.handleSpeechEnd()
		case "stop":
			return
		case "playback_progress":
			gen := cascadeMessageUint(message, "generationId", "generation_id")
			session.handlePlaybackProgress(gen, cascadeMessageInt(message, "played_audio_ms"), cascadeMessageInt(message, "received_audio_ms"))
		case "tts_playback_ended":
			session.handlePlaybackEnded(cascadeMessageUint(message, "generationId", "generation_id"))
		case "ping":
			_ = writeJSON(map[string]any{"event": "pong"})
		case "media.sources":
			if params.Call == nil {
				continue
			}
			sources := params.Call.Sources
			if data, ok := message["data"].(map[string]any); ok {
				if value, ok := data["audio"].(bool); ok {
					sources.Audio = value
				}
				if value, ok := data["camera"].(bool); ok {
					sources.Camera = value
				}
				if value, ok := data["screen"].(bool); ok {
					sources.Screen = value
				}
			}
			params.Call.SetSources(sources)
			_ = writeJSON(map[string]any{"event": "media.sources.updated", "data": sources})
		}
	}
}

func cascadeMessageUint(message map[string]any, keys ...string) uint64 {
	for _, key := range keys {
		value, ok := message[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case float64:
			if typed < 0 {
				return 0
			}
			return uint64(typed)
		case string:
			parsed, err := strconv.ParseUint(strings.TrimSpace(typed), 10, 64)
			if err == nil {
				return parsed
			}
		}
	}
	return 0
}

func cascadeMessageInt(message map[string]any, key string) int64 {
	value, ok := message[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err == nil {
			return parsed
		}
	}
	return 0
}

func cascadeElapsedMillis(start time.Time, endNanos int64) int64 {
	if endNanos == 0 {
		return -1
	}
	return time.Unix(0, endNanos).Sub(start).Milliseconds()
}
