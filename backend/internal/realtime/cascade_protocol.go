// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"

	"github.com/gorilla/websocket"
)

const (
	scMsgTypeFullClient      byte = 0x10
	scMsgTypeAudioOnlyClient byte = 0x20
	scMsgTypeFullServer      byte = 0x90
	scMsgTypeAudioOnlyServer byte = 0xB0
	scMsgTypeError           byte = 0xF0

	scFlagPositiveSeq byte = 0x01
	scFlagLastNoSeq   byte = 0x02
	scFlagNegativeSeq byte = 0x03
	scFlagWithEvent   byte = 0x04

	scSerializationRaw  byte = 0x00
	scSerializationJSON byte = 0x10

	scCompressionNone byte = 0x00
	scCompressionGZIP byte = 0x01
)

const (
	scEventStartConnection    int32 = 1
	scEventFinishConnection   int32 = 2
	scEventConnectionStarted  int32 = 50
	scEventConnectionFailed   int32 = 51
	scEventConnectionFinished int32 = 52

	scEventStartSession    int32 = 100
	scEventCancelSession   int32 = 101
	scEventFinishSession   int32 = 102
	scEventSessionStarted  int32 = 150
	scEventSessionCanceled int32 = 151
	scEventSessionFinished int32 = 152
	scEventSessionFailed   int32 = 153

	scEventTaskRequest  int32 = 200
	scEventUpdateConfig int32 = 201

	scEventTTSSentenceStart int32 = 350
	scEventTTSSentenceEnd   int32 = 351
	scEventTTSResponse      int32 = 352
	scEventTTSEnded         int32 = 359

	scEventASRInfo     int32 = 450
	scEventASRResponse int32 = 451
	scEventASREnded    int32 = 459
)

type scFrame struct {
	MessageType   byte
	Flags         byte
	Serialization byte
	Compression   byte
	Event         int32
	HasEvent      bool
	SessionID     string
	ConnectID     string
	ErrorCode     uint32
	Payload       []byte
}

func scIsConnectionEvent(event int32) bool {
	switch event {
	case scEventStartConnection,
		scEventFinishConnection,
		scEventConnectionStarted,
		scEventConnectionFailed,
		scEventConnectionFinished:
		return true
	default:
		return false
	}
}

func scGzip(payload []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(payload); err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func scUngzip(payload []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

func scEncodeEvent(messageType, serialization, compression byte, event int32, sessionID string, payload []byte) ([]byte, error) {
	if !scIsConnectionEvent(event) && sessionID == "" {
		return nil, fmt.Errorf("speech session id is required for event %d", event)
	}
	encodedPayload := payload
	var err error
	if compression == scCompressionGZIP {
		encodedPayload, err = scGzip(payload)
		if err != nil {
			return nil, fmt.Errorf("gzip speech payload: %w", err)
		}
	}

	buf := bytes.NewBuffer(make([]byte, 0, 16+len(sessionID)+len(encodedPayload)))
	buf.Write([]byte{0x11, messageType | scFlagWithEvent, serialization | compression, 0x00})
	if err := binary.Write(buf, binary.BigEndian, event); err != nil {
		return nil, err
	}
	if !scIsConnectionEvent(event) {
		if err := binary.Write(buf, binary.BigEndian, uint32(len(sessionID))); err != nil {
			return nil, err
		}
		buf.WriteString(sessionID)
	}
	if err := binary.Write(buf, binary.BigEndian, uint32(len(encodedPayload))); err != nil {
		return nil, err
	}
	buf.Write(encodedPayload)
	return buf.Bytes(), nil
}

func scEncodeJSONEvent(event int32, sessionID string, payload any) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal speech event %d: %w", event, err)
	}
	return scEncodeEvent(
		scMsgTypeFullClient,
		scSerializationJSON,
		scCompressionNone,
		event,
		sessionID,
		data,
	)
}

func scEncodeAudioPacket(messageType, flags, serialization, compression byte, payload []byte) ([]byte, error) {
	encoded := payload
	if compression == scCompressionGZIP {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		if _, err := zw.Write(payload); err != nil {
			_ = zw.Close()
			return nil, err
		}
		if err := zw.Close(); err != nil {
			return nil, err
		}
		encoded = buf.Bytes()
	}
	buf := bytes.NewBuffer(make([]byte, 0, 8+len(encoded)))
	buf.Write([]byte{0x11, messageType | flags, serialization | compression, 0x00})
	if err := binary.Write(buf, binary.BigEndian, uint32(len(encoded))); err != nil {
		return nil, err
	}
	buf.Write(encoded)
	return buf.Bytes(), nil
}

func scDecodeFrame(frame []byte) (*scFrame, error) {
	if len(frame) < 4 {
		return nil, fmt.Errorf("speech frame too short: %d", len(frame))
	}
	headerSize := int(frame[0]&0x0F) * 4
	if headerSize < 4 || headerSize > len(frame) {
		return nil, fmt.Errorf("invalid speech header size: %d", headerSize)
	}

	out := &scFrame{
		MessageType:   frame[1] & 0xF0,
		Flags:         frame[1] & 0x0F,
		Serialization: frame[2] & 0xF0,
		Compression:   frame[2] & 0x0F,
	}
	offset := headerSize

	readUint32 := func() (uint32, error) {
		if offset+4 > len(frame) {
			return 0, io.ErrUnexpectedEOF
		}
		value := binary.BigEndian.Uint32(frame[offset : offset+4])
		offset += 4
		return value, nil
	}
	readSizedString := func(label string) (string, error) {
		length, err := readUint32()
		if err != nil {
			return "", fmt.Errorf("decode speech %s length: %w", label, err)
		}
		if uint64(offset)+uint64(length) > uint64(len(frame)) {
			return "", io.ErrUnexpectedEOF
		}
		value := string(frame[offset : offset+int(length)])
		offset += int(length)
		return value, nil
	}

	sequenceFlag := out.Flags & 0x03
	if sequenceFlag == scFlagPositiveSeq || sequenceFlag == scFlagNegativeSeq {
		if _, err := readUint32(); err != nil {
			return nil, fmt.Errorf("decode speech sequence: %w", err)
		}
	}

	if out.MessageType == scMsgTypeError {
		code, err := readUint32()
		if err != nil {
			return nil, fmt.Errorf("decode speech error code: %w", err)
		}
		out.ErrorCode = code
		payloadLen, err := readUint32()
		if err != nil {
			return nil, fmt.Errorf("decode speech error payload length: %w", err)
		}
		if uint64(offset)+uint64(payloadLen) > uint64(len(frame)) {
			return nil, io.ErrUnexpectedEOF
		}
		out.Payload = append([]byte(nil), frame[offset:offset+int(payloadLen)]...)
		return out, nil
	}

	if out.Flags&scFlagWithEvent != 0 {
		eventRaw, err := readUint32()
		if err != nil {
			return nil, fmt.Errorf("decode speech event: %w", err)
		}
		out.Event = int32(eventRaw)
		out.HasEvent = true

		if !scIsConnectionEvent(out.Event) {
			out.SessionID, err = readSizedString("session id")
			if err != nil {
				return nil, err
			}
		} else if out.Event == scEventConnectionStarted || out.Event == scEventConnectionFailed || out.Event == scEventConnectionFinished {
			if offset+4 > len(frame) {
				return nil, io.ErrUnexpectedEOF
			}
			firstLen := int(binary.BigEndian.Uint32(frame[offset : offset+4]))
			remainingAfterFirstLen := len(frame) - (offset + 4)
			if remainingAfterFirstLen != firstLen && firstLen >= 0 && remainingAfterFirstLen >= firstLen+4 {
				payloadLenOffset := offset + 4 + firstLen
				candidatePayloadLen := int(binary.BigEndian.Uint32(frame[payloadLenOffset : payloadLenOffset+4]))
				if len(frame)-(payloadLenOffset+4) == candidatePayloadLen {
					out.ConnectID, err = readSizedString("connect id")
					if err != nil {
						return nil, err
					}
				}
			}
		}
	}

	payloadLen, err := readUint32()
	if err != nil {
		return nil, fmt.Errorf("decode speech payload length: %w", err)
	}
	if uint64(offset)+uint64(payloadLen) > uint64(len(frame)) {
		return nil, io.ErrUnexpectedEOF
	}
	out.Payload = append([]byte(nil), frame[offset:offset+int(payloadLen)]...)
	return out, nil
}

func (f *scFrame) decodedPayload() ([]byte, error) {
	if f == nil || len(f.Payload) == 0 {
		return nil, nil
	}
	if f.Compression != scCompressionGZIP {
		return f.Payload, nil
	}
	data, err := scUngzip(f.Payload)
	if err != nil {
		return nil, fmt.Errorf("ungzip speech payload: %w", err)
	}
	return data, nil
}

func scReadExpectedProviderEvent(conn *websocket.Conn, expected int32, label string) ([]byte, error) {
	for {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", label, err)
		}
		if messageType != websocket.BinaryMessage {
			continue
		}
		frame, err := scDecodeFrame(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", label, err)
		}
		payload, payloadErr := frame.decodedPayload()
		if frame.MessageType == scMsgTypeError {
			message := string(payload)
			if payloadErr != nil || message == "" {
				message = "provider error"
			}
			return nil, fmt.Errorf("%s failed %d: %s", label, frame.ErrorCode, message)
		}
		if !frame.HasEvent {
			continue
		}
		if frame.Event == expected {
			return payload, nil
		}
		if frame.Event == scEventConnectionFailed || frame.Event == scEventSessionFailed {
			return nil, fmt.Errorf("%s rejected: %s", label, string(payload))
		}
	}
}
