package marshalling

// ---------------------------------------------------------------------------
// Little-endian readers (EtherCAT wire format)
// ---------------------------------------------------------------------------

// GetUint8 reads one byte from b and returns the value together with the
// remaining slice.
func GetUint8(b []byte) (uint8, []byte) {
	return b[0], b[1:]
}

// GetUint16 reads two bytes (little-endian) from b and returns the value
// together with the remaining slice.
func GetUint16(b []byte) (uint16, []byte) {
	return uint16(b[0]) | uint16(b[1])<<8, b[2:]
}

// GetUint32 reads four bytes (little-endian) from b and returns the value
// together with the remaining slice.
func GetUint32(b []byte) (uint32, []byte) {
	v := uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
	return v, b[4:]
}

// ---------------------------------------------------------------------------
// Little-endian writers (EtherCAT wire format)
// ---------------------------------------------------------------------------

// PutUint8 writes v into b and returns the remaining slice.
func PutUint8(b []byte, v uint8) []byte {
	b[0] = v
	return b[1:]
}

// PutUint16 writes v (little-endian) into b and returns the remaining slice.
func PutUint16(b []byte, v uint16) []byte {
	b[0] = uint8(v)
	b[1] = uint8(v >> 8)
	return b[2:]
}

// PutUint32 writes v (little-endian) into b and returns the remaining slice.
func PutUint32(b []byte, v uint32) []byte {
	b[0] = uint8(v)
	b[1] = uint8(v >> 8)
	b[2] = uint8(v >> 16)
	b[3] = uint8(v >> 24)
	return b[4:]
}

// ---------------------------------------------------------------------------
// Little-endian extractors (no slice advancement)
// ---------------------------------------------------------------------------

// XGetUint8 returns the first byte of b without advancing the slice.
func XGetUint8(b []byte) uint8 {
	return b[0]
}

// XGetUint16 returns the first two bytes of b as a little-endian uint16
// without advancing the slice.
func XGetUint16(b []byte) uint16 {
	return uint16(b[0]) | uint16(b[1])<<8
}

// XGetUint32 returns the first four bytes of b as a little-endian uint32
// without advancing the slice.
func XGetUint32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

// ---------------------------------------------------------------------------
// Big-endian readers (Ethernet wire format)
// ---------------------------------------------------------------------------

// GetUint8BE reads one byte from b and returns the value together with the
// remaining slice.
func GetUint8BE(b []byte) (uint8, []byte) {
	return b[0], b[1:]
}

// GetUint16BE reads two bytes (big-endian) from b and returns the value
// together with the remaining slice.
func GetUint16BE(b []byte) (uint16, []byte) {
	return uint16(b[0])<<8 | uint16(b[1]), b[2:]
}

// GetUint32BE reads four bytes (big-endian) from b and returns the value
// together with the remaining slice.
func GetUint32BE(b []byte) (uint32, []byte) {
	v := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	return v, b[4:]
}

// ---------------------------------------------------------------------------
// Big-endian writers (Ethernet wire format)
// ---------------------------------------------------------------------------

// PutUint8BE writes v into b and returns the remaining slice.
func PutUint8BE(b []byte, v uint8) []byte {
	b[0] = v
	return b[1:]
}

// PutUint16BE writes v (big-endian) into b and returns the remaining slice.
func PutUint16BE(b []byte, v uint16) []byte {
	b[0] = uint8(v >> 8)
	b[1] = uint8(v)
	return b[2:]
}

// PutUint32BE writes v (big-endian) into b and returns the remaining slice.
func PutUint32BE(b []byte, v uint32) []byte {
	b[0] = uint8(v >> 24)
	b[1] = uint8(v >> 16)
	b[2] = uint8(v >> 8)
	b[3] = uint8(v)
	return b[4:]
}
