package business

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type realtimeContextKey struct{}

func (e *Engine) RealtimeAuthority(ctx context.Context, request Request) (context.Context, coordination.ExecutionScope, func(), error) {
	if request.ExpectedScope == nil || request.RoleID == "" || request.RoleID != request.ExpectedScope.RoleID {
		return ctx, coordination.ExecutionScope{}, func() {}, coordination.ErrScopeExpired
	}
	current, scope, finish, err := e.continuityAuthority(ctx, request)
	if err != nil {
		return ctx, scope, finish, err
	}
	if summaryAuthority(scope) != summaryAuthority(*request.ExpectedScope) {
		finish()
		return ctx, scope, func() {}, coordination.ErrScopeExpired
	}
	if err := coordination.ValidateCurrent(current); err != nil {
		finish()
		return ctx, scope, func() {}, err
	}
	return current, scope, finish, nil
}

func RealtimePCM(pcm []byte) (Attachment, error) {
	if len(pcm) < 2 || len(pcm)%2 != 0 || len(pcm) > (1<<20)-44 {
		return Attachment{}, errors.New("实时语音必须为 16 kHz 单声道 PCM16，单轮最多 32 秒")
	}
	wav := make([]byte, 44+len(pcm))
	copy(wav, "RIFF")
	binary.LittleEndian.PutUint32(wav[4:], uint32(len(wav)-8))
	copy(wav[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(wav[16:], 16)
	binary.LittleEndian.PutUint16(wav[20:], 1)
	binary.LittleEndian.PutUint16(wav[22:], 1)
	binary.LittleEndian.PutUint32(wav[24:], 16000)
	binary.LittleEndian.PutUint32(wav[28:], 32000)
	binary.LittleEndian.PutUint16(wav[32:], 2)
	binary.LittleEndian.PutUint16(wav[34:], 16)
	copy(wav[36:], "data")
	binary.LittleEndian.PutUint32(wav[40:], uint32(len(pcm)))
	copy(wav[44:], pcm)
	digest := sha256.Sum256(wav)
	return Attachment{Kind: "audio", Name: "realtime.wav", MIME: "audio/wav", Data: base64.StdEncoding.EncodeToString(wav), Hash: hex.EncodeToString(digest[:])}, nil
}

func (e *Engine) RealtimeTurn(ctx context.Context, request Request, pcm []byte, visual *Attachment, emit func(Event) error) (Response, error) {
	if _, err := uuid.Parse(request.RequestID); err != nil {
		return Response{}, errors.New("实时对话每轮必须使用独立 UUID")
	}
	current, _, finish, err := e.RealtimeAuthority(ctx, request)
	if err != nil {
		return Response{}, err
	}
	defer finish()
	audio, err := RealtimePCM(pcm)
	if err != nil {
		return Response{}, err
	}
	request.Attachments = []Attachment{audio}
	if visual != nil {
		if visual.Kind != "image" {
			return Response{}, errors.New("实时视觉只能携带经校验的图片帧")
		}
		request.Attachments = append(request.Attachments, *visual)
	}
	request.Message = "[实时语音]"
	return e.RunEvents(context.WithValue(current, realtimeContextKey{}, true), request, emit)
}
