package sdk

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

type IDGenerator interface {
	NewID() string
}

type UUIDGenerator struct{}

func (g UUIDGenerator) NewID() string {
	return uuid.New().string()
}

type TimestampGenerator struct {
	counter uint64
}

func (g *TimestampGenerator) NewID() string {
	count := atomic.AddUint64(&g.counter, 1)
	return fmt.Sprintf("msg-%d-%d", time.Now().UnixNano(), count)
}
