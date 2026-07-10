package ecfr

import (
	"errors"
	"unsafe"
)

// Sentinel errors for hot-path use — avoids fmt.Errorf allocation overhead.
var (
	errHeaderBufferNil = errors.New("header buffer is nil")
	errHeaderTooShort  = errors.New("header buffer too short: need 2 bytes")
	errNotEnoughForHdr = errors.New("not enough bytes for header")
)

// ---------------------------------------------------------------------------
// Header — 2-byte EtherCAT frame header
// ---------------------------------------------------------------------------

// Header represents the 2-byte EtherCAT frame header.
// Bit layout:
//
//	bits 0-10:  frame length (11 bits)
//	bits 12-15: frame type (4 bits)
type Header struct {
	Word   uint16
	buffer []byte
}

// maxFrameLength is the maximum value for the 11-bit length field.
const maxFrameLength = (1 << 11) - 1

// Overlay reads the 2-byte header from b.
// 从 b 读取 2 字节帧头。
// Perf: single-cycle uint16 read via unsafe.Pointer, zero allocs.
func (h *Header) Overlay(b []byte) ([]byte, error) {
	if len(b) < 2 {
		return nil, errNotEnoughForHdr
	}
	h.buffer = b
	h.Word = *(*uint16)(unsafe.Pointer(&b[0]))
	return b[2:], nil
}

// Commit writes the 2-byte header back to the buffer.
// 将 2 字节帧头写回缓冲区。
// Perf: single-cycle uint16 write via unsafe.Pointer.
func (h *Header) Commit() ([]byte, error) {
	if h.buffer == nil {
		return nil, errHeaderBufferNil
	}
	if len(h.buffer) < 2 {
		return nil, errHeaderTooShort
	}
	*(*uint16)(unsafe.Pointer(&h.buffer[0])) = h.Word
	return h.buffer[:2], nil
}

// FrameLength returns the frame length (lower 11 bits of Word).
func (h *Header) FrameLength() uint16 {
	return h.Word & maxFrameLength
}

// Type returns the frame type (upper 4 bits of Word).
func (h *Header) Type() uint8 {
	return uint8(h.Word >> 12)
}

// SetType sets the frame type (upper 4 bits of Word).
func (h *Header) SetType(t uint8) {
	h.Word = (h.Word & maxFrameLength) | (uint16(t&0x0f) << 12)
}
