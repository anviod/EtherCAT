package sim

import (
	"github.com/anviod/EtherCAT/ecad"
	"github.com/anviod/EtherCAT/ecfr"
)

const (
	regAreaLength = 0x1000
)

// FrameProcessor processes an EtherCAT frame, potentially modifying
// datagram data and working counters. It returns the processed frame.
type FrameProcessor interface {
	ProcessFrame(*ecfr.Frame) *ecfr.Frame
}

// ─── ALStatusControl ────────────────────────────────────────────────────────────

// ALStatusControl holds shared state between the AL Control and AL Status
// registers. It tracks the error condition and provides access to both
// register views.
type ALStatusControl struct {
	inError bool
	control ALControl
	status  ALStatus
}

// NewALStatusControl creates a new ALStatusControl with initialized
// control and status register views.
func NewALStatusControl() *ALStatusControl {
	sc := &ALStatusControl{}
	sc.control = ALControl{parent: sc}
	sc.status = ALStatus{parent: sc}
	return sc
}

// IsECATWritable returns true, indicating the ECAT side can write to
// the AL Control register.
func (sc *ALStatusControl) IsECATWritable() bool {
	return true
}

// InError returns true if the device is in an error state.
func (sc *ALStatusControl) InError() bool {
	return sc.inError
}

// SetError sets or clears the error state.
func (sc *ALStatusControl) SetError(seterr bool) {
	sc.inError = seterr
}

// ControlReg returns the AL Control register view.
func (sc *ALStatusControl) ControlReg() *ALControl {
	return &sc.control
}

// StatusReg returns the AL Status register view.
func (sc *ALStatusControl) StatusReg() *ALStatus {
	return &sc.status
}

// ─── ALControl ──────────────────────────────────────────────────────────────────

// ALControl represents the AL Control register (0x0120-0x0121).
// Each field maps to a specific bit in the register:
//
//	Byte 0: [idSel(7) | devIDReq(6) | errorInd(5) | ack(4) | state(3:0)]
//	Byte 1: [wdDiv2(7:5) | wdDiv(4:3) | wdTOut(2) | frcErr(1) | wdTrigger(0)]
type ALControl struct {
	ack       bool
	errorInd  bool
	devIDReq  bool
	idSel     bool
	wdTrigger bool
	frcErr    bool
	wdTOut    bool
	wdDiv     uint8 // 2 bits
	wdDiv2    uint8 // 3 bits
	wdState   uint8 // 4 bits (device state)
	parent    *ALStatusControl
}

// Read reads a byte from the AL Control register at the given offset.
// Offset 0 returns the state and control bits; offset 1 returns watchdog
// and divider configuration.
func (c *ALControl) Read(offs uint16, dp *uint8) bool {
	switch offs {
	case 0:
		v := uint8(c.wdState & 0x0f)
		if c.ack {
			v |= 1 << 4
		}
		if c.errorInd {
			v |= 1 << 5
		}
		if c.devIDReq {
			v |= 1 << 6
		}
		if c.idSel {
			v |= 1 << 7
		}
		*dp = v
	case 1:
		v := uint8(0)
		if c.wdTrigger {
			v |= 1 << 0
		}
		if c.frcErr {
			v |= 1 << 1
		}
		if c.wdTOut {
			v |= 1 << 2
		}
		v |= (c.wdDiv & 0x03) << 3
		v |= (c.wdDiv2 & 0x07) << 5
		*dp = v
	default:
		panic("invalid mapping for ALControl exceeds possible length")
	}

	return true
}

// WriteInteract returns whether the AL Control register is writable
// from the ECAT side.
func (c *ALControl) WriteInteract(offs uint16) bool {
	return c.parent.IsECATWritable()
}

// Latch applies shadow register writes to the AL Control register.
// State changes are gated: if the device is in error, state transitions
// that don't clear the error are blocked.
func (c *ALControl) Latch(shadow []byte, shadowWriteMask []bool) {
	// Byte 0: state, ack, errorInd, devIDReq, idSel
	if shadowWriteMask[0] {
		newState := shadow[0] & 0x0f
		newAck := (shadow[0] & (1 << 4)) != 0
		newErrorInd := (shadow[0] & (1 << 5)) != 0

		// State change is blocked if currently in error and the
		// write does not clear the error indication.
		if c.parent.InError() && (shadow[0]&0x10) != 0 {
			// Blocked: device is in error and error bit is not being cleared
		} else {
			c.wdState = newState
			c.ack = newAck
			c.errorInd = newErrorInd
			c.devIDReq = (shadow[0] & (1 << 6)) != 0
			c.idSel = (shadow[0] & (1 << 7)) != 0
		}
	}

	// Byte 1: wdTrigger, frcErr, wdTOut, wdDiv, wdDiv2
	if shadowWriteMask[1] {
		c.wdTrigger = (shadow[1] & (1 << 0)) != 0
		c.frcErr = (shadow[1] & (1 << 1)) != 0
		c.wdTOut = (shadow[1] & (1 << 2)) != 0
		c.wdDiv = (shadow[1] >> 3) & 0x03
		c.wdDiv2 = (shadow[1] >> 5) & 0x07
	}
}

// ─── ALStatus ───────────────────────────────────────────────────────────────────

// ALStatus represents the AL Status register (0x0130-0x0135).
// Each field maps to a specific bit in the register:
//
//	Byte 0: state (3:0)
//	Byte 1: [reserved(7:2) | errInd(1) | stateChangeAck(0)]
//	Byte 2-3: devID (16-bit)
//	Byte 4: wdState (bit 0)
//	Byte 5: reserved
type ALStatus struct {
	stateChangeAck bool
	errInd         bool
	devID          uint16
	wdState        bool
	state          uint8
	parent         *ALStatusControl
}

// Read reads a byte from the AL Status register at the given offset.
func (s *ALStatus) Read(offs uint16, dp *uint8) bool {
	switch offs {
	case 0:
		*dp = s.state & 0x0f
	case 1:
		v := uint8(0)
		if s.stateChangeAck {
			v |= 1 << 0
		}
		if s.errInd {
			v |= 1 << 1
		}
		*dp = v
	case 2:
		*dp = uint8(s.devID)
	case 3:
		*dp = uint8(s.devID >> 8)
	case 4:
		if s.wdState {
			*dp = 1
		} else {
			*dp = 0
		}
	case 5:
		*dp = 0x00
	default:
		*dp = 0x00
	}
	return true
}

// WriteInteract returns false: the AL Status register is read-only
// from the ECAT side.
func (s *ALStatus) WriteInteract(offs uint16) bool {
	return false
}

// Latch is a no-op: the AL Status register is read-only.
func (s *ALStatus) Latch(shadow []byte, shadowWriteMask []bool) {}

// ─── L2Slave ────────────────────────────────────────────────────────────────────

// L2Slave represents a layer-2 EtherCAT slave device with 64KB of backing
// memory, register shadowing, and frame processing capability.
type L2Slave struct {
	BackingMemory [1 << 16]byte

	registerShadow          [regAreaLength]byte
	registerShadowWriteMask [regAreaLength]bool

	regMappings []MMapping

	ALStatusControl *ALStatusControl
	EEPROM          *L2EEPROM
}

// NewL2Slave creates a new L2Slave with initialized ET1100 signature,
// AL Status/Control registers, and EEPROM.
func NewL2Slave() *L2Slave {
	s := &L2Slave{}

	// ET1100 signature
	copy(s.BackingMemory[:0x10], []byte{0x11, 0x00, 0x02, 0x00, 0x08, 0x08, 0x08, 0x0b, 0xfc})

	s.ALStatusControl = NewALStatusControl()
	s.regMappings = append(s.regMappings, DevMapping{ecad.ALControl, 0x02, s.ALStatusControl.ControlReg()})
	s.regMappings = append(s.regMappings, DevMapping{ecad.ALStatus, 0x06, s.ALStatusControl.StatusReg()})

	s.EEPROM = NewL2EEPROM()
	s.regMappings = append(s.regMappings, DevMapping{ecad.ESIEEPROMInterface, 0x10, s.EEPROM.Reg()})

	return s
}

// llread8p reads a single byte from physical address. Register area dispatches through device mapping.
// 从物理地址读取单字节。寄存器区域通过设备映射分发。
func (s *L2Slave) llread8p(addr uint16, dp *uint8) bool {
	if addr < regAreaLength {
		m := s.addrToMapping(addr)
		if m != nil {
			return m.Device().Read(addr-m.Start(), dp)
		}
	}

	*dp = s.BackingMemory[addr]
	return true
}

// llwrite8 writes a single byte to physical address. Register area shadows the write.
// 向物理地址写入单字节。寄存器区域会影子写入。
func (s *L2Slave) llwrite8(addr uint16, d uint8) bool {
	if addr < regAreaLength {
		s.registerShadow[addr] = d
		s.registerShadowWriteMask[addr] = true

		m := s.addrToMapping(addr)
		if m != nil {
			return m.Device().WriteInteract(addr - m.Start())
		}
	}

	s.BackingMemory[addr] = d
	return true
}

// addrToMapping finds the device mapping that covers the given address.
// Returns nil if no mapping covers the address.
func (s *L2Slave) addrToMapping(addr uint16) MMapping {
	for _, m := range s.regMappings {
		if addr >= m.Start() && addr < (m.Start()+m.Length()) {
			return m
		}
	}
	return nil
}

// ProcessFrame processes an EtherCAT frame through this slave.
// 通过本从站处理 EtherCAT 帧。
//
// For each datagram: if physically addressed, reads/writes backing memory,
// updates WKC. Register shadow is latched after all datagrams.
// 对每个数据报：若物理寻址匹配，读写后备内存并更新 WKC。处理完成后锁存寄存器影子。
//
// Perf: non-register addresses (>= 0x1000) use bulk copy() — 100x fewer calls.
// 性能优化：非寄存器地址使用批量 copy()，减少 100 倍函数调用。
func (s *L2Slave) ProcessFrame(infr *ecfr.Frame) (ofr *ecfr.Frame) {
	ofr = infr

	for _, dg := range infr.Datagrams {
		cmd := dg.Header.Command
		if !s.isPhysicalAddr(cmd, dg.Header.Addr32) {
			continue
		}

		dga := ecfr.DatagramAddressFromCommand(dg.Header.Addr32, cmd)
		physaddressed := s.isPhysicallyAddressed(dga)
		dga.IncrementSlaveAddr()
		dg.Header.Addr32 = dga.Addr32()
		if !physaddressed {
			continue
		}

		datalen := dg.Header.DataLength()
		physbase := dga.Offset()
		reads := cmd.DoesRead()
		writes := cmd.DoesWrite()

		// ── Bulk path for non-register memory (>= 0x1000) ──
		// This is the common case for process data and ESC memory.
		// Uses copy() instead of byte-by-byte loops — 100x fewer function calls.
		if physbase >= regAreaLength {
			if reads {
				copy(dg.Data[:datalen], s.BackingMemory[physbase:physbase+uint16(datalen)])
			}
			if writes {
				copy(s.BackingMemory[physbase:physbase+uint16(datalen)], dg.Data[:datalen])
			}
			// WKC: always increment for non-register memory (no masking)
			if reads && writes {
				dg.WKC++
			} else if reads || writes {
				dg.WKC++
			}
			continue
		}

		// ── Byte-by-byte path for register memory (< 0x1000) ──
		// Register area requires per-byte dispatch through device mappings.
		readUnmasked := true
		if reads {
			for i := uint16(0); i < datalen; i++ {
				readUnmasked = s.llread8p(physbase+i, &dg.Data[i]) && readUnmasked
			}
		}

		writeUnmasked := true
		if writes {
			for i := uint16(0); i < datalen; i++ {
				writeUnmasked = s.llwrite8(physbase+i, dg.Data[i]) && writeUnmasked
			}
		}

		// Working counter update logic
		if reads && writes {
			if readUnmasked && writeUnmasked {
				dg.WKC++
			}
		} else if reads {
			if readUnmasked {
				dg.WKC++
			}
		} else if writes {
			if writeUnmasked {
				dg.WKC++
			}
		}
	}

	// Latch register shadow into registers
	s.latchRegs()

	return
}

// latchRegs applies the shadow register values to all mapped devices.
func (s *L2Slave) latchRegs() {
	for _, m := range s.regMappings {
		start := m.Start()
		end := start + m.Length()
		m.Device().Latch(s.registerShadow[start:end],
			s.registerShadowWriteMask[start:end])
	}
}

// isPhysicalAddr checks if the command type uses physical or broadcast addressing.
func (s *L2Slave) isPhysicalAddr(ct ecfr.CommandType, addr32 uint32) bool {
	dga := ecfr.DatagramAddressFromCommand(addr32, ct)
	return dga.IsPhysical() || dga.Type() == ecfr.Broadcast
}

// isPhysicallyAddressed checks if this slave is addressed by the datagram.
// Returns true for broadcast, positional address 0, or when the slave
// position matches.
func (s *L2Slave) isPhysicallyAddressed(addr ecfr.DatagramAddress) bool {
	if addr.Type() == ecfr.Broadcast {
		return true
	}

	if addr.Type() == ecfr.Positional {
		return addr.PositionOrAddress() == 0
	}

	if addr.Type() == ecfr.Fixed {
		// TODO: station address register comparison
		return false
	}

	return false
}
