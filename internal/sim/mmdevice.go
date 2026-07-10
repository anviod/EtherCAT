package sim

// MMDevice represents a memory-mapped device that can be read from, written to,
// and latched with shadow register values.
type MMDevice interface {
	Read(offs uint16, dp *uint8) bool
	WriteInteract(offs uint16) bool
	Latch(shadow []byte, shadowWriteMask []bool)
}

// MMapping represents a mapping of a memory region to a device.
type MMapping interface {
	Start() uint16
	Length() uint16
	Device() MMDevice
}

// DevMapping implements the MMapping interface, describing a contiguous
// region of register space that is mapped to a specific MMDevice.
type DevMapping struct {
	StartAddr   uint16
	LengthField uint16
	DeviceField MMDevice
}

// Start returns the starting address of the mapping.
func (d DevMapping) Start() uint16 { return d.StartAddr }

// Length returns the length of the mapping in bytes.
func (d DevMapping) Length() uint16 { return d.LengthField }

// Device returns the MMDevice that handles this mapped region.
func (d DevMapping) Device() MMDevice { return d.DeviceField }