package sdk

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

type IDGenerator interface {
	NewID() string
}

type UUIDGenerator struct{}

func (g UUIDGenerator) NewID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		// IDGenerator cannot return an error. Preserve uniqueness and protocol
		// validity without importing an external UUID package if OS entropy is
		// temporarily unavailable.
		return (&TimestampGenerator{}).NewID()
	}
	raw[6] = (raw[6] & 0x0f) | 0x40 // RFC 4122 version 4
	raw[8] = (raw[8] & 0x3f) | 0x80 // RFC 4122 variant
	var out [36]byte
	hex.Encode(out[0:8], raw[0:4])
	out[8] = '-'
	hex.Encode(out[9:13], raw[4:6])
	out[13] = '-'
	hex.Encode(out[14:18], raw[6:8])
	out[18] = '-'
	hex.Encode(out[19:23], raw[8:10])
	out[23] = '-'
	hex.Encode(out[24:36], raw[10:16])
	return string(out[:])
}

type TimestampGenerator struct {
	counter uint64
}

func (g *TimestampGenerator) NewID() string {
	count := atomic.AddUint64(&g.counter, 1)
	return fmt.Sprintf("msg-%d-%d", time.Now().UnixNano(), count)
}
