package ecee

import (
	"errors"
	"time"

	"github.com/anviod/EtherCAT/ecad"
	"github.com/anviod/EtherCAT/ecfr"
	"github.com/anviod/EtherCAT/ecmd"
)

// EEPROM defines the interface for accessing an EtherCAT slave's EEPROM.
type EEPROM interface {
	// ReadWord reads a 16-bit word from the EEPROM at the given address.
	ReadWord(addr uint32) (uint16, error)
	// WriteWord writes a 16-bit word to the EEPROM at the given address.
	WriteWord(addr uint32, word uint16) error
	// Close releases the EEPROM accessor.
	Close() error
}

// blindEEPROM implements EEPROM by communicating directly with the ESC
// EEPROM interface registers.
type blindEEPROM struct {
	comm   ecmd.Commander
	addr   ecfr.DatagramAddress
	closed bool
}

// New creates a new EEPROM accessor for the slave at the given datagram address.
func New(commander ecmd.Commander, addr ecfr.DatagramAddress) (EEPROM, error) {
	return &blindEEPROM{
		comm: commander,
		addr: addr,
	}, nil
}

// waitForIdle polls the EEPROM Control/Status register until the busy bit
// clears, the error bit is set, or the timeout expires.
//
// The original implementation had a critical bug: the timeout branch was
// empty, causing the function to loop forever when the busy bit never cleared.
// This version correctly uses time.After and select to enforce the timeout,
// and returns "EEPROM timeout" when the deadline is exceeded.
func (ee *blindEEPROM) waitForIdle(timeout time.Duration) error {
	if timeout == 0 {
		timeout = 250 * time.Millisecond
	}

	deadline := time.After(timeout)
	opts := ecmd.Options{FramelossTries: 3}

	for {
		select {
		case <-deadline:
			return errors.New("EEPROM timeout")
		default:
		}

		regAddr := ee.addr
		regAddr.SetOffset(ecad.EEPROMControlStatus)
		status, err := ecmd.ExecuteRead16Options(ee.comm, regAddr, 1, opts)
		if err != nil {
			return err
		}

		// bit 4 (0x0010) = busy, bit 1 (0x0002) = error
		if status&0x0010 == 0 && status&0x0002 == 0 {
			return nil
		}

		if status&0x0002 != 0 {
			return errors.New("EEPROM error")
		}
		// busy — continue polling
	}
}

// ReadWord reads a 16-bit word from the EEPROM at the given address.
func (ee *blindEEPROM) ReadWord(addr uint32) (uint16, error) {
	if ee.closed {
		return 0, errors.New("ecee eeprom is already closed")
	}

	err := ee.waitForIdle(250 * time.Millisecond)
	if err != nil {
		return 0, err
	}

	dgaddr := ee.addr

	// Write the EEPROM address to the ESC.
	dgaddr.SetOffset(ecad.EEPROMAddress)
	wb := []byte{uint8(addr), uint8(addr >> 8), uint8(addr >> 16), uint8(addr >> 24)}
	err = ecmd.ExecuteWrite(ee.comm, dgaddr, wb, 1)
	if err != nil {
		return 0, err
	}

	err = ee.waitForIdle(250 * time.Millisecond)
	if err != nil {
		return 0, err
	}

	// Read 8 bytes from the EEPROM Data register; the low 2 bytes are the result.
	dgaddr = ee.addr
	dgaddr.SetOffset(ecad.EEPROMData)
	rb, err := ecmd.ExecuteRead(ee.comm, dgaddr, 8, 1)
	if err != nil {
		return 0, err
	}

	word := uint16(rb[0]) | uint16(rb[1])<<8
	return word, nil
}

// WriteWord writes a 16-bit word to the EEPROM at the given address.
func (ee *blindEEPROM) WriteWord(addr uint32, word uint16) error {
	if ee.closed {
		return errors.New("ecee eeprom is already closed")
	}

	err := ee.waitForIdle(250 * time.Millisecond)
	if err != nil {
		return err
	}

	dgaddr := ee.addr

	// Write the EEPROM address to the ESC.
	dgaddr.SetOffset(ecad.EEPROMAddress)
	wb := []byte{uint8(addr), uint8(addr >> 8), uint8(addr >> 16), uint8(addr >> 24)}
	err = ecmd.ExecuteWrite(ee.comm, dgaddr, wb, 1)
	if err != nil {
		return err
	}

	// Write the word data to the EEPROM Data register (low 2 bytes).
	dgaddr = ee.addr
	dgaddr.SetOffset(ecad.EEPROMData)
	wb = []byte{uint8(word), uint8(word >> 8)}
	err = ecmd.ExecuteWrite(ee.comm, dgaddr, wb, 1)
	if err != nil {
		return err
	}

	err = ee.waitForIdle(250 * time.Millisecond)
	return err
}

// Close marks the EEPROM accessor as closed.
func (ee *blindEEPROM) Close() error {
	ee.closed = true
	return nil
}
