package ecad

// ─── ESC Information (0x0000-0x000F) ────────────────────────────────────────────

const (
	Type                 = 0x0000 // ESC type identifier
	Revision             = 0x0001 // ESC revision number
	Build                = 0x0002 // ESC build number
	FMMUsSupported       = 0x0004 // Number of supported FMMUs
	RAMSize              = 0x0006 // RAM size in KB
	PortDescriptor       = 0x0007 // Physical port descriptor
	ESCFeaturesSupported = 0x0008 // ESC feature flags
)

// ─── Station Address (0x0010-0x001F) ───────────────────────────────────────────

const (
	ConfiguredStationAddress = 0x0010 // Configured station address (16-bit)
	ConfiguredStationAlias   = 0x0012 // Configured station alias (16-bit)
)

// ─── ESC Reset (0x0040-0x004F) ─────────────────────────────────────────────────

const ESCResetECAT = 0x0040 // ESC reset register for ECAT unit

// ─── Data Link Layer (0x0100-0x011F) ───────────────────────────────────────────

const (
	DLControl = 0x0100 // Data Link Control register
	DLStatus  = 0x0110 // Data Link Status register
)

// ─── Application Layer (0x0120-0x013F) ─────────────────────────────────────────

const (
	ALControl    = 0x0120 // Application Layer Control register
	ALStatus     = 0x0130 // Application Layer Status register
	ALStatusCode = 0x0134 // Application Layer Status Code register
)

// ─── PDI Control (0x0140-0x014F) ───────────────────────────────────────────────

const PDIControl = 0x0140 // Process Data Interface Control register

// ─── Interrupt / Event (0x0200-0x02FF) ─────────────────────────────────────────

const ECATEventMask = 0x0200 // ECAT event mask register

// ─── EEPROM Interface (0x0500-0x050F) ──────────────────────────────────────────

const (
	ESIEEPROMInterface   = 0x0500 // EEPROM Interface (alias for EEPROMConfiguration)
	EEPROMConfiguration  = 0x0500 // EEPROM Configuration register
	EEPROMPDIAccessState = 0x0501 // EEPROM PDI Access State register
	EEPROMControlStatus  = 0x0502 // EEPROM Control/Status register
	EEPROMAddress        = 0x0504 // EEPROM Address register (32-bit)
	EEPROMData           = 0x0508 // EEPROM Data register (64-bit)
)

// ─── FMMU (0x0600-0x07FF) ──────────────────────────────────────────────────────

const FMMUBase = 0x0600 // Base address for FMMU configuration

// FMMU channel stride is 16 bytes per channel.

// ─── Sync Manager (0x0800-0x0FFF) ──────────────────────────────────────────────

const (
	SyncMangerBase = 0x0800 // Base address for Sync Manager configuration
)

// Sync Manager channel layout (each channel is 8 bytes):
const (
	SyncManagerChannelLen          = 0x08 // Channel stride
	SyncManagerPhysStartAddrOffset = 0x00 // Physical start address offset
	SyncManagerLengthOffset        = 0x02 // Length offset
	SyncManagerControlOffset       = 0x04 // Control register offset
	SyncManagerStatusOffset        = 0x05 // Status register offset
	SyncManagerActivateOffset      = 0x06 // Activate register offset
	SyncManagerPDIControlOffset    = 0x07 // PDI Control offset
)
