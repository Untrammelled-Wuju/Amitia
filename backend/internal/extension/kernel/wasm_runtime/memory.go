package wasm_runtime

import (
	"encoding/binary"
	"fmt"
	"unicode/utf8"
)

type MemoryAccess struct {
	memSize   uint64
	maxRead   int64
	maxWrite  int64
	alloced   map[uint32]uint32
	dealloced map[uint32]bool
}

func NewMemoryAccess(memSize uint64, maxRead, maxWrite int64) *MemoryAccess {
	return &MemoryAccess{
		memSize:   memSize,
		maxRead:   maxRead,
		maxWrite:  maxWrite,
		alloced:   make(map[uint32]uint32),
		dealloced: make(map[uint32]bool),
	}
}

func (m *MemoryAccess) ValidateRead(ptr uint32, length uint32) error {
	if length == 0 {
		return nil
	}
	if int64(length) > m.maxRead {
		return NewWASMError(ErrCodeMemoryLimit, fmt.Sprintf("read exceeds max: %d > %d", length, m.maxRead), nil)
	}
	end, ok := safeAddU32(ptr, length)
	if !ok {
		return NewWASMError(ErrCodeMemoryAccess, fmt.Sprintf("ptr+%d overflow: ptr=%d", length, ptr), nil)
	}
	if uint64(end) > m.memSize {
		return NewWASMError(ErrCodeMemoryAccess, fmt.Sprintf("read out of bounds: [%d, %d) > %d", ptr, end, m.memSize), nil)
	}
	return nil
}

func (m *MemoryAccess) ValidateWrite(ptr uint32, length uint32) error {
	if length == 0 {
		return nil
	}
	if int64(length) > m.maxWrite {
		return NewWASMError(ErrCodeMemoryLimit, fmt.Sprintf("write exceeds max: %d > %d", length, m.maxWrite), nil)
	}
	end, ok := safeAddU32(ptr, length)
	if !ok {
		return NewWASMError(ErrCodeMemoryAccess, fmt.Sprintf("ptr+%d overflow: ptr=%d", length, ptr), nil)
	}
	if uint64(end) > m.memSize {
		return NewWASMError(ErrCodeMemoryAccess, fmt.Sprintf("write out of bounds: [%d, %d) > %d", ptr, end, m.memSize), nil)
	}
	return nil
}

func (m *MemoryAccess) TrackAlloc(ptr uint32, size uint32) {
	m.alloced[ptr] = size
	delete(m.dealloced, ptr)
}

func (m *MemoryAccess) TrackDealloc(ptr uint32) error {
	if _, ok := m.alloced[ptr]; !ok {
		return NewWASMError(ErrCodeMemoryAccess, fmt.Sprintf("dealloc unallocated ptr: %d", ptr), nil)
	}
	if m.dealloced[ptr] {
		return NewWASMError(ErrCodeMemoryAccess, fmt.Sprintf("double free: ptr=%d", ptr), nil)
	}
	m.dealloced[ptr] = true
	return nil
}

func (m *MemoryAccess) UpdateMemSize(size uint64) {
	m.memSize = size
}

func ValidateUTF8(data []byte) error {
	if !utf8.Valid(data) {
		return NewWASMError(ErrCodeOutputInvalid, "output is not valid UTF-8", nil)
	}
	return nil
}

func safeAddU32(a, b uint32) (uint32, bool) {
	sum := uint64(a) + uint64(b)
	if sum > 0xFFFFFFFF {
		return 0, false
	}
	return uint32(sum), true
}

func ReadMemBytes(mem []byte, ptr uint32, length uint32) ([]byte, error) {
	end, ok := safeAddU32(ptr, length)
	if !ok {
		return nil, NewWASMError(ErrCodeMemoryAccess, "memory read overflow", nil)
	}
	if uint64(end) > uint64(len(mem)) {
		return nil, NewWASMError(ErrCodeMemoryAccess, "memory read out of bounds", nil)
	}
	return mem[ptr:end], nil
}

func WriteMemBytes(mem []byte, ptr uint32, data []byte) error {
	end := uint32(ptr) + uint32(len(data))
	if uint64(end) > uint64(len(mem)) {
		return NewWASMError(ErrCodeMemoryAccess, "memory write out of bounds", nil)
	}
	copy(mem[ptr:], data)
	return nil
}

func ReadU32FromMem(mem []byte, ptr uint32) (uint32, error) {
	if int(ptr)+4 > len(mem) {
		return 0, NewWASMError(ErrCodeMemoryAccess, "memory read u32 out of bounds", nil)
	}
	return binary.LittleEndian.Uint32(mem[ptr:]), nil
}

func WriteU32ToMem(mem []byte, ptr uint32, val uint32) error {
	if int(ptr)+4 > len(mem) {
		return NewWASMError(ErrCodeMemoryAccess, "memory write u32 out of bounds", nil)
	}
	binary.LittleEndian.PutUint32(mem[ptr:], val)
	return nil
}
