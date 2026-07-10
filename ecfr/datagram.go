package ecfr

import (
	"fmt"
	"unsafe"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

// DatagramOverheadLength is the overhead bytes per datagram: 10 bytes header + 2 bytes WKC.
const DatagramOverheadLength = 12

// datagramHeaderLength is the fixed size of a datagram header on the wire.
const datagramHeaderLength = 10

// maxDatagramDataLen is the maximum data length for a single datagram (11 bits).
const maxDatagramDataLen = (1 << 11) - 1

// wkcOffset is the offset of the Working Counter within the datagram buffer
// (10 bytes header + data length).
const wkcOffset = 10

// ---------------------------------------------------------------------------
// CommandType
// ---------------------------------------------------------------------------

// CommandType is a 4-bit EtherCAT command type.
type CommandType uint8

// EtherCAT command types.
const (
	NOP  CommandType = 0
	APRD CommandType = 1
	APWR CommandType = 2
	APRW CommandType = 3
	FPRD CommandType = 4
	FPWR CommandType = 5
	FPRW CommandType = 6
	BRD  CommandType = 7
	BWR  CommandType = 8
	BRW  CommandType = 9
	LRD  CommandType = 10
	LWR  CommandType = 11
	LRW  CommandType = 12
	ARMW CommandType = 13
	FRMW CommandType = 14
)

var commandNames = [...]string{
	"NOP", "APRD", "APWR", "APRW", "FPRD", "FPWR", "FPRW",
	"BRD", "BWR", "BRW", "LRD", "LWR", "LRW", "ARMW", "FRMW",
}

// String returns the human-readable name of the command.
func (ct CommandType) String() string {
	if int(ct) < len(commandNames) {
		return commandNames[ct]
	}
	return fmt.Sprintf("CommandType(%d)", ct)
}

// DoesRead returns true if this command performs a read operation.
func (ct CommandType) DoesRead() bool {
	switch ct {
	case NOP, BWR, LWR, APWR, FPWR:
		return false
	default:
		return true
	}
}

// DoesWrite returns true if this command performs a write operation.
func (ct CommandType) DoesWrite() bool {
	switch ct {
	case NOP, APRD, FPRD, BRD, LRD:
		return false
	default:
		return true
	}
}

// ---------------------------------------------------------------------------
// DatagramAddressType
// ---------------------------------------------------------------------------

// DatagramAddressType classifies the addressing mode of a datagram.
type DatagramAddressType uint

const (
	UninitializedDatagramAddressType DatagramAddressType = iota
	Positional
	Fixed
	Broadcast
	Logical
)

// ---------------------------------------------------------------------------
// DatagramAddress
// ---------------------------------------------------------------------------

// DatagramAddress represents a 32-bit EtherCAT address with an associated
// addressing mode.
type DatagramAddress struct {
	addr uint32
	typ  DatagramAddressType
}

// String returns a human-readable representation of the address.
func (da DatagramAddress) String() string {
	switch da.typ {
	case Positional:
		return fmt.Sprintf("Positional(addr=%d,offs=0x%04x)", int16(da.addr&0xFFFF), uint16(da.addr>>16))
	case Fixed:
		return fmt.Sprintf("Fixed(station=0x%04x,offs=0x%04x)", uint16(da.addr&0xFFFF), uint16(da.addr>>16))
	case Broadcast:
		return fmt.Sprintf("Broadcast(addr=0x%08x)", da.addr)
	case Logical:
		return fmt.Sprintf("Logical(addr=0x%08x)", da.addr)
	default:
		return fmt.Sprintf("Uninitialized(addr=0x%08x)", da.addr)
	}
}

// Addr32 returns the raw 32-bit address value.
func (da DatagramAddress) Addr32() uint32 { return da.addr }

// Type returns the addressing mode.
func (da DatagramAddress) Type() DatagramAddressType { return da.typ }

// Offset returns the upper 16 bits of the address (register offset).
func (da DatagramAddress) Offset() uint16 { return uint16(da.addr >> 16) }

// SetOffset sets the upper 16 bits of the address.
func (da *DatagramAddress) SetOffset(offs uint16) {
	da.addr = (da.addr & 0x0000FFFF) | (uint32(offs) << 16)
}

// PositionOrAddress returns the lower 16 bits (position or station address).
func (da DatagramAddress) PositionOrAddress() uint16 { return uint16(da.addr & 0xFFFF) }

// IncrementSlaveAddr increments the lower 16 bits (slave position or address).
func (da *DatagramAddress) IncrementSlaveAddr() {
	da.addr = (da.addr & 0xFFFF0000) | ((da.addr + 1) & 0x0000FFFF)
}

// IsPhysical returns true for positional or fixed addressing.
func (da DatagramAddress) IsPhysical() bool {
	return da.typ == Positional || da.typ == Fixed
}

// DatagramAddressFromCommand constructs a DatagramAddress from a raw 32-bit
// address and a command type, inferring the address type.
func DatagramAddressFromCommand(addr32 uint32, ct CommandType) DatagramAddress {
	var typ DatagramAddressType
	switch ct {
	case APRD, APWR, APRW, ARMW:
		typ = Positional
	case FPRD, FPWR, FPRW, FRMW:
		typ = Fixed
	case BRD, BWR, BRW:
		typ = Broadcast
	case LRD, LWR, LRW:
		typ = Logical
	default:
		typ = UninitializedDatagramAddressType
	}
	return DatagramAddress{addr: addr32, typ: typ}
}

// PositionalAddr creates a position-based address (auto-increment).
func PositionalAddr(position int16, offset uint16) DatagramAddress {
	return DatagramAddress{
		addr: (uint32(offset) << 16) | uint32(uint16(position)),
		typ:  Positional,
	}
}

// FixedAddr creates a fixed station address.
func FixedAddr(stationaddr uint16, offset uint16) DatagramAddress {
	return DatagramAddress{
		addr: (uint32(offset) << 16) | uint32(stationaddr),
		typ:  Fixed,
	}
}

// ---------------------------------------------------------------------------
// DatagramHeader — 10-byte wire format
// ---------------------------------------------------------------------------
// Wire layout (all little-endian):
//   Offset 0: Command   (uint8)
//   Offset 1: Index     (uint8)
//   Offset 2: Addr32    (uint32 LE)
//   Offset 6: LenWord   (uint16 LE)  bits 0-10:data, bit14:roundtrip, bit15:!last
//   Offset 8: Interrupt (uint16 LE)
//
// NOTE: We use *[10]byte (not a struct) to avoid alignment padding issues.
// A [10]byte array has no padding — byte 0 is always byte 0 of the wire.

// DatagramHeader represents the 10-byte EtherCAT datagram header.
type DatagramHeader struct {
	Command   CommandType
	Index     uint8
	Addr32    uint32
	LenWord   uint16
	Interrupt uint16

	buffer []byte
}

// Overlay reads the 10-byte header from d.
// 从 d 读取 10 字节头部。
//
// Perf: single uint64 read (bytes 0-7) + uint16 read (bytes 8-9) via unsafe.Pointer.
// 2 memory ops instead of 10 byte accesses.
// 性能优化：单次 uint64 读取 + uint16 读取，2 次内存操作替代 10 次字节访问。
func (dh *DatagramHeader) Overlay(d []byte) ([]byte, error) {
	if len(d) < datagramHeaderLength {
		return nil, fmt.Errorf("need %d bytes for datagram header, have %d", datagramHeaderLength, len(d))
	}
	lo := *(*uint64)(unsafe.Pointer(&d[0])) // bytes 0-7: Cmd, Idx, Addr32, LenWord
	hi := *(*uint16)(unsafe.Pointer(&d[8])) // bytes 8-9: Interrupt
	dh.Command = CommandType(byte(lo))
	dh.Index = byte(lo >> 8)
	dh.Addr32 = uint32(lo >> 16)
	dh.LenWord = uint16(lo >> 48)
	dh.Interrupt = hi
	dh.buffer = d
	return d[datagramHeaderLength:], nil
}

// Commit writes the 10-byte header back to the buffer.
// 将 10 字节头部写回缓冲区。
//
// Perf: single uint64 write (bytes 0-7) + uint16 write (bytes 8-9).
// 性能优化：单次 uint64 写入 + uint16 写入。
func (dh *DatagramHeader) Commit() ([]byte, error) {
	if dh.buffer == nil {
		return nil, fmt.Errorf("datagram header buffer is nil")
	}
	if len(dh.buffer) < datagramHeaderLength {
		return nil, fmt.Errorf("datagram header buffer too short: need %d bytes, have %d", datagramHeaderLength, len(dh.buffer))
	}
	lo := uint64(dh.Command) | uint64(dh.Index)<<8 | uint64(dh.Addr32)<<16 | uint64(dh.LenWord)<<48
	*(*uint64)(unsafe.Pointer(&dh.buffer[0])) = lo
	*(*uint16)(unsafe.Pointer(&dh.buffer[8])) = uint16(dh.Interrupt)
	return dh.buffer[:datagramHeaderLength], nil
}

// SlaveAddr returns the lower 16 bits of Addr32 (position or station address).
func (dh *DatagramHeader) SlaveAddr() uint16 { return uint16(dh.Addr32 & 0xFFFF) }

// OffsetAddr returns the upper 16 bits of Addr32 (register offset).
func (dh *DatagramHeader) OffsetAddr() uint16 { return uint16(dh.Addr32 >> 16) }

// LogicalAddr returns the full Addr32 as a logical address.
func (dh *DatagramHeader) LogicalAddr() uint32 { return dh.Addr32 }

// DataLength returns the lower 11 bits of LenWord.
func (dh *DatagramHeader) DataLength() uint16 { return dh.LenWord & maxDatagramDataLen }

// Roundtrip returns true if bit 14 is set (roundtrip flag).
func (dh *DatagramHeader) Roundtrip() bool { return (dh.LenWord >> 14 & 1) == 1 }

// Last returns true if bit 15 is 0 (last datagram indicator, active low).
func (dh *DatagramHeader) Last() bool { return (dh.LenWord >> 15 & 1) == 0 }

// SetLast sets the "last" indicator (bit 15 = 0 for last, 1 for not last).
func (dh *DatagramHeader) SetLast(last bool) {
	if last {
		dh.LenWord &^= 1 << 15
	} else {
		dh.LenWord |= 1 << 15
	}
}

// ---------------------------------------------------------------------------
// Datagram
// ---------------------------------------------------------------------------

// Datagram represents a complete EtherCAT datagram: header + data + WKC.
type Datagram struct {
	Header DatagramHeader
	Data   []byte
	WKC    uint16

	buffer []byte
}

// Overlay reads a complete datagram (header + data + WKC) from d.
// 从 d 读取完整数据报（头部 + 数据 + WKC）。
// Perf: WKC read via unsafe.Pointer for zero-overhead access.
func (dg *Datagram) Overlay(d []byte) ([]byte, error) {
	rem, err := dg.Header.Overlay(d)
	if err != nil {
		return nil, err
	}

	datalen := int(dg.Header.DataLength())
	needTotal := datalen + 2 // data + WKC
	if len(rem) < needTotal {
		return nil, fmt.Errorf("need %d bytes for data+WKC, have %d", needTotal, len(rem))
	}
	dg.Data = rem[:datalen]

	// WKC: read directly via unsafe.Pointer for zero-overhead
	dg.WKC = *(*uint16)(unsafe.Pointer(&rem[datalen]))
	dg.buffer = d
	return rem[needTotal:], nil
}

// Commit writes the datagram (header + WKC) back to the buffer.
// 将数据报（头部 + WKC）写回缓冲区。
// Perf: WKC write via unsafe.Pointer. Error from Header.Commit() is checked.
func (dg *Datagram) Commit() ([]byte, error) {
	if dg.buffer == nil {
		return nil, fmt.Errorf("datagram buffer is nil")
	}

	// Commit header
	_, err := dg.Header.Commit()
	if err != nil {
		return nil, fmt.Errorf("header commit: %w", err)
	}

	datalen := int(dg.Header.DataLength())
	totalLen := datagramHeaderLength + datalen + 2
	if len(dg.buffer) < totalLen {
		return nil, fmt.Errorf("datagram buffer too short: need %d bytes, have %d", totalLen, len(dg.buffer))
	}

	// WKC: write directly via unsafe.Pointer for zero-overhead
	*(*uint16)(unsafe.Pointer(&dg.buffer[wkcOffset+datalen])) = dg.WKC
	return dg.buffer[:totalLen], nil
}

// ByteLen returns the total length of the datagram on the wire.
func (dg *Datagram) ByteLen() int {
	return int(dg.Header.DataLength()) + DatagramOverheadLength
}

// Summary returns a compact human-readable summary of the datagram.
func (dg *Datagram) Summary() string {
	return fmt.Sprintf("%v %s len=%d wkc=%d",
		dg.Header.Command,
		DatagramAddressFromCommand(dg.Header.Addr32, dg.Header.Command),
		dg.Header.DataLength(),
		dg.WKC,
	)
}

// SetDataLen adjusts the data length of the datagram and re-slices Data.
func (dg *Datagram) SetDataLen(ndl int) error {
	if ndl < 0 || ndl > maxDatagramDataLen {
		return fmt.Errorf("data length %d exceeds maximum %d", ndl, maxDatagramDataLen)
	}
	// Set length in the LenWord field, preserving the upper bits
	dg.Header.LenWord = (dg.Header.LenWord & ^uint16(maxDatagramDataLen)) | uint16(ndl)
	// Re-slice data from the buffer
	if dg.buffer != nil {
		dataStart := datagramHeaderLength
		dataEnd := dataStart + ndl
		if dataEnd+2 > len(dg.buffer) {
			return fmt.Errorf("buffer too small for data length %d", ndl)
		}
		dg.Data = dg.buffer[dataStart:dataEnd]
	}
	return nil
}

// PointDatagramTo overlays a Datagram onto the given byte slice.
func PointDatagramTo(d []byte) (Datagram, error) {
	var dg Datagram
	_, err := dg.Overlay(d)
	return dg, err
}

// PointDatagramHeaderTo overlays a DatagramHeader onto the given byte slice.
func PointDatagramHeaderTo(d []byte) (DatagramHeader, error) {
	var dh DatagramHeader
	_, err := dh.Overlay(d)
	return dh, err
}