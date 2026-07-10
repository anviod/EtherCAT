package sim

// L2EEPROM simulates an EtherCAT EEPROM with 8192 16-bit words.
// It provides register-level access through the L2EEPROMRegisterSet.
type L2EEPROM struct {
	Array [8 * 1024]uint16

	Addr        uint32
	DataScratch [8]byte // data in wire encoding

	PDIControl         bool
	WriteEnable        bool
	ChecksumError      bool
	EENotLoaded        bool
	MissingAcknowledge bool
	ErrorWriteEnable   bool
	Busy               bool
}

// NewL2EEPROM creates a new L2EEPROM instance with pre-filled test data.
// Each word is initialized to 0xEE00 + index.
func NewL2EEPROM() *L2EEPROM {
	ee := &L2EEPROM{}

	for i := 0; i < len(ee.Array); i++ {
		ee.Array[i] = 0xee00 + uint16(i)
	}

	return ee
}

// Reg returns the register set interface for accessing EEPROM registers
// in the 0x0500-0x050F address range.
func (ee *L2EEPROM) Reg() *L2EEPROMRegisterSet {
	return &L2EEPROMRegisterSet{ee}
}

// L2EEPROMRegisterSet implements MMDevice for the EEPROM register area
// (addresses 0x0500-0x050F).
type L2EEPROMRegisterSet struct {
	*L2EEPROM
}

// Read reads a byte from the EEPROM register area at the given offset.
// The register layout is:
//
//	0x00: PDI Control (bit 0 = PDI access)
//	0x01: PDI Access State (reserved)
//	0x02: EEPROM Control/Status (bit 0 = write enable, bits 6-7 = address/size config)
//	0x03: EEPROM Command/Status (bits 0-1 = command, bits 3-7 = status flags)
//	0x04-0x07: EEPROM Address (32-bit, little-endian)
//	0x08-0x0F: EEPROM Data (64-bit scratch buffer)
func (ee *L2EEPROMRegisterSet) Read(offs uint16, dp *uint8) bool {
	switch offs {
	case 0:
		if ee.PDIControl {
			*dp = 0x01
		} else {
			*dp = 0x00
		}
	case 1:
		*dp = 0x00
	case 2:
		if ee.WriteEnable {
			*dp |= 0x01
		}
		*dp |= 0xc0 // 2 address bytes, support 8 bytes of data
	case 3:
		// lower 3 bits are command, upper 5 bits are status flags
		if ee.ChecksumError {
			*dp |= 1 << (11 - 8)
		}
		if ee.EENotLoaded {
			*dp |= 1 << (12 - 8)
		}
		if ee.MissingAcknowledge {
			*dp |= 1 << (13 - 8)
		}
		if ee.ErrorWriteEnable {
			*dp |= 1 << (14 - 8)
		}
		if ee.Busy {
			*dp |= 1 << (15 - 8)
		}
	case 4:
		*dp = uint8(ee.Addr)
	case 5:
		*dp = uint8(ee.Addr >> 8)
	case 6:
		*dp = uint8(ee.Addr >> 16)
	case 7:
		*dp = uint8(ee.Addr >> 24)
	default:
		if offs > 16 {
			panic("invalid use of ee reg area, read past end")
		}
		if offs >= 8 && offs < 16 {
			*dp = ee.DataScratch[offs-8]
		}
	}

	return true
}

// WriteInteract handles write interaction for the EEPROM register area.
// Register 0x02 (Control) and 0x03 (Command) are writeable only when not busy.
func (ee *L2EEPROMRegisterSet) WriteInteract(offs uint16) bool {
	if offs == 2 || offs == 3 {
		return !ee.Busy
	}
	return true
}

// Latch applies shadow register writes to the EEPROM state.
//
// FIXED: The address bytes (offsets 4-7) are now properly reconstructed using
// correct uint32 type conversions. The original code had a bug where
// uint32(shadow[7]) << 32 would overflow to 0 on a 32-bit type.
// The fix uses uint32(shadow[7]) | uint32(shadow[6])<<8 |
// uint32(shadow[5])<<16 | uint32(shadow[4])<<24 to construct the full
// 32-bit address with proper type promotion before each shift.
func (ee *L2EEPROMRegisterSet) Latch(shadow []byte, shadowWriteMask []bool) {
	for offs := 0; offs < len(shadow); offs++ {
		switch {
		case offs == 0:
			if shadowWriteMask[0] {
				if shadow[0]&0x01 != 0 {
					ee.PDIControl = true
				} else {
					ee.PDIControl = false
				}
			}
		case offs == 1:
			// PDI access state - not simulated
		case offs == 2:
			if shadowWriteMask[2] {
				if shadow[2]&0x01 != 0 {
					ee.WriteEnable = true
				} else {
					ee.WriteEnable = false
				}
			}
		case offs == 3:
			if shadowWriteMask[3] {
				switch shadow[3] & 0x03 {
				case 0x00:
					ee.ChecksumError = false
					ee.EENotLoaded = false
					ee.MissingAcknowledge = false
					ee.ErrorWriteEnable = false
				case 0x01:
					// Read command: clear busy and read into scratch
					ee.Busy = false
					ee.readIntoScratch()
				default:
					// write/reload not supported
				}
			}
		case offs == 4, offs == 5, offs == 6, offs == 7:
			// Address bytes are handled collectively below
		case offs >= 8 && offs < 16:
			if shadowWriteMask[offs] {
				ee.DataScratch[offs-8] = shadow[offs]
			}
		}
	}

	// Reconstruct address from shadow bytes if any address byte was written.
	// FIXED: Using proper uint32 type conversions to avoid shift overflow.
	// The original code had uint32(shadow[7]) << 32 which evaluates to 0
	// because shifting a uint32 by 32 bits is undefined (modulo bit width).
	// The correct expression uses <<24 for the top byte with proper type casting.
	if shadowWriteMask[4] || shadowWriteMask[5] || shadowWriteMask[6] || shadowWriteMask[7] {
		ee.Addr = uint32(shadow[7]) | uint32(shadow[6])<<8 | uint32(shadow[5])<<16 | uint32(shadow[4])<<24
	}
}

// readIntoScratch reads 4 words (8 bytes) from the EEPROM array at the current
// address into the DataScratch buffer in little-endian wire encoding.
func (ee *L2EEPROM) readIntoScratch() {
	for i := 0; i < 4; i++ {
		w16 := ee.Array[(int(ee.Addr)+i)%len(ee.Array)]
		ee.DataScratch[i*2] = uint8(w16)
		ee.DataScratch[i*2+1] = uint8(w16 >> 8)
	}
}
