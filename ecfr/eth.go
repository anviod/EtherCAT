package ecfr

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// ---------------------------------------------------------------------------
// ETHAddr
// ---------------------------------------------------------------------------

// ETHAddr is a 6-byte Ethernet MAC address.
type ETHAddr [6]byte

// String returns the MAC address in colon-separated hex notation.
func (ea ETHAddr) String() string {
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		ea[0], ea[1], ea[2], ea[3], ea[4], ea[5])
}

// sliceToETHAddr copies 6 bytes from s into an ETHAddr. Caller must ensure
// len(s) >= 6.
func sliceToETHAddr(s []byte) ETHAddr {
	var e ETHAddr
	copy(e[:], s[:6])
	return e
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	minFramelenWithFCS = 64
	fcsLen             = 4
	minHeaderAndPayload = minFramelenWithFCS - fcsLen

	// including FCS
	maxFramelenNoVLAN = 1522
	maxFramelenVLAN   = 1526
	maxFramelen       = maxFramelenVLAN

	offsetDestination = 0
	offsetSource      = 6
	offsetVLANOrType  = 12
	offsetVLANTCI     = 14

	etherTypeVLAN = 0x8100
)

// ---------------------------------------------------------------------------
// ETHFrame
// ---------------------------------------------------------------------------

// ETHFrame represents an Ethernet frame encapsulating EtherCAT data.
type ETHFrame struct {
	Destination ETHAddr
	Source      ETHAddr
	Type        uint16
	UseVlan     bool
	VLANTCI     uint16

	framebuf []byte
}

// OverlayETHFrame creates an ETHFrame pointing into fb (allocates if empty).
// 创建指向 fb 的 ETHFrame（fb 为空时自动分配）。
// Perf: Type/VLAN read via binary.BigEndian (network byte order).
func OverlayETHFrame(fb []byte) (*ETHFrame, error) {
	if len(fb) == 0 {
		fb = make([]byte, maxFramelen)
	}

	if len(fb) < minFramelenWithFCS {
		return nil, fmt.Errorf(
			"NewETHFrame: buffer too small, need at least %d bytes",
			minHeaderAndPayload)
	}

	ef := &ETHFrame{}
	ef.framebuf = fb

	ef.Destination = sliceToETHAddr(fb[offsetDestination:offsetSource])
	ef.Source = sliceToETHAddr(fb[offsetSource:offsetVLANOrType])

	// Ethernet Type is big-endian (network byte order)
	ef.Type = binary.BigEndian.Uint16(fb[offsetVLANOrType : offsetVLANOrType+2])

	if ef.Type == etherTypeVLAN {
		ef.UseVlan = true
		ef.VLANTCI = binary.BigEndian.Uint16(fb[offsetVLANTCI : offsetVLANTCI+2])
	}

	return ef, nil
}

// GetHeaderLen returns the length of the Ethernet header (dest + src + type,
// plus optional VLAN tag).
func (ef *ETHFrame) GetHeaderLen() int {
	vlanlen := 0
	if ef.UseVlan {
		vlanlen = 4
	}
	return 6 + 6 + 2 + vlanlen
}

// GetFooterLen returns the length of the Ethernet footer (FCS = 4 bytes).
func (ef *ETHFrame) GetFooterLen() int {
	return fcsLen
}

// GetFrameBuf returns the underlying frame buffer.
func (ef *ETHFrame) GetFrameBuf() []byte {
	return ef.framebuf
}

// GetPayload returns the payload slice (between the header and the FCS).
func (ef *ETHFrame) GetPayload() []byte {
	return ef.framebuf[ef.GetHeaderLen() : len(ef.framebuf)-ef.GetFooterLen()]
}

// SetPayloadLen adjusts the total frame length to accommodate a payload of npl
// bytes.
func (ef *ETHFrame) SetPayloadLen(npl int) error {
	nl := npl + ef.GetHeaderLen() + ef.GetFooterLen()
	if nl < minFramelenWithFCS {
		return fmt.Errorf(
			"SetPayloadLen: payload too small, need at least %d bytes",
			minFramelenWithFCS-ef.GetHeaderLen()-ef.GetFooterLen())
	}

	maxnl := maxFramelenNoVLAN
	if ef.UseVlan {
		maxnl = maxFramelenVLAN
	}

	if nl > maxnl {
		return fmt.Errorf(
			"SetPayloadLen: payload too big, maximum for this configuration is %d bytes",
			maxnl-ef.GetHeaderLen())
	}

	if nl > cap(ef.framebuf) {
		return fmt.Errorf(
			"SetPayloadLen: payload of %d bytes too big for buffer, buffer can hold a %d bytes maximum",
			npl,
			cap(ef.framebuf)-ef.GetFooterLen()-ef.GetHeaderLen())
	}

	ef.framebuf = ef.framebuf[:nl]
	return nil
}

// WriteDown serializes Ethernet header fields into the frame buffer.
// 将以太网头部字段序列化到帧缓冲区。
// Perf: Type field written via binary.BigEndian (network byte order).
func (ef *ETHFrame) WriteDown() error {
	copy(ef.framebuf[0:6], ef.Destination[:])
	copy(ef.framebuf[6:12], ef.Source[:])

	if ef.UseVlan {
		return errors.New("VLAN tags are not supported")
	}

	binary.BigEndian.PutUint16(ef.framebuf[offsetVLANOrType:], ef.Type)
	return nil
}