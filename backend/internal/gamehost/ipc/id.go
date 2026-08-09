package ipc

import (
	"fmt"
	"sync/atomic"

	"github.com/google/uuid"
)

type uuidIDGenerator struct {
	counter uint64
}

func NewUUIDIDGenerator() IDGenerator {
	return &uuidIDGenerator{}
}

func (g *uuidIDGenerator) Generate() ConnectionID {
	n := atomic.AddUint64(&g.counter, 1)
	id := fmt.Sprintf("ipc-%s-%d", uuid.New().String(), n)
	return ConnectionID(id)
}
