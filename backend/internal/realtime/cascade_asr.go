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

const cascadeASRChunkBytes = 6400

type cascadeASREvent struct {
	Text         string
	Final        bool
	StartTime    int64
	EndTime      int64
	StablePrefix string
}

type cascadeASRSession struct {
	ctx          context.Context
	cancel       context.CancelFunc
	conn         *websocket.Conn
	writeMu      sync.Mutex
	bufferMu     sync.Mutex
	buffer       []byte
	events       chan cascadeASREvent
	errors       chan error
	done         chan struct{}
	closeOnce    sync.Once
	lastPacketAt atomic.Int64
}

func newCascadeASRSession(ctx context.Context, cfg cascadeSpeechConfig) (*cascadeASRSession, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if !strings.HasPrefix(endpoint, "wss://") && !strings.HasPrefix(endpoint, "ws://") {
		endpoint = defaultCascadeASREndpoint
	}
	headers, err := cascadeASRHeaders(cfg)
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
			return nil, fmt.Errorf("streaming asr connect failed: %w: %s", err, body)
		}
		return nil, fmt.Errorf("streaming asr connect failed: %w", err)
	}
	streamCtx, cancel := context.WithCancel(ctx)
	session := &cascadeASRSession{
		ctx:    streamCtx,
		cancel: cancel,
		conn:   conn,
		events: make(chan cascadeASREvent, 32),
		errors: make(chan error, 4),
		done:   make(chan struct{}),
	}
	if err := session.sendConfig(); err != nil {
		_ = conn.Close()
		cancel()
		return nil, err
	}
	session.lastPacketAt.Store(time.Now().UnixNano())
	go session.readLoop()
	go session.keepaliveLoop()
	return session, nil
}

func cascadeASRHeaders(cfg cascadeSpeechConfig) (http.Header, error) {
	headers := http.Header{}
	apiKey := strings.TrimSpace(cfg.ApiKey)
	accessKey := strings.TrimSpace(cfg.AccessKey)
	appKey := strings.TrimSpace(cfg.AppID)
	if apiKey != "" {
		headers.Set("X-Api-Key", apiKey)
	} else if accessKey != "" && appKey != "" {
		headers.Set("X-Api-App-Key", appKey)
		headers.Set("X-Api-Access-Key", accessKey)
	} else {
		return nil, fmt.Errorf("streaming asr credentials are not configured")
	}
	resourceID := strings.TrimSpace(cfg.ResourceID)
	if resourceID == "" || strings.HasPrefix(resourceID, "volc.seedasr.auc") {
		resourceID = defaultCascadeASRResourceID
	}
	headers.Set("X-Api-Resource-Id", resourceID)
	headers.Set("X-Api-Request-Id", uuid.NewString())
	headers.Set("X-Api-Connect-Id", uuid.NewString())
	headers.Set("X-Api-Sequence", "-1")
	return headers, nil
}

func (s *cascadeASRSession) sendConfig() error {
	payload := map[string]any{
		"user": map[string]any{"uid": uuid.NewString()},
		"audio": map[string]any{
			"format":  "pcm",
			"codec":   "raw",
			"rate":    16000,
			"bits":    16,
			"channel": 1,
		},
		"request": map[string]any{
			"model_name":           "bigmodel",
			"enable_nonstream":     true,
			"enable_itn":           true,
			"enable_punc":          true,
			"enable_ddc":           false,
			"show_utterances":      true,
			"result_type":          "full",
			"end_window_size":      800,
			"force_to_speech_time": 1000,
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	frame, err := scEncodeAudioPacket(scMsgTypeFullClient, 0, scSerializationJSON, scCompressionGZIP, data)
	if err != nil {
		return err
	}
	return s.write(frame)
}

func (s *cascadeASRSession) SendPCM(pcm []byte) error {
	if len(pcm) == 0 {
		return nil
	}
	s.bufferMu.Lock()
	s.buffer = append(s.buffer, pcm...)
	chunks := make([][]byte, 0, len(s.buffer)/cascadeASRChunkBytes)
	for len(s.buffer) >= cascadeASRChunkBytes {
		chunk := append([]byte(nil), s.buffer[:cascadeASRChunkBytes]...)
		s.buffer = s.buffer[cascadeASRChunkBytes:]
		chunks = append(chunks, chunk)
	}
	s.bufferMu.Unlock()
	for _, chunk := range chunks {
		if err := s.sendAudio(chunk, false); err != nil {
			return err
		}
	}
	return nil
}

func (s *cascadeASRSession) sendAudio(pcm []byte, last bool) error {
	flags := byte(0)
	if last {
		flags = scFlagLastNoSeq
	}
	frame, err := scEncodeAudioPacket(scMsgTypeAudioOnlyClient, flags, scSerializationRaw, scCompressionGZIP, pcm)
	if err != nil {
		return err
	}
	if err := s.write(frame); err != nil {
		return err
	}
	s.lastPacketAt.Store(time.Now().UnixNano())
	return nil
}

func (s *cascadeASRSession) keepaliveLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	silence := make([]byte, cascadeASRChunkBytes)
	for {
		select {
		case <-s.ctx.Done():
			return
		case now := <-ticker.C:
			last := time.Unix(0, s.lastPacketAt.Load())
			if now.Sub(last) >= 2*time.Second {
				_ = s.sendAudio(silence, false)
			}
		}
	}
}

func (s *cascadeASRSession) write(frame []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	default:
	}
	_ = s.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	err := s.conn.WriteMessage(websocket.BinaryMessage, frame)
	_ = s.conn.SetWriteDeadline(time.Time{})
	return err
}

func (s *cascadeASRSession) readLoop() {
	defer close(s.done)
	seenFinals := make(map[string]struct{})
	lastPartialText := ""
	for {
		messageType, data, err := s.conn.ReadMessage()
		if err != nil {
			if s.ctx.Err() == nil {
				s.pushError(fmt.Errorf("streaming asr read failed: %w", err))
			}
			return
		}
		if messageType != websocket.BinaryMessage {
			continue
		}
		frame, err := scDecodeFrame(data)
		if err != nil {
			s.pushError(fmt.Errorf("streaming asr invalid frame: %w", err))
			return
		}
		payload, payloadErr := frame.decodedPayload()
		if frame.MessageType == scMsgTypeError {
			message := strings.TrimSpace(string(payload))
			if message == "" {
				message = "provider error"
			}
			if frame.ErrorCode == 1013 {
				continue
			}
			s.pushError(fmt.Errorf("streaming asr error %d: %s", frame.ErrorCode, message))
			return
		}
		if payloadErr != nil || frame.MessageType != scMsgTypeFullServer || len(payload) == 0 {
			continue
		}
		var response struct {
			Result struct {
				Text       string `json:"text"`
				Utterances []struct {
					Text      string `json:"text"`
					Definite  bool   `json:"definite"`
					StartTime int64  `json:"start_time"`
					EndTime   int64  `json:"end_time"`
				} `json:"utterances"`
			} `json:"result"`
		}
		if json.Unmarshal(payload, &response) != nil {
			continue
		}
		lastPartial := -1
		for i, utterance := range response.Result.Utterances {
			text := strings.TrimSpace(utterance.Text)
			if text == "" {
				continue
			}
			if utterance.Definite {
				key := fmt.Sprintf("%d:%d", utterance.StartTime, utterance.EndTime)
				if utterance.StartTime == 0 && utterance.EndTime == 0 {
					key = text
				}
				if _, exists := seenFinals[key]; exists {
					continue
				}
				seenFinals[key] = struct{}{}
				s.pushEvent(cascadeASREvent{Text: text, Final: true, StartTime: utterance.StartTime, EndTime: utterance.EndTime, StablePrefix: text})
				lastPartialText = ""
				continue
			}
			lastPartial = i
		}
		if lastPartial >= 0 {
			utterance := response.Result.Utterances[lastPartial]
			text := strings.TrimSpace(utterance.Text)
			if text != "" {
				stable := cascadeCommonPrefix(lastPartialText, text)
				s.pushEvent(cascadeASREvent{Text: text, Final: false, StartTime: utterance.StartTime, EndTime: utterance.EndTime, StablePrefix: stable})
				lastPartialText = text
			}
		}
	}
}

func (s *cascadeASRSession) pushEvent(event cascadeASREvent) {
	select {
	case s.events <- event:
	case <-s.ctx.Done():
	}
}

func (s *cascadeASRSession) pushError(err error) {
	select {
	case s.errors <- err:
	default:
	}
}

func (s *cascadeASRSession) Events() <-chan cascadeASREvent {
	return s.events
}

func (s *cascadeASRSession) Errors() <-chan error {
	return s.errors
}

func (s *cascadeASRSession) Close() {
	s.closeOnce.Do(func() {
		s.bufferMu.Lock()
		remaining := append([]byte(nil), s.buffer...)
		s.buffer = nil
		s.bufferMu.Unlock()
		if len(remaining) > 0 {
			_ = s.sendAudio(remaining, false)
		}
		_ = s.sendAudio(nil, true)
		s.cancel()
		_ = s.conn.Close()
		select {
		case <-s.done:
		case <-time.After(300 * time.Millisecond):
		}
	})
}

func cascadeCommonPrefix(a, b string) string {
	ar, br := []rune(strings.TrimSpace(a)), []rune(strings.TrimSpace(b))
	n := len(ar)
	if len(br) < n {
		n = len(br)
	}
	i := 0
	for i < n && ar[i] == br[i] {
		i++
	}
	return string(ar[:i])
}
