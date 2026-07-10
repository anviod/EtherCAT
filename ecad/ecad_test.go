package ecad

import (
	"testing"
)

func TestRegisterAddresses(t *testing.T) {
	// Verify all register addresses are within the valid ESC register space (0x0000-0x0FFF)
	registers := map[string]uint16{
		"Type":                    Type,
		"Revision":                Revision,
		"Build":                   Build,
		"FMMUsSupported":          FMMUsSupported,
		"RAMSize":                 RAMSize,
		"PortDescriptor":          PortDescriptor,
		"ESCFeaturesSupported":    ESCFeaturesSupported,
		"ConfiguredStationAddress": ConfiguredStationAddress,
		"ConfiguredStationAlias":  ConfiguredStationAlias,
		"ESCResetECAT":            ESCResetECAT,
		"DLControl":               DLControl,
		"DLStatus":                DLStatus,
		"ALControl":               ALControl,
		"ALStatus":                ALStatus,
		"ALStatusCode":            ALStatusCode,
		"PDIControl":              PDIControl,
		"ECATEventMask":           ECATEventMask,
		"EEPROMConfiguration":     EEPROMConfiguration,
		"EEPROMPDIAccessState":    EEPROMPDIAccessState,
		"EEPROMControlStatus":     EEPROMControlStatus,
		"EEPROMAddress":           EEPROMAddress,
		"EEPROMData":              EEPROMData,
		"FMMUBase":                FMMUBase,
		"SyncMangerBase":          SyncMangerBase,
	}

	for name, addr := range registers {
		t.Run(name, func(t *testing.T) {
			if addr > 0x0FFF {
				t.Errorf("register %s address 0x%04X out of valid range (0x0000-0x0FFF)", name, addr)
			}
		})
	}
}

func TestSynonymAliases(t *testing.T) {
	// Verify that ESIEEPROMInterface and EEPROMConfiguration are the same
	if ESIEEPROMInterface != EEPROMConfiguration {
		t.Error("ESIEEPROMInterface should be identical to EEPROMConfiguration")
	}
}

func TestSyncManagerOffsets(t *testing.T) {
	// Verify Sync Manager offsets are within a single channel (0x00-0x07)
	offsets := map[string]uint16{
		"PhysStartAddr": SyncManagerPhysStartAddrOffset,
		"Length":        SyncManagerLengthOffset,
		"Control":       SyncManagerControlOffset,
		"Status":        SyncManagerStatusOffset,
		"Activate":      SyncManagerActivateOffset,
		"PDIControl":    SyncManagerPDIControlOffset,
	}

	for name, off := range offsets {
		t.Run(name, func(t *testing.T) {
			if off >= SyncManagerChannelLen {
				t.Errorf("SyncManager offset %d exceeds channel length %d", off, SyncManagerChannelLen)
			}
		})
	}
}

func TestChannelLenConsistency(t *testing.T) {
	// SyncManagerChannelLen must be 8 bytes per the EtherCAT specification
	if SyncManagerChannelLen != 8 {
		t.Errorf("SyncManagerChannelLen should be 8, got %d", SyncManagerChannelLen)
	}
}

func TestAddressOverlapDetection(t *testing.T) {
	// Verify no register address overlaps between groups
	type region struct {
		name  string
		start uint16
		end   uint16
	}

	regions := []region{
		{"ESC Info", Type, ESCFeaturesSupported},
		{"Station Addr", ConfiguredStationAddress, ConfiguredStationAlias},
		{"DL", DLControl, DLStatus},
		{"AL", ALControl, ALStatusCode},
		{"PDI", PDIControl, PDIControl},
		{"EEPROM", EEPROMConfiguration, EEPROMData},
		{"FMMU", FMMUBase, FMMUBase},
		{"SyncM", SyncMangerBase, SyncMangerBase},
	}

	for i := 0; i < len(regions); i++ {
		for j := i + 1; j < len(regions); j++ {
			a, b := regions[i], regions[j]
			if a.start <= b.end && b.start <= a.end {
				t.Errorf("potential overlap between %s [0x%04X-0x%04X] and %s [0x%04X-0x%04X]",
					a.name, a.start, a.end, b.name, b.start, b.end)
			}
		}
	}
}

// ─── Benchmarks ─────────────────────────────────────────────────────────────────

func BenchmarkRegisterAccess(b *testing.B) {
	var sum uint16
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sum += Type
		sum += ALControl
		sum += EEPROMData
	}
	_ = sum
}