// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"encoding/binary"
)

const (
	MsgTypeFullClient  = 0x01
	MsgTypeAudioOnly   = 0x02
	MsgTypeFullServer  = 0x09
	MsgTypeAudioServer = 0x0B

	FlagHasEvent        = 0x04
	FlagSeqNonTerminal  = 0x01
	FlagSeqLastNumbered = 0x02
	FlagSeqLastNegative = 0x03

	SerMethodRaw  = 0x00
	SerMethodJSON = 0x01

	CompNone = 0x00
	CompGzip = 0x01

	EvtStartConnection  = 1
	EvtFinishConnection = 2
	EvtStartSession     = 100
	EvtTaskRequest      = 200
	EvtUpdateConfig     = 201
)

func buildHeader(msgType, flags, serMethod, compMethod byte) [4]byte {
	return [4]byte{
		(1 << 4) | 1,
		(msgType << 4) | (flags & 0x0F),
		(serMethod << 4) | (compMethod & 0x0F),
		0x00,
	}
}

func buildEventFrame(msgType byte, eventCode int32, sessionID string, payload []byte) []byte {
	hdr := buildHeader(msgType, FlagHasEvent, SerMethodJSON, CompNone)
	buf := make([]byte, 0, 256)
	buf = append(buf, hdr[:]...)

	eb := make([]byte, 4)
	binary.BigEndian.PutUint32(eb, uint32(eventCode))
	buf = append(buf, eb...)

	isConn := eventCode == 1 || eventCode == 2
	if !isConn {
		sid := []byte(sessionID)
		lb := make([]byte, 4)
		binary.BigEndian.PutUint32(lb, uint32(len(sid)))
		buf = append(buf, lb...)
		if len(sid) > 0 {
			buf = append(buf, sid...)
		}
	}

	plen := make([]byte, 4)
	if payload == nil {
		payload = []byte{}
	}
	binary.BigEndian.PutUint32(plen, uint32(len(payload)))
	buf = append(buf, plen...)
	buf = append(buf, payload...)

	return buf
}

func buildAudioFrame(sessionID string, pcmData []byte) []byte {
	hdr := buildHeader(MsgTypeAudioOnly, FlagHasEvent, SerMethodRaw, CompNone)
	buf := make([]byte, 0, 256+len(pcmData))
	buf = append(buf, hdr[:]...)

	eb := make([]byte, 4)
	binary.BigEndian.PutUint32(eb, uint32(EvtTaskRequest))
	buf = append(buf, eb...)

	sid := []byte(sessionID)
	lb := make([]byte, 4)
	binary.BigEndian.PutUint32(lb, uint32(len(sid)))
	buf = append(buf, lb...)
	if len(sid) > 0 {
		buf = append(buf, sid...)
	}

	plen := make([]byte, 4)
	binary.BigEndian.PutUint32(plen, uint32(len(pcmData)))
	buf = append(buf, plen...)
	buf = append(buf, pcmData...)

	return buf
}

type FrameHeader struct {
	MsgType    byte
	Flags      byte
	SerMethod  byte
	CompMethod byte
}

func parseHeader(hdr [4]byte) FrameHeader {
	return FrameHeader{
		MsgType:    hdr[1] >> 4,
		Flags:      hdr[1] & 0x0F,
		SerMethod:  hdr[2] >> 4,
		CompMethod: hdr[2] & 0x0F,
	}
}

type ParsedFrame struct {
	Hdr       FrameHeader
	EventCode int32
	SessionID string
	Payload   []byte
}

func parseFrame(data []byte) (*ParsedFrame, error) {
	if len(data) < 4 {
		return nil, nil
	}
	var hdrBytes [4]byte
	copy(hdrBytes[:], data[:4])
	hdr := parseHeader(hdrBytes)

	pos := 4
	f := &ParsedFrame{Hdr: hdr}

	if hdr.Flags&FlagHasEvent != 0 {
		if pos+4 > len(data) {
			return f, nil
		}
		f.EventCode = int32(binary.BigEndian.Uint32(data[pos:]))
		pos += 4
	}

	isConn := f.EventCode == 1 || f.EventCode == 2 || f.EventCode == 50 || f.EventCode == 51 || f.EventCode == 52
	if !isConn {
		if pos+4 > len(data) {
			return f, nil
		}
		sidLen := binary.BigEndian.Uint32(data[pos:])
		pos += 4
		if sidLen > 0 && sidLen < 1024 && pos+int(sidLen) <= len(data) {
			f.SessionID = string(data[pos : pos+int(sidLen)])
			pos += int(sidLen)
		}
	}

	if pos+4 > len(data) {
		return f, nil
	}
	payloadLen := binary.BigEndian.Uint32(data[pos:])
	pos += 4

	if payloadLen > 0 && pos+int(payloadLen) <= len(data) {
		f.Payload = make([]byte, payloadLen)
		copy(f.Payload, data[pos:pos+int(payloadLen)])
	}

	return f, nil
}
