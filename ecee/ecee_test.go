package ecee

import (
	"errors"
	"testing"
	"time"

	"github.com/anviod/EtherCAT/ecad"
	"github.com/anviod/EtherCAT/ecfr"
	"github.com/anviod/EtherCAT/ecmd"
)

// ---------------------------------------------------------------------------
// mockCommander — implements ecmd.Commander for testing
// ---------------------------------------------------------------------------

// mockCommander implements ecmd.Commander and allows configuring the
// responses returned to the code under test.
type mockCommander struct {
	// lastCmd stores the most recently created ExecutingCommand so that
	// Cycle() can inspect and populate it.
	lastCmd *ecmd.ExecutingCommand

	// readData maps register offsets to the byte data returned on reads.
	readData map[uint16][]byte

	// statusSeq is a sequence of uint16 status values returned by successive
	// reads of the EEPROMControlStatus register. When the sequence is
	// exhausted the last value is repeated.
	statusSeq []uint16
	statusIdx int

	// writeAddr and writeData record the last write offset and payload.
	writeAddr uint16
	writeData []byte

	// Per-offset read errors.
	readErr map[uint16]error

	// Global errors.
	cycleErr error
	newErr   error
}

// getStatusResponse returns the next status value from the sequence, or
// repeats the last value when the sequence is exhausted. If the sequence
// is empty it returns the busy flag (0x0010) so timeout tests work.
func (m *mockCommander) getStatusResponse() uint16 {
	if m.statusIdx < len(m.statusSeq) {
		val := m.statusSeq[m.statusIdx]
		m.statusIdx++
		return val
	}
	if len(m.statusSeq) > 0 {
		return m.statusSeq[len(m.statusSeq)-1]
	}
	return 0x0010 // busy — forces timeout
}

// New creates a new ExecutingCommand with the given data length.
func (m *mockCommander) New(datalen int) (*ecmd.ExecutingCommand, error) {
	if m.newErr != nil {
		return nil, m.newErr
	}

	// Outgoing datagram buffer.
	outBuf := make([]byte, ecfr.DatagramOverheadLength+datalen)
	dgOut, err := ecfr.PointDatagramTo(outBuf)
	if err != nil {
		return nil, err
	}
	if err := dgOut.SetDataLen(datalen); err != nil {
		return nil, err
	}

	// Incoming datagram buffer.
	inBuf := make([]byte, ecfr.DatagramOverheadLength+datalen)
	dgIn, err := ecfr.PointDatagramTo(inBuf)
	if err != nil {
		return nil, err
	}
	if err := dgIn.SetDataLen(datalen); err != nil {
		return nil, err
	}

	m.lastCmd = &ecmd.ExecutingCommand{
		DatagramOut: &dgOut,
		DatagramIn:  &dgIn,
	}
	return m.lastCmd, nil
}

// Cycle processes the last created command by populating the incoming
// datagram based on the offset of the outgoing datagram.
func (m *mockCommander) Cycle() error {
	if m.cycleErr != nil {
		return m.cycleErr
	}

	cmd := m.lastCmd
	if cmd == nil {
		return nil
	}

	offset := cmd.DatagramOut.Header.OffsetAddr()

	if cmd.DatagramOut.Header.Command.DoesRead() {
		// Check for a per-offset read error.
		// Must set Arrived and Overlayed so that ChooseDefaultError
		// returns cmd.Error instead of NoFrame/NoOverlay.
		if err, ok := m.readErr[offset]; ok {
			cmd.Error = err
			cmd.Arrived = true
			cmd.Overlayed = true
			cmd.DatagramIn.WKC = 1
			return nil
		}

		var data []byte
		if offset == uint16(ecad.EEPROMControlStatus) {
			status := m.getStatusResponse()
			data = make([]byte, 2)
			data[0] = byte(status)
			data[1] = byte(status >> 8)
		} else if d, ok := m.readData[offset]; ok {
			data = make([]byte, len(d))
			copy(data, d)
		}

		// If no data configured, leave the zero-initialised buffer.
		if data != nil {
			n := len(data)
			if n > len(cmd.DatagramIn.Data) {
				n = len(cmd.DatagramIn.Data)
			}
			copy(cmd.DatagramIn.Data, data[:n])
		}
		cmd.DatagramIn.WKC = 1
		cmd.Arrived = true
		cmd.Overlayed = true
	} else if cmd.DatagramOut.Header.Command.DoesWrite() {
		m.writeAddr = offset
		m.writeData = make([]byte, len(cmd.DatagramOut.Data))
		copy(m.writeData, cmd.DatagramOut.Data)
		cmd.Arrived = true
		cmd.Overlayed = true
		cmd.DatagramIn.WKC = 1
	}

	return nil
}

// Close is a no-op for the mock.
func (m *mockCommander) Close() error {
	return nil
}

// ---------------------------------------------------------------------------
// helper: create a test DatagramAddress
// ---------------------------------------------------------------------------

func testAddr() ecfr.DatagramAddress {
	return ecfr.PositionalAddr(0, 0)
}

// ---------------------------------------------------------------------------
// TestNew
// ---------------------------------------------------------------------------

func TestNew(t *testing.T) {
	mc := &mockCommander{}
	ee, err := New(mc, testAddr())
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if ee == nil {
		t.Fatal("New returned nil EEPROM")
	}
}

// ---------------------------------------------------------------------------
// TestReadWord
// ---------------------------------------------------------------------------

func TestReadWord_Basic(t *testing.T) {
	mc := &mockCommander{
		// Two idle responses for the two waitForIdle calls.
		statusSeq: []uint16{0x0000, 0x0000},
		// EEPROMData returns 0xABCD in the low 2 bytes.
		readData: map[uint16][]byte{
			uint16(ecad.EEPROMData): {0xCD, 0xAB, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
	}
	ee, err := New(mc, testAddr())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	word, err := ee.ReadWord(0x0040)
	if err != nil {
		t.Fatalf("ReadWord failed: %v", err)
	}
	if word != 0xABCD {
		t.Fatalf("ReadWord: want 0xABCD, got 0x%04X", word)
	}

	// Verify the address was written correctly.
	if mc.writeAddr != uint16(ecad.EEPROMAddress) {
		t.Errorf("writeAddr = 0x%04X, want 0x%04X", mc.writeAddr, ecad.EEPROMAddress)
	}
	// Address 0x0040 should have been written as 4 bytes LE.
	if len(mc.writeData) != 4 {
		t.Fatalf("writeData len = %d, want 4", len(mc.writeData))
	}
	if mc.writeData[0] != 0x40 || mc.writeData[1] != 0x00 || mc.writeData[2] != 0x00 || mc.writeData[3] != 0x00 {
		t.Errorf("writeData = % x, want 40 00 00 00", mc.writeData)
	}
}

func TestReadWord_Timeout(t *testing.T) {
	mc := &mockCommander{
		// Always busy — forces waitForIdle to time out.
		statusSeq: []uint16{},
	}
	ee, err := New(mc, testAddr())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	start := time.Now()
	_, err = ee.ReadWord(0x0040)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("ReadWord should have returned timeout error")
	}
	if err.Error() != "EEPROM timeout" {
		t.Errorf("unexpected error: %v", err)
	}
	// Should time out roughly within 250ms + some margin.
	if elapsed > 500*time.Millisecond {
		t.Errorf("timeout took too long: %v", elapsed)
	}
}

func TestReadWord_Error(t *testing.T) {
	mc := &mockCommander{
		// Error bit set (bit 1 = 0x0002).
		statusSeq: []uint16{0x0002},
	}
	ee, err := New(mc, testAddr())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	_, err = ee.ReadWord(0x0040)
	if err == nil {
		t.Fatal("ReadWord should have returned error")
	}
	if err.Error() != "EEPROM error" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReadWord_Closed(t *testing.T) {
	mc := &mockCommander{}
	ee, err := New(mc, testAddr())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ee.Close()

	_, err = ee.ReadWord(0x0040)
	if err == nil {
		t.Fatal("ReadWord on closed EEPROM should return error")
	}
	if err.Error() != "ecee eeprom is already closed" {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestWriteWord
// ---------------------------------------------------------------------------

func TestWriteWord_Basic(t *testing.T) {
	mc := &mockCommander{
		// Two idle responses for the two waitForIdle calls.
		statusSeq: []uint16{0x0000, 0x0000},
	}
	ee, err := New(mc, testAddr())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	err = ee.WriteWord(0x0060, 0xBEEF)
	if err != nil {
		t.Fatalf("WriteWord failed: %v", err)
	}

	// The last write should have been to EEPROMData with the word value.
	if mc.writeAddr != uint16(ecad.EEPROMData) {
		t.Errorf("last writeAddr = 0x%04X, want 0x%04X (EEPROMData)", mc.writeAddr, ecad.EEPROMData)
	}
	if len(mc.writeData) != 2 {
		t.Fatalf("writeData len = %d, want 2", len(mc.writeData))
	}
	if mc.writeData[0] != 0xEF || mc.writeData[1] != 0xBE {
		t.Errorf("writeData = % x, want EF BE", mc.writeData)
	}
}

func TestWriteWord_Timeout(t *testing.T) {
	mc := &mockCommander{
		// Always busy.
		statusSeq: []uint16{},
	}
	ee, err := New(mc, testAddr())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	start := time.Now()
	err = ee.WriteWord(0x0060, 0xBEEF)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("WriteWord should have returned timeout error")
	}
	if err.Error() != "EEPROM timeout" {
		t.Errorf("unexpected error: %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("timeout took too long: %v", elapsed)
	}
}

func TestWriteWord_Closed(t *testing.T) {
	mc := &mockCommander{}
	ee, err := New(mc, testAddr())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ee.Close()

	err = ee.WriteWord(0x0060, 0xBEEF)
	if err == nil {
		t.Fatal("WriteWord on closed EEPROM should return error")
	}
	if err.Error() != "ecee eeprom is already closed" {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestClose
// ---------------------------------------------------------------------------

func TestClose(t *testing.T) {
	mc := &mockCommander{}
	ee, err := New(mc, testAddr())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	err = ee.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify that subsequent operations fail.
	_, err = ee.ReadWord(0)
	if err == nil {
		t.Fatal("ReadWord after Close should return error")
	}
}

// ---------------------------------------------------------------------------
// TestWaitForIdle
// ---------------------------------------------------------------------------

func TestWaitForIdle_ImmediateIdle(t *testing.T) {
	mc := &mockCommander{
		statusSeq: []uint16{0x0000}, // idle immediately
	}
	ee := &blindEEPROM{
		comm: mc,
		addr: testAddr(),
	}

	err := ee.waitForIdle(0)
	if err != nil {
		t.Fatalf("waitForIdle should not error on immediate idle: %v", err)
	}
}

func TestWaitForIdle_AfterFewRetries(t *testing.T) {
	mc := &mockCommander{
		// Busy twice, then idle.
		statusSeq: []uint16{0x0010, 0x0010, 0x0000},
	}
	ee := &blindEEPROM{
		comm: mc,
		addr: testAddr(),
	}

	err := ee.waitForIdle(0)
	if err != nil {
		t.Fatalf("waitForIdle should succeed after retries: %v", err)
	}
	// Should have consumed exactly 3 status values.
	if mc.statusIdx != 3 {
		t.Errorf("expected 3 status reads, got %d", mc.statusIdx)
	}
}

func TestWaitForIdle_Timeout(t *testing.T) {
	mc := &mockCommander{
		// Empty sequence — always returns busy.
		statusSeq: []uint16{},
	}
	ee := &blindEEPROM{
		comm: mc,
		addr: testAddr(),
	}

	start := time.Now()
	err := ee.waitForIdle(0) // 0 → defaults to 250ms
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("waitForIdle should return timeout error")
	}
	if err.Error() != "EEPROM timeout" {
		t.Errorf("unexpected error: %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("timeout took too long: %v", elapsed)
	}
}

func TestWaitForIdle_ErrorBit(t *testing.T) {
	mc := &mockCommander{
		statusSeq: []uint16{0x0002}, // error bit set
	}
	ee := &blindEEPROM{
		comm: mc,
		addr: testAddr(),
	}

	err := ee.waitForIdle(0)
	if err == nil {
		t.Fatal("waitForIdle should return error when error bit is set")
	}
	if err.Error() != "EEPROM error" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWaitForIdle_CustomTimeout(t *testing.T) {
	mc := &mockCommander{
		statusSeq: []uint16{}, // always busy
	}
	ee := &blindEEPROM{
		comm: mc,
		addr: testAddr(),
	}

	start := time.Now()
	err := ee.waitForIdle(50 * time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("waitForIdle should return timeout error")
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("custom timeout took too long: %v", elapsed)
	}
}

// ---------------------------------------------------------------------------
// TestWaitForIdle_CommanderError
// ---------------------------------------------------------------------------

func TestWaitForIdle_CommanderReadError(t *testing.T) {
	mc := &mockCommander{
		readErr: map[uint16]error{
			uint16(ecad.EEPROMControlStatus): errors.New("comm read error"),
		},
	}
	ee := &blindEEPROM{
		comm: mc,
		addr: testAddr(),
	}

	err := ee.waitForIdle(0)
	if err == nil {
		t.Fatal("waitForIdle should propagate commander error")
	}
	if err.Error() != "comm read error" {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkReadWord(b *testing.B) {
	mc := &mockCommander{
		statusSeq: []uint16{0x0000, 0x0000},
		readData: map[uint16][]byte{
			uint16(ecad.EEPROMData): {0xCD, 0xAB, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
	}
	ee, _ := New(mc, testAddr())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Reset the mock state for each iteration.
		mc.statusIdx = 0
		_, _ = ee.ReadWord(0x0040)
	}
}

func BenchmarkWriteWord(b *testing.B) {
	mc := &mockCommander{
		statusSeq: []uint16{0x0000, 0x0000},
	}
	ee, _ := New(mc, testAddr())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mc.statusIdx = 0
		_ = ee.WriteWord(0x0060, 0xBEEF)
	}
}
