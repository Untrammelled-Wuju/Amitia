// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type cascadeTTSConnection struct {
	ctx       context.Context
	cancel    context.CancelFunc
	conn      *websocket.Conn
	writeMu   sync.Mutex
	consumed  atomic.Bool
	closeOnce sync.Once
}

type cascadeTTSSession struct {
	ctx        context.Context
	cancel     context.CancelFunc
	connection *cascadeTTSConnection
	sessionID  string
	done       chan error
	closeOnce  sync.Once
}

func newCascadeTTSConnection(ctx context.Context, cfg cascadeSpeechConfig, voiceID string) (*cascadeTTSConnection, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if !strings.HasPrefix(endpoint, "wss://") && !strings.HasPrefix(endpoint, "ws://") {
		endpoint = defaultCascadeTTSEndpoint
	}
	voiceID = cascadeTTSVoiceID(voiceID)
	cfg.ResourceID = cascadeTTSResourceID(cfg.ResourceID, voiceID)
	headers, err := cascadeTTSHeaders(cfg)
	if err != nil {
		return nil, err
	}
	conn, resp, err := (&websocket.Dialer{HandshakeTimeout: 10 * time.Second}).DialContext(ctx, endpoint, headers)
	if err != nil {
		body := ""
		if resp != nil && resp.Body != nil {
			data, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			body = strings.TrimSpace(string(data))
			_ = resp.Body.Close()
		}
		if body != "" {
			return nil, fmt.Errorf("streaming tts connect failed: %w: %s", err, body)
		}
		return nil, fmt.Errorf("streaming tts connect failed: %w", err)
	}
	connectionCtx, cancel := context.WithCancel(ctx)
	connection := &cascadeTTSConnection{
		ctx:    connectionCtx,
		cancel: cancel,
		conn:   conn,
	}
	handshakeDone := make(chan struct{})
	go func() {
		select {
		case <-connectionCtx.Done():
			_ = conn.Close()
		case <-handshakeDone:
		}
	}()
	defer close(handshakeDone)
	if err := connection.sendJSON(scEventStartConnection, "", map[string]any{"namespace": "BidirectionalTTS"}); err != nil {
		connection.Close()
		return nil, err
	}
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if _, err := scReadExpectedProviderEvent(conn, scEventConnectionStarted, "tts connection handshake"); err != nil {
		connection.Close()
		return nil, err
	}
	_ = conn.SetReadDeadline(time.Time{})
	return connection, nil
}

func (c *cascadeTTSConnection) Start(ctx context.Context, cfg cascadeSpeechConfig, voiceID, language, instruction, sectionID string, onAudio func([]byte)) (*cascadeTTSSession, error) {
	if !c.consumed.CompareAndSwap(false, true) {
		return nil, fmt.Errorf("streaming tts connection is already in use")
	}
	select {
	case <-c.ctx.Done():
		return nil, c.ctx.Err()
	default:
	}
	voiceID = cascadeTTSVoiceID(voiceID)
	cfg.ResourceID = cascadeTTSResourceID(cfg.ResourceID, voiceID)
	model := strings.TrimSpace(cfg.Model)
	if !strings.HasPrefix(model, "seed-tts-2.0-") {
		model = defaultCascadeTTSModel
	}
	additions := map[string]any{
		"context_texts": []string{strings.TrimSpace(instruction)},
		"section_id":    strings.TrimSpace(sectionID),
	}
	if lang := cascadeTTSLanguage(language); lang != "" {
		additions["explicit_language"] = lang
	}
	additionsJSON, err := json.Marshal(additions)
	if err != nil {
		c.Close()
		return nil, err
	}
	sessionCtx, cancel := context.WithCancel(ctx)
	session := &cascadeTTSSession{
		ctx:        sessionCtx,
		cancel:     cancel,
		connection: c,
		sessionID:  uuid.NewString(),
		done:       make(chan error, 1),
	}
	payload := map[string]any{
		"namespace": "BidirectionalTTS",
		"req_params": map[string]any{
			"model":   model,
			"speaker": voiceID,
			"audio_params": map[string]any{
				"format":      "pcm",
				"sample_rate": 24000,
			},
			"additions": string(additionsJSON),
		},
	}
	if err := session.sendJSON(scEventStartSession, session.sessionID, payload); err != nil {
		c.Close()
		return nil, err
	}
	_ = c.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if _, err := scReadExpectedProviderEvent(c.conn, scEventSessionStarted, "tts start session"); err != nil {
		c.Close()
		return nil, err
	}
	_ = c.conn.SetReadDeadline(time.Time{})
	go session.readLoop(onAudio)
	return session, nil
}

func newCascadeTTSSession(ctx context.Context, cfg cascadeSpeechConfig, voiceID, language, instruction, sectionID string, onAudio func([]byte)) (*cascadeTTSSession, error) {
	connection, err := newCascadeTTSConnection(ctx, cfg, voiceID)
	if err != nil {
		return nil, err
	}
	session, err := connection.Start(ctx, cfg, voiceID, language, instruction, sectionID, onAudio)
	if err != nil {
		connection.Close()
		return nil, err
	}
	return session, nil
}

func cascadeTTSHeaders(cfg cascadeSpeechConfig) (http.Header, error) {
	headers := http.Header{}
	apiKey := strings.TrimSpace(cfg.ApiKey)
	accessKey := strings.TrimSpace(cfg.AccessKey)
	appID := strings.TrimSpace(cfg.AppID)
	if apiKey != "" {
		headers.Set("X-Api-Key", apiKey)
	} else if accessKey != "" && appID != "" {
		headers.Set("X-Api-App-Id", appID)
		headers.Set("X-Api-Access-Key", accessKey)
	} else {
		return nil, fmt.Errorf("streaming tts credentials are not configured")
	}
	resourceID := strings.TrimSpace(cfg.ResourceID)
	if resourceID == "" || strings.HasPrefix(resourceID, "volc.speech.") {
		resourceID = defaultCascadeTTSResourceID
	}
	headers.Set("X-Api-Resource-Id", resourceID)
	headers.Set("X-Api-Connect-Id", uuid.NewString())
	return headers, nil
}

func (s *cascadeTTSSession) SendText(text string) error {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return s.sendJSON(scEventTaskRequest, s.sessionID, map[string]any{
		"namespace": "BidirectionalTTS",
		"event":     scEventTaskRequest,
		"req_params": map[string]any{
			"text": text,
		},
	})
}

func (s *cascadeTTSSession) Finish() error {
	return s.sendJSON(scEventFinishSession, s.sessionID, map[string]any{})
}

func (s *cascadeTTSSession) sendJSON(event int32, sessionID string, payload any) error {
	frame, err := scEncodeJSONEvent(event, sessionID, payload)
	if err != nil {
		return err
	}
	s.connection.writeMu.Lock()
	defer s.connection.writeMu.Unlock()
	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	default:
	}
	_ = s.connection.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	err = s.connection.conn.WriteMessage(websocket.BinaryMessage, frame)
	_ = s.connection.conn.SetWriteDeadline(time.Time{})
	return err
}

func (s *cascadeTTSSession) readLoop(onAudio func([]byte)) {
	var result error
	defer func() {
		s.done <- result
		close(s.done)
	}()
	for {
		messageType, data, err := s.connection.conn.ReadMessage()
		if err != nil {
			if s.ctx.Err() == nil {
				result = fmt.Errorf("streaming tts read failed: %w", err)
			}
			return
		}
		if messageType != websocket.BinaryMessage {
			continue
		}
		frame, err := scDecodeFrame(data)
		if err != nil {
			result = fmt.Errorf("streaming tts invalid frame: %w", err)
			return
		}
		payload, payloadErr := frame.decodedPayload()
		if frame.MessageType == scMsgTypeError {
			message := strings.TrimSpace(string(payload))
			if message == "" {
				message = "provider error"
			}
			result = fmt.Errorf("streaming tts error %d: %s", frame.ErrorCode, message)
			return
		}
		if payloadErr != nil {
			result = payloadErr
			return
		}
		if frame.MessageType == scMsgTypeAudioOnlyServer && len(payload) > 0 {
			if onAudio != nil {
				onAudio(append([]byte(nil), payload...))
			}
			continue
		}
		if frame.MessageType != scMsgTypeFullServer || !frame.HasEvent {
			continue
		}
		switch frame.Event {
		case scEventSessionFinished:
			return
		case scEventSessionFailed, scEventConnectionFailed:
			message := strings.TrimSpace(string(payload))
			if message == "" {
				message = fmt.Sprintf("event %d", frame.Event)
			}
			result = fmt.Errorf("streaming tts failed: %s", message)
			return
		}
	}
}

func (s *cascadeTTSSession) Wait(ctx context.Context) error {
	select {
	case err := <-s.done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *cascadeTTSSession) Cancel() {
	s.closeOnce.Do(func() {
		_ = s.sendJSON(scEventCancelSession, s.sessionID, map[string]any{})
		s.cancel()
		s.connection.Close()
	})
}

func (s *cascadeTTSSession) Close() {
	s.closeOnce.Do(func() {
		s.cancel()
		s.connection.Close()
	})
}

func (c *cascadeTTSConnection) sendJSON(event int32, sessionID string, payload any) error {
	frame, err := scEncodeJSONEvent(event, sessionID, payload)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	select {
	case <-c.ctx.Done():
		return c.ctx.Err()
	default:
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	err = c.conn.WriteMessage(websocket.BinaryMessage, frame)
	_ = c.conn.SetWriteDeadline(time.Time{})
	return err
}

func (c *cascadeTTSConnection) Close() {
	c.closeOnce.Do(func() {
		_ = c.sendJSON(scEventFinishConnection, "", map[string]any{"namespace": "BidirectionalTTS"})
		c.cancel()
		_ = c.conn.Close()
	})
}
