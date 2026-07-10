package sim

import (
	"testing"

	"github.com/anviod/EtherCAT/ecad"
	"github.com/anviod/EtherCAT/ecfr"
)

// ─── Helpers ────────────────────────────────────────────────────────────────────

// makeTestFrame creates a test frame with a single datagram of the given
// command, address, data length, and optional data payload.
func makeTestFrame(command ecfr.CommandType, addr32 uint32, dataLen int, data []byte) *ecfr.Frame {
	buf := make([]byte, 256)
	frame, err := ecfr.PointFrameTo(buf)
	if err != nil {
		panic(err)
	}
	dg, err := frame.NewDatagram(dataLen)
	if err != nil {
		panic(err)
	}
	dg.Header.Command = command
	dg.Header.Addr32 = addr32
	dg.Header.SetLast(true)
	if len(data) > 0 {
		copy(dg.Data, data)
	}
	return &frame
}

// makeMultiDatagramFrame creates a test frame with multiple datagrams.
func makeMultiDatagramFrame(specs []struct {
	cmd     ecfr.CommandType
	addr32  uint32
	dataLen int
	data    []byte
}) *ecfr.Frame {
	buf := make([]byte, 512)
	frame, err := ecfr.PointFrameTo(buf)
	if err != nil {
		panic(err)
	}
	for i, spec := range specs {
		dg, err := frame.NewDatagram(spec.dataLen)
		if err != nil {
			panic(err)
		}
		dg.Header.Command = spec.cmd
		dg.Header.Addr32 = spec.addr32
		if i == len(specs)-1 {
			dg.Header.SetLast(true)
		}
		if len(spec.data) > 0 {
			copy(dg.Data, spec.data)
		}
	}
	return &frame
}

// ─── DevMapping Tests ───────────────────────────────────────────────────────────

func TestDevMapping(t *testing.T) {
	dm := DevMapping{StartAddr: 0x0120, LengthField: 2, DeviceField: NewALStatusControl().ControlReg()}
	if dm.Start() != 0x0120 {
		t.Errorf("Start() = %d, want 0x0120", dm.Start())
	}
	if dm.Length() != 2 {
		t.Errorf("Length() = %d, want 2", dm.Length())
	}
	if dm.Device() == nil {
		t.Error("Device() returned nil")
	}
}

// ─── ALControl Tests ────────────────────────────────────────────────────────────

func TestALControlRead(t *testing.T) {
	sc := NewALStatusControl()
	ctrl := sc.ControlReg()

	// Byte 0: state=INIT(1), no flags
	ctrl.wdState = 1
	var dp uint8
	ctrl.Read(0, &dp)
	if dp != 0x01 {
		t.Errorf("byte 0 = 0x%02X, want 0x01", dp)
	}

	// Byte 0: state=INIT(1), ack=true
	ctrl.ack = true
	ctrl.Read(0, &dp)
	if dp != 0x11 {
		t.Errorf("byte 0 with ack = 0x%02X, want 0x11", dp)
	}
	ctrl.ack = false

	// Byte 0: state=INIT(1), errorInd=true
	ctrl.errorInd = true
	ctrl.Read(0, &dp)
	if dp != 0x21 {
		t.Errorf("byte 0 with errorInd = 0x%02X, want 0x21", dp)
	}
	ctrl.errorInd = false

	// Byte 0: state=INIT(1), devIDReq=true
	ctrl.devIDReq = true
	ctrl.Read(0, &dp)
	if dp != 0x41 {
		t.Errorf("byte 0 with devIDReq = 0x%02X, want 0x41", dp)
	}
	ctrl.devIDReq = false

	// Byte 0: state=INIT(1), idSel=true
	ctrl.idSel = true
	ctrl.Read(0, &dp)
	if dp != 0x81 {
		t.Errorf("byte 0 with idSel = 0x%02X, want 0x81", dp)
	}
	ctrl.idSel = false

	// Byte 1: all zero
	ctrl.Read(1, &dp)
	if dp != 0x00 {
		t.Errorf("byte 1 zero = 0x%02X, want 0x00", dp)
	}

	// Byte 1: wdDiv=2, wdDiv2=5
	ctrl.wdDiv = 2
	ctrl.wdDiv2 = 5
	ctrl.Read(1, &dp)
	if dp != 0xB0 {
		t.Errorf("byte 1 with divs = 0x%02X, want 0xB0", dp)
	}
}

func TestALControlWriteInteract(t *testing.T) {
	sc := NewALStatusControl()
	ctrl := sc.ControlReg()
	if !ctrl.WriteInteract(0) {
		t.Error("WriteInteract should return true")
	}
}

func TestALControlLatch(t *testing.T) {
	sc := NewALStatusControl()
	ctrl := sc.ControlReg()

	shadow := []byte{0x01, 0x00}
	mask := []bool{true, true}
	ctrl.Latch(shadow, mask)
	if ctrl.wdState != 0x01 {
		t.Errorf("state after latch = %d, want 0x01", ctrl.wdState)
	}
}

func TestALControlLatchErrorGate(t *testing.T) {
	sc := NewALStatusControl()
	ctrl := sc.ControlReg()

	// Put device in error state
	sc.SetError(true)

	// Try to change state while in error with ack=1 (should be blocked)
	shadow := []byte{0x12, 0x00} // state=2, ack=1
	mask := []bool{true, true}
	ctrl.Latch(shadow, mask)
	if ctrl.wdState != 0x00 {
		t.Errorf("state should be blocked when in error and ack is set, got %d", ctrl.wdState)
	}

	// Change state while in error but with ack=0 (should be allowed)
	shadow = []byte{0x02, 0x00} // state=2, ack=0
	mask = []bool{true, true}
	ctrl.Latch(shadow, mask)
	if ctrl.wdState != 0x02 {
		t.Errorf("state should be allowed with ack=0, got %d", ctrl.wdState)
	}
}

// ─── ALStatus Tests ─────────────────────────────────────────────────────────────

func TestALStatusRead(t *testing.T) {
	sc := NewALStatusControl()
	status := sc.StatusReg()

	// Set some internal state
	status.state = 0x04
	status.errInd = true
	status.devID = 0x1234

	var dp uint8
	status.Read(0, &dp)
	if dp != 0x04 {
		t.Errorf("byte 0 = 0x%02X, want 0x04", dp)
	}

	status.Read(1, &dp)
	if dp != 0x02 {
		t.Errorf("byte 1 = 0x%02X, want 0x02 (errInd)", dp)
	}

	status.Read(2, &dp)
	if dp != 0x34 {
		t.Errorf("byte 2 = 0x%02X, want 0x34 (devID low)", dp)
	}

	status.Read(3, &dp)
	if dp != 0x12 {
		t.Errorf("byte 3 = 0x%02X, want 0x12 (devID high)", dp)
	}

	status.Read(4, &dp)
	if dp != 0x00 {
		t.Errorf("byte 4 = 0x%02X, want 0x00", dp)
	}

	status.Read(5, &dp)
	if dp != 0x00 {
		t.Errorf("byte 5 = 0x%02X, want 0x00", dp)
	}
}

func TestALStatusWriteInteract(t *testing.T) {
	sc := NewALStatusControl()
	status := sc.StatusReg()
	if status.WriteInteract(0) {
		t.Error("ALStatus WriteInteract should return false (read-only)")
	}
}

func TestALStatusLatch(t *testing.T) {
	sc := NewALStatusControl()
	status := sc.StatusReg()
	// Latch is a no-op for ALStatus
	status.Latch(nil, nil)
}

func TestALStatusControlInError(t *testing.T) {
	sc := NewALStatusControl()
	if sc.InError() {
		t.Error("should not be in error initially")
	}
	sc.SetError(true)
	if !sc.InError() {
		t.Error("should be in error after SetError(true)")
	}
}

func TestALStatusControlSetError(t *testing.T) {
	sc := NewALStatusControl()
	sc.SetError(true)
	if !sc.InError() {
		t.Error("SetError(true) failed")
	}
	sc.SetError(false)
	if sc.InError() {
		t.Error("SetError(false) failed")
	}
}

func TestALStatusControlIsECATWritable(t *testing.T) {
	sc := NewALStatusControl()
	if !sc.IsECATWritable() {
		t.Error("IsECATWritable should return true")
	}
}

func TestALStatusControlControlReg(t *testing.T) {
	sc := NewALStatusControl()
	ctrl := sc.ControlReg()
	if ctrl == nil {
		t.Error("ControlReg() returned nil")
	}
}

func TestALStatusControlStatusReg(t *testing.T) {
	sc := NewALStatusControl()
	status := sc.StatusReg()
	if status == nil {
		t.Error("StatusReg() returned nil")
	}
}

// ─── L2Slave Tests ──────────────────────────────────────────────────────────────

func TestL2SlaveNew(t *testing.T) {
	slave := NewL2Slave()
	if slave.ALStatusControl == nil {
		t.Error("ALStatusControl is nil")
	}
	if slave.EEPROM == nil {
		t.Error("EEPROM is nil")
	}
	if slave.BackingMemory[0] != 0x11 {
		t.Errorf("ET1100 signature[0] = 0x%02X, want 0x11", slave.BackingMemory[0])
	}
	if slave.BackingMemory[1] != 0x00 {
		t.Errorf("ET1100 signature[1] = 0x%02X, want 0x00", slave.BackingMemory[1])
	}
}

func TestL2SlaveProcessFrameRead(t *testing.T) {
	slave := NewL2Slave()
	slave.BackingMemory[0x1000] = 0x42
	slave.BackingMemory[0x1001] = 0x43
	slave.BackingMemory[0x1002] = 0x44
	slave.BackingMemory[0x1003] = 0x45

	addr := ecfr.PositionalAddr(0, 0x1000)
	frame := makeTestFrame(ecfr.APRD, addr.Addr32(), 4, nil)

	slave.ProcessFrame(frame)

	dg := frame.Datagrams[0]
	if dg.Data[0] != 0x42 {
		t.Errorf("data[0] = 0x%02X, want 0x42", dg.Data[0])
	}
	if dg.Data[1] != 0x43 {
		t.Errorf("data[1] = 0x%02X, want 0x43", dg.Data[1])
	}
	if dg.Data[2] != 0x44 {
		t.Errorf("data[2] = 0x%02X, want 0x44", dg.Data[2])
	}
	if dg.Data[3] != 0x45 {
		t.Errorf("data[3] = 0x%02X, want 0x45", dg.Data[3])
	}
}

func TestL2SlaveProcessFrameWrite(t *testing.T) {
	slave := NewL2Slave()
	addr := ecfr.PositionalAddr(0, 0x1000)
	data := []byte{0xAA, 0xBB, 0xCC, 0xDD}
	frame := makeTestFrame(ecfr.APWR, addr.Addr32(), 4, data)

	slave.ProcessFrame(frame)

	if slave.BackingMemory[0x1000] != 0xAA {
		t.Errorf("BackingMemory[0x1000] = 0x%02X, want 0xAA", slave.BackingMemory[0x1000])
	}
	if slave.BackingMemory[0x1001] != 0xBB {
		t.Errorf("BackingMemory[0x1001] = 0x%02X, want 0xBB", slave.BackingMemory[0x1001])
	}
	if slave.BackingMemory[0x1002] != 0xCC {
		t.Errorf("BackingMemory[0x1002] = 0x%02X, want 0xCC", slave.BackingMemory[0x1002])
	}
	if slave.BackingMemory[0x1003] != 0xDD {
		t.Errorf("BackingMemory[0x1003] = 0x%02X, want 0xDD", slave.BackingMemory[0x1003])
	}
}

func TestL2SlaveProcessFrameWKC(t *testing.T) {
	slave := NewL2Slave()

	addr := ecfr.PositionalAddr(0, 0x1000)
	frame := makeTestFrame(ecfr.APRD, addr.Addr32(), 4, nil)
	slave.ProcessFrame(frame)
	if frame.Datagrams[0].WKC != 1 {
		t.Errorf("APRD WKC = %d, want 1", frame.Datagrams[0].WKC)
	}

	frame = makeTestFrame(ecfr.APWR, addr.Addr32(), 4, []byte{1, 2, 3, 4})
	slave.ProcessFrame(frame)
	if frame.Datagrams[0].WKC != 1 {
		t.Errorf("APWR WKC = %d, want 1", frame.Datagrams[0].WKC)
	}

	brdAddr := uint32(0x1000) << 16
	frame = makeTestFrame(ecfr.BRD, brdAddr, 4, nil)
	slave.ProcessFrame(frame)
	if frame.Datagrams[0].WKC != 1 {
		t.Errorf("BRD WKC = %d, want 1", frame.Datagrams[0].WKC)
	}
}

func TestL2SlaveProcessFrameMultiDatagram(t *testing.T) {
	slave := NewL2Slave()
	slave.BackingMemory[0x1000] = 0x11
	slave.BackingMemory[0x2000] = 0x22

	specs := []struct {
		cmd     ecfr.CommandType
		addr32  uint32
		dataLen int
		data    []byte
	}{
		{ecfr.APRD, ecfr.PositionalAddr(0, 0x1000).Addr32(), 4, nil},
		{ecfr.APRD, ecfr.PositionalAddr(0, 0x2000).Addr32(), 4, nil},
	}

	frame := makeMultiDatagramFrame(specs)
	slave.ProcessFrame(frame)

	if len(frame.Datagrams) != 2 {
		t.Fatalf("got %d datagrams, want 2", len(frame.Datagrams))
	}

	if frame.Datagrams[0].Data[0] != 0x11 {
		t.Errorf("dg1 data[0] = 0x%02X, want 0x11", frame.Datagrams[0].Data[0])
	}
	if frame.Datagrams[0].WKC != 1 {
		t.Errorf("dg1 WKC = %d, want 1", frame.Datagrams[0].WKC)
	}

	if frame.Datagrams[1].Data[0] != 0x22 {
		t.Errorf("dg2 data[0] = 0x%02X, want 0x22", frame.Datagrams[1].Data[0])
	}
	if frame.Datagrams[1].WKC != 1 {
		t.Errorf("dg2 WKC = %d, want 1", frame.Datagrams[1].WKC)
	}
}

func TestL2SlaveBroadcast(t *testing.T) {
	slave := NewL2Slave()
	slave.BackingMemory[0x1000] = 0x77

	frame := makeTestFrame(ecfr.BRD, uint32(0x1000)<<16, 4, nil)
	slave.ProcessFrame(frame)

	if frame.Datagrams[0].WKC != 1 {
		t.Errorf("BRD WKC = %d, want 1", frame.Datagrams[0].WKC)
	}
	if frame.Datagrams[0].Data[0] != 0x77 {
		t.Errorf("BRD data[0] = 0x%02X, want 0x77", frame.Datagrams[0].Data[0])
	}
}

func TestL2SlaveNonPhysicalAddr(t *testing.T) {
	slave := NewL2Slave()

	frame := makeTestFrame(ecfr.LRD, 0x10000000, 4, nil)
	slave.ProcessFrame(frame)

	if frame.Datagrams[0].WKC != 0 {
		t.Errorf("LRD WKC = %d, want 0 (logical addresses not supported)", frame.Datagrams[0].WKC)
	}
}

func TestL2SlaveRegisterRead(t *testing.T) {
	slave := NewL2Slave()

	addr := ecfr.PositionalAddr(0, ecad.ALControl)
	frame := makeTestFrame(ecfr.APRD, addr.Addr32(), 2, nil)

	slave.ProcessFrame(frame)

	dg := frame.Datagrams[0]
	if dg.WKC != 1 {
		t.Errorf("WKC = %d, want 1", dg.WKC)
	}
	if dg.Data[0] != 0x00 {
		t.Errorf("ALControl byte 0 = 0x%02X, want 0x00", dg.Data[0])
	}
}

func TestL2SlaveRegisterWrite(t *testing.T) {
	slave := NewL2Slave()

	addr := ecfr.PositionalAddr(0, ecad.ALControl)
	data := []byte{0x04, 0x00}
	frame := makeTestFrame(ecfr.APWR, addr.Addr32(), 2, data)

	slave.ProcessFrame(frame)

	var dp uint8
	slave.ALStatusControl.ControlReg().Read(0, &dp)
	if dp&0x0f != 0x04 {
		t.Errorf("ALControl state = 0x%02X, want 0x04", dp&0x0f)
	}
}

// ─── L2EEPROM Tests ─────────────────────────────────────────────────────────────

func TestL2EEPROMNew(t *testing.T) {
	ee := NewL2EEPROM()

	if ee.Array[0] != 0xee00 {
		t.Errorf("Array[0] = 0x%04X, want 0xee00", ee.Array[0])
	}
	if ee.Array[100] != 0xee00+100 {
		t.Errorf("Array[100] = 0x%04X, want 0x%04X", ee.Array[100], 0xee00+100)
	}
	if ee.Busy {
		t.Error("Busy should be false initially")
	}
}

func TestL2EEPROMRegRead(t *testing.T) {
	ee := NewL2EEPROM()
	reg := ee.Reg()

	// Read control/status register (offset 0)
	var dp uint8
	reg.Read(0, &dp)
	if dp&0x80 != 0 {
		t.Error("Busy bit should be 0 initially")
	}

	// Read address register (offsets 4-7)
	reg.Read(4, &dp)
	reg.Read(5, &dp)
	reg.Read(6, &dp)
	reg.Read(7, &dp)

	// Read data register (offsets 8-15)
	reg.Read(8, &dp)
	reg.Read(9, &dp)
	reg.Read(10, &dp)
	reg.Read(11, &dp)
	reg.Read(12, &dp)
	reg.Read(13, &dp)
	reg.Read(14, &dp)
	reg.Read(15, &dp)
}

func TestL2EEPROMRegWriteInteract(t *testing.T) {
	ee := NewL2EEPROM()
	reg := ee.Reg()

	// Control register is writable
	if !reg.WriteInteract(0) {
		t.Error("WriteInteract(0) should be true")
	}
	if !reg.WriteInteract(1) {
		t.Error("WriteInteract(1) should be true")
	}

	// Address register is writable
	if !reg.WriteInteract(4) {
		t.Error("WriteInteract(4) should be true")
	}
}

func TestL2EEPROMDataAccess(t *testing.T) {
	ee := NewL2EEPROM()
	reg := ee.Reg()

	// Set address to word offset 0 and issue read command
	shadow := []byte{0, 0, 0, 0x01, 0, 0, 0, 0}
	mask := []bool{false, false, false, true, false, false, false, false}
	reg.Latch(shadow, mask)

	// Read data at offset 8 (data register)
	var dp uint8
	reg.Read(8, &dp)
	expected := uint8(ee.Array[0] & 0xFF)
	if dp != expected {
		t.Errorf("data[0] low = 0x%02X, want 0x%02X", dp, expected)
	}
}

func TestL2EEPROMAddrLatch(t *testing.T) {
	ee := NewL2EEPROM()
	reg := ee.Reg()

	// Set EEPROM address to word offset 50 (addr goes to shadow[7] as LSB)
	addr := uint16(50)
	shadow := []byte{
		0, 0, // ctrl (offset 0-1)
		0, 0, // reserved (offset 2-3)
		0, 0, 0, byte(addr), // addr (offset 4-7): shadow[7]=LSB
	}
	mask := []bool{false, false, false, false, false, false, false, true}
	// Issue read command
	shadowCmd := []byte{0, 0, 0, 0x01, 0, 0, 0, 0}
	maskCmd := []bool{false, false, false, true, false, false, false, false}
	reg.Latch(shadow, mask)
	reg.Latch(shadowCmd, maskCmd)

	// Read data at offset 8 (data register)
	var dp uint8
	reg.Read(8, &dp)
	expected := uint8(ee.Array[addr] & 0xFF)
	if dp != expected {
		t.Errorf("data at addr %d = 0x%02X, want 0x%02X", addr, dp, expected)
	}
}

func TestL2EEPROMAddrLatchBugFix(t *testing.T) {
	ee := NewL2EEPROM()
	reg := ee.Reg()

	// Test that address latch works correctly for all 4 address bytes
	// Latch constructs Addr as: shadow[7] | shadow[6]<<8 | shadow[5]<<16 | shadow[4]<<24
	addr := uint16(0x1234)
	shadow := []byte{
		0, 0, // ctrl
		0, 0, // reserved
		0, 0, byte(addr >> 8), byte(addr), // addr: shadow[6]=0x12, shadow[7]=0x34
	}
	mask := []bool{false, false, false, false, false, false, true, true}
	// Issue read command
	shadowCmd := []byte{0, 0, 0, 0x01, 0, 0, 0, 0}
	maskCmd := []bool{false, false, false, true, false, false, false, false}
	reg.Latch(shadow, mask)
	reg.Latch(shadowCmd, maskCmd)

	var dp uint8
	reg.Read(8, &dp)
	expected := uint8(ee.Array[addr] & 0xFF)
	if dp != expected {
		t.Errorf("data at addr 0x%04X = 0x%02X, want 0x%02X", addr, dp, expected)
	}
}

func TestL2EEPROMAddrLatchPartial(t *testing.T) {
	ee := NewL2EEPROM()
	reg := ee.Reg()

	// Set initial address to word offset 0x10 via shadow[7]
	shadow := []byte{0, 0, 0, 0, 0, 0, 0, 0x10}
	mask := []bool{false, false, false, false, false, false, false, true}
	// Issue read command
	shadowCmd := []byte{0, 0, 0, 0x01, 0, 0, 0, 0}
	maskCmd := []bool{false, false, false, true, false, false, false, false}
	reg.Latch(shadow, mask)
	reg.Latch(shadowCmd, maskCmd)

	var dp uint8
	reg.Read(8, &dp)
	expected := uint8(ee.Array[0x10] & 0xFF)
	if dp != expected {
		t.Errorf("partial latch data = 0x%02X, want 0x%02X", dp, expected)
	}
}

func TestL2EEPROMCommandLatch(t *testing.T) {
	ee := NewL2EEPROM()
	reg := ee.Reg()

	// Set address first
	addr := uint16(10)
	shadow := []byte{0, 0, 0, 0, 0, 0, 0, byte(addr)}
	mask := []bool{false, false, false, false, false, false, false, true}
	reg.Latch(shadow, mask)

	// Now issue read command (command 0x01 = read)
	shadow = []byte{0, 0, 0, 0x01, 0, 0, 0, 0}
	mask = []bool{false, false, false, true, false, false, false, false}
	reg.Latch(shadow, mask)

	// Verify busy is cleared after read command completes
	var dp uint8
	reg.Read(3, &dp)
	if dp&0x80 != 0 {
		t.Error("EEPROM should not be busy after read command completes")
	}

	// Verify data is readable
	reg.Read(8, &dp)
	expected := uint8(ee.Array[addr] & 0xFF)
	if dp != expected {
		t.Errorf("data at addr %d = 0x%02X, want 0x%02X", addr, dp, expected)
	}
}

func TestL2EEPROMDataScratchWrite(t *testing.T) {
	ee := NewL2EEPROM()
	reg := ee.Reg()

	// Set address
	addr := uint16(20)
	shadow := []byte{0, 0, 0, 0, 0, 0, 0, byte(addr)}
	mask := []bool{false, false, false, false, false, false, false, true}
	reg.Latch(shadow, mask)

	// Write data to scratch register (offsets 8-15)
	shadow = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0xAB, 0xCD, 0, 0, 0, 0, 0, 0}
	mask = make([]bool, 16)
	for i := 8; i < 16; i++ {
		mask[i] = true
	}
	reg.Latch(shadow, mask)

	// Verify scratch data is accessible
	var dp uint8
	reg.Read(8, &dp)
	if dp != 0xAB {
		t.Errorf("scratch data[0] = 0x%02X, want 0xAB", dp)
	}
}

// ─── L2Bus Tests ────────────────────────────────────────────────────────────────

func TestL2BusNew(t *testing.T) {
	bus := &L2Bus{}

	frame, err := bus.New(256)
	if err != nil {
		t.Fatal(err)
	}
	if frame == nil {
		t.Fatal("New() returned nil frame")
	}

	dg, err := frame.NewDatagram(4)
	if err != nil {
		t.Fatal(err)
	}
	if dg == nil {
		t.Fatal("NewDatagram returned nil")
	}
}

func TestL2BusCycle(t *testing.T) {
	bus := &L2Bus{}
	slave := NewL2Slave()
	slave.BackingMemory[0x1000] = 0x42
	bus.Slaves = append(bus.Slaves, slave)

	frame, err := bus.New(256)
	if err != nil {
		t.Fatal(err)
	}
	dg, err := frame.NewDatagram(4)
	if err != nil {
		t.Fatal(err)
	}
	dg.Header.Command = ecfr.APRD
	dg.Header.Addr32 = ecfr.PositionalAddr(0, 0x1000).Addr32()
	dg.Header.SetLast(true)

	iframes, err := bus.Cycle()
	if err != nil {
		t.Fatal(err)
	}

	if len(iframes) != 1 {
		t.Fatalf("got %d frames, want 1", len(iframes))
	}

	idg := iframes[0].Datagrams[0]
	if idg.WKC != 1 {
		t.Errorf("WKC = %d, want 1", idg.WKC)
	}
	if idg.Data[0] != 0x42 {
		t.Errorf("data[0] = 0x%02X, want 0x42", idg.Data[0])
	}
}

func TestL2BusMultiSlave(t *testing.T) {
	bus := &L2Bus{}

	slave1 := NewL2Slave()
	slave1.BackingMemory[0x1000] = 0x11
	slave2 := NewL2Slave()
	slave2.BackingMemory[0x1000] = 0x22

	bus.Slaves = append(bus.Slaves, slave1, slave2)

	frame, _ := bus.New(256)
	dg, _ := frame.NewDatagram(4)
	dg.Header.Command = ecfr.BRD
	dg.Header.Addr32 = uint32(0x1000) << 16
	dg.Header.SetLast(true)

	iframes, err := bus.Cycle()
	if err != nil {
		t.Fatal(err)
	}

	if len(iframes) != 1 {
		t.Fatalf("got %d frames, want 1", len(iframes))
	}

	idg := iframes[0].Datagrams[0]
	if idg.WKC != 2 {
		t.Errorf("WKC = %d, want 2 (both slaves)", idg.WKC)
	}
	if idg.Data[0] != 0x22 {
		t.Errorf("data[0] = 0x%02X, want 0x22 (last slave)", idg.Data[0])
	}
}

func TestL2BusClose(t *testing.T) {
	bus := &L2Bus{}
	if err := bus.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

func TestL2BusFramerInterface(t *testing.T) {
	var _ Framer = (*L2Bus)(nil)
}

func TestL2BusCycleEmpty(t *testing.T) {
	bus := &L2Bus{}
	bus.Slaves = append(bus.Slaves, NewL2Slave())

	iframes, err := bus.Cycle()
	if err != nil {
		t.Fatal(err)
	}
	if len(iframes) != 0 {
		t.Errorf("got %d frames, want 0", len(iframes))
	}
}

func TestL2BusCycleMultipleFrames(t *testing.T) {
	bus := &L2Bus{}
	slave := NewL2Slave()
	slave.BackingMemory[0x1000] = 0xAA
	slave.BackingMemory[0x2000] = 0xBB
	bus.Slaves = append(bus.Slaves, slave)

	for i := 0; i < 2; i++ {
		frame, _ := bus.New(256)
		dg, _ := frame.NewDatagram(4)
		dg.Header.Command = ecfr.APRD
		if i == 0 {
			dg.Header.Addr32 = ecfr.PositionalAddr(0, 0x1000).Addr32()
		} else {
			dg.Header.Addr32 = ecfr.PositionalAddr(0, 0x2000).Addr32()
		}
		dg.Header.SetLast(true)
	}

	iframes, err := bus.Cycle()
	if err != nil {
		t.Fatal(err)
	}

	if len(iframes) != 2 {
		t.Fatalf("got %d frames, want 2", len(iframes))
	}

	if iframes[0].Datagrams[0].Data[0] != 0xAA {
		t.Errorf("frame 0 data[0] = 0x%02X, want 0xAA", iframes[0].Datagrams[0].Data[0])
	}
	if iframes[1].Datagrams[0].Data[0] != 0xBB {
		t.Errorf("frame 1 data[0] = 0x%02X, want 0xBB", iframes[1].Datagrams[0].Data[0])
	}
}

// ─── Benchmarks ─────────────────────────────────────────────────────────────────

func BenchmarkL2SlaveProcessFrame(b *testing.B) {
	slave := NewL2Slave()
	addr := ecfr.PositionalAddr(0, 0x1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		frame := makeTestFrame(ecfr.APRD, addr.Addr32(), 4, nil)
		slave.ProcessFrame(frame)
	}
}

func BenchmarkL2EEPROMRegRead(b *testing.B) {
	ee := NewL2EEPROM()
	reg := ee.Reg()
	var dp uint8

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for offs := uint16(0); offs < 16; offs++ {
			reg.Read(offs, &dp)
		}
	}
}

func BenchmarkL2BusCycle(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus := &L2Bus{}
		slave := NewL2Slave()
		bus.Slaves = append(bus.Slaves, slave)

		frame, _ := bus.New(256)
		dg, _ := frame.NewDatagram(4)
		dg.Header.Command = ecfr.APRD
		dg.Header.Addr32 = ecfr.PositionalAddr(0, 0x1000).Addr32()
		dg.Header.SetLast(true)

		bus.Cycle()
	}
}

// ─── Bulk Processing Benchmarks ─────────────────────────────────────────────────

// BenchmarkL2SlaveProcessFrameBulk measures bulk read throughput for
// non-register memory (>= 0x1000) with 100 bytes of data. This exercises
// the fast copy() path in ProcessFrame.
func BenchmarkL2SlaveProcessFrameBulk(b *testing.B) {
	b.ReportAllocs()
	slave := NewL2Slave()
	for i := 0; i < 100; i++ {
		slave.BackingMemory[0x1000+uint16(i)] = byte(i)
	}
	addr := ecfr.PositionalAddr(0, 0x1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		frame := makeTestFrame(ecfr.APRD, addr.Addr32(), 100, nil)
		slave.ProcessFrame(frame)
	}
}

// BenchmarkL2SlaveProcessFrameBulkLarge measures bulk read throughput for
// non-register memory with 1000 bytes of data — a typical process data
// image size.
func BenchmarkL2SlaveProcessFrameBulkLarge(b *testing.B) {
	b.ReportAllocs()
	slave := NewL2Slave()
	for i := 0; i < 1000; i++ {
		slave.BackingMemory[0x1000+uint16(i)] = byte(i)
	}
	addr := ecfr.PositionalAddr(0, 0x1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := make([]byte, 2048)
		frame, err := ecfr.PointFrameTo(buf)
		if err != nil {
			b.Fatal(err)
		}
		dg, err := frame.NewDatagram(1000)
		if err != nil {
			b.Fatal(err)
		}
		dg.Header.Command = ecfr.APRD
		dg.Header.Addr32 = addr.Addr32()
		dg.Header.SetLast(true)
		slave.ProcessFrame(&frame)
	}
}

// BenchmarkL2SlaveProcessFrameRegister measures register-area processing
// throughput (byte-by-byte path) for the AL Control register.
func BenchmarkL2SlaveProcessFrameRegister(b *testing.B) {
	b.ReportAllocs()
	slave := NewL2Slave()
	addr := ecfr.PositionalAddr(0, ecad.ALControl)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		frame := makeTestFrame(ecfr.APRD, addr.Addr32(), 2, nil)
		slave.ProcessFrame(frame)
	}
}

// BenchmarkL2BusFullPipeline measures the full bus pipeline: create a bus,
// add a slave, create a frame with a datagram, run Cycle, and verify the
// result.
func BenchmarkL2BusFullPipeline(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus := &L2Bus{}
		slave := NewL2Slave()
		slave.BackingMemory[0x1000] = 0x42
		bus.Slaves = append(bus.Slaves, slave)

		frame, _ := bus.New(256)
		dg, _ := frame.NewDatagram(4)
		dg.Header.Command = ecfr.APRD
		dg.Header.Addr32 = ecfr.PositionalAddr(0, 0x1000).Addr32()
		dg.Header.SetLast(true)

		iframes, _ := bus.Cycle()
		if len(iframes) != 1 || iframes[0].Datagrams[0].Data[0] != 0x42 {
			b.Fatal("pipeline verification failed")
		}
	}
}

// ─── Boundary Tests ─────────────────────────────────────────────────────────────

// TestL2SlaveBulkReadBoundary verifies correct behavior at the register
// area boundary (0x0FFF / 0x1000). Addresses below 0x1000 use the
// byte-by-byte register path; addresses at or above 0x1000 use the bulk
// copy() path.
func TestL2SlaveBulkReadBoundary(t *testing.T) {
	slave := NewL2Slave()

	// Fill known values on both sides of the boundary
	slave.BackingMemory[0x0FFF] = 0xAB
	slave.BackingMemory[0x1000] = 0xCD
	slave.BackingMemory[0x1001] = 0xEF

	// ── Register area: address 0x0FFF (byte-by-byte path) ──
	addr := ecfr.PositionalAddr(0, 0x0FFF)
	frame := makeTestFrame(ecfr.APRD, addr.Addr32(), 1, nil)
	slave.ProcessFrame(frame)
	if frame.Datagrams[0].WKC != 1 {
		t.Errorf("0x0FFF read WKC = %d, want 1", frame.Datagrams[0].WKC)
	}
	if frame.Datagrams[0].Data[0] != 0xAB {
		t.Errorf("0x0FFF data = 0x%02X, want 0xAB", frame.Datagrams[0].Data[0])
	}

	// ── Non-register area: address 0x1000 (bulk copy path) ──
	addr = ecfr.PositionalAddr(0, 0x1000)
	frame = makeTestFrame(ecfr.APRD, addr.Addr32(), 2, nil)
	slave.ProcessFrame(frame)
	if frame.Datagrams[0].WKC != 1 {
		t.Errorf("0x1000 read WKC = %d, want 1", frame.Datagrams[0].WKC)
	}
	if frame.Datagrams[0].Data[0] != 0xCD {
		t.Errorf("0x1000 data[0] = 0x%02X, want 0xCD", frame.Datagrams[0].Data[0])
	}
	if frame.Datagrams[0].Data[1] != 0xEF {
		t.Errorf("0x1000 data[1] = 0x%02X, want 0xEF", frame.Datagrams[0].Data[1])
	}
}

// TestL2SlaveMaxDataLength verifies that the slave correctly handles a
// large read across non-register memory (1500 bytes, near Ethernet MTU).
func TestL2SlaveMaxDataLength(t *testing.T) {
	slave := NewL2Slave()

	const dataLen = 1500
	for i := 0; i < dataLen; i++ {
		slave.BackingMemory[0x1000+uint16(i)] = byte(i & 0xFF)
	}

	addr := ecfr.PositionalAddr(0, 0x1000)

	// Create a frame large enough for the 1500-byte datagram
	buf := make([]byte, 2048)
	frame, err := ecfr.PointFrameTo(buf)
	if err != nil {
		t.Fatal(err)
	}
	dg, err := frame.NewDatagram(dataLen)
	if err != nil {
		t.Fatal(err)
	}
	dg.Header.Command = ecfr.APRD
	dg.Header.Addr32 = addr.Addr32()
	dg.Header.SetLast(true)

	slave.ProcessFrame(&frame)

	if frame.Datagrams[0].WKC != 1 {
		t.Errorf("WKC = %d, want 1", frame.Datagrams[0].WKC)
	}

	// Verify first and last bytes
	if frame.Datagrams[0].Data[0] != 0x00 {
		t.Errorf("data[0] = 0x%02X, want 0x00", frame.Datagrams[0].Data[0])
	}
	if frame.Datagrams[0].Data[dataLen-1] != byte((dataLen-1)&0xFF) {
		t.Errorf("data[%d] = 0x%02X, want 0x%02X", dataLen-1,
			frame.Datagrams[0].Data[dataLen-1], byte((dataLen-1)&0xFF))
	}
}

// ---------------------------------------------------------------------------
// 扩展热路径测试 (Extended Hot-Path Tests)
// ---------------------------------------------------------------------------

// TestL2BusCycleMultiSlave verifies L2Bus.Cycle correctly processes frames
// through multiple slaves using broadcast addressing.
// 验证 L2Bus.Cycle 使用广播寻址正确处理多从站帧处理。
func TestL2BusCycleMultiSlave(t *testing.T) {
	bus := &L2Bus{}
	for i := 0; i < 3; i++ {
		slave := NewL2Slave()
		slave.BackingMemory[0x1000] = byte(0x10 + i)
		bus.Slaves = append(bus.Slaves, slave)
	}

	frame, _ := bus.New(256)
	dg, _ := frame.NewDatagram(4)
	dg.Header.Command = ecfr.BRD
	dg.Header.Addr32 = ecfr.PositionalAddr(0, 0x1000).Addr32()
	dg.Header.SetLast(true)

	iframes, err := bus.Cycle()
	if err != nil {
		t.Fatalf("Cycle failed: %v", err)
	}
	if len(iframes) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(iframes))
	}
	// WKC should be incremented by all 3 slaves for broadcast
	if iframes[0].Datagrams[0].WKC != 3 {
		t.Errorf("expected WKC=3 (3 slaves broadcast), got %d", iframes[0].Datagrams[0].WKC)
	}
}

// TestL2BusCycleReadAfterWrite verifies L2Bus.Cycle correctly handles a
// write followed by a read across separate frames.
// 验证 L2Bus.Cycle 正确处理跨帧的写后读操作。
func TestL2BusCycleReadAfterWrite(t *testing.T) {
	bus := &L2Bus{}
	slave := NewL2Slave()
	bus.Slaves = append(bus.Slaves, slave)

	// Frame 1: Write 0x42 to 0x1000
	wframe, _ := bus.New(256)
	wdg, _ := wframe.NewDatagram(2)
	wdg.Header.Command = ecfr.APWR
	wdg.Header.Addr32 = ecfr.PositionalAddr(0, 0x1000).Addr32()
	wdg.Data[0] = 0x42
	wdg.Data[1] = 0x43
	wdg.Header.SetLast(true)

	iframes, err := bus.Cycle()
	if err != nil {
		t.Fatalf("Write cycle failed: %v", err)
	}
	if len(iframes) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(iframes))
	}

	// Frame 2: Read back from 0x1000
	rframe, _ := bus.New(256)
	rdg, _ := rframe.NewDatagram(2)
	rdg.Header.Command = ecfr.APRD
	rdg.Header.Addr32 = ecfr.PositionalAddr(0, 0x1000).Addr32()
	rdg.Header.SetLast(true)

	iframes, err = bus.Cycle()
	if err != nil {
		t.Fatalf("Read cycle failed: %v", err)
	}
	if len(iframes) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(iframes))
	}
	// The read should see the value written by the previous frame
	if iframes[0].Datagrams[0].Data[0] != 0x42 {
		t.Errorf("read data[0] = 0x%02X, want 0x42", iframes[0].Datagrams[0].Data[0])
	}
	if iframes[0].Datagrams[0].Data[1] != 0x43 {
		t.Errorf("read data[1] = 0x%02X, want 0x43", iframes[0].Datagrams[0].Data[1])
	}
}

// TestL2BusCycleBroadcast verifies L2Bus.Cycle handles broadcast datagrams
// (BRD) correctly, where all slaves increment WKC.
// 验证 L2Bus.Cycle 正确处理广播数据报 (BRD)，所有从站均递增 WKC。
func TestL2BusCycleBroadcast(t *testing.T) {
	bus := &L2Bus{}
	for i := 0; i < 5; i++ {
		slave := NewL2Slave()
		slave.BackingMemory[0x1000] = byte(0xAA)
		bus.Slaves = append(bus.Slaves, slave)
	}

	frame, _ := bus.New(256)
	dg, _ := frame.NewDatagram(4)
	dg.Header.Command = ecfr.BRD
	dg.Header.Addr32 = ecfr.PositionalAddr(0, 0x1000).Addr32()
	dg.Header.SetLast(true)

	iframes, err := bus.Cycle()
	if err != nil {
		t.Fatalf("Cycle failed: %v", err)
	}
	if len(iframes) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(iframes))
	}
	// All 5 slaves should increment WKC for broadcast
	if iframes[0].Datagrams[0].WKC != 5 {
		t.Errorf("expected WKC=5 (5 slaves broadcast), got %d", iframes[0].Datagrams[0].WKC)
	}
}

// TestL2BusCycleEmptySlaves verifies L2Bus.Cycle handles bus with no slaves.
// 验证 L2Bus.Cycle 处理无从站的总线。
func TestL2BusCycleEmptySlaves(t *testing.T) {
	bus := &L2Bus{}

	frame, _ := bus.New(256)
	dg, _ := frame.NewDatagram(4)
	dg.Header.Command = ecfr.APRD
	dg.Header.Addr32 = ecfr.PositionalAddr(0, 0x1000).Addr32()
	dg.Header.SetLast(true)

	iframes, err := bus.Cycle()
	if err != nil {
		t.Fatalf("Cycle with empty slaves should not error: %v", err)
	}
	if len(iframes) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(iframes))
	}
	// WKC should remain 0 with no slaves
	if iframes[0].Datagrams[0].WKC != 0 {
		t.Errorf("expected WKC=0 (no slaves), got %d", iframes[0].Datagrams[0].WKC)
	}
}

// TestL2BusCloseCleanup verifies L2Bus.Close correctly cleans up resources.
// 验证 L2Bus.Close 正确清理资源。
func TestL2BusCloseCleanup(t *testing.T) {
	bus := &L2Bus{}
	slave := NewL2Slave()
	bus.Slaves = append(bus.Slaves, slave)

	err := bus.Close()
	if err != nil {
		t.Errorf("Close should not error: %v", err)
	}
}

// TestL2SlaveNOPCommand verifies that a NOP command does not modify WKC.
// 验证 NOP 命令不修改 WKC。
func TestL2SlaveNOPCommand(t *testing.T) {
	slave := NewL2Slave()
	addr := ecfr.PositionalAddr(0, 0x1000)

	frame := makeTestFrame(ecfr.NOP, addr.Addr32(), 4, nil)
	slave.ProcessFrame(frame)

	// NOP commands should not increment WKC
	if frame.Datagrams[0].WKC != 0 {
		t.Errorf("NOP WKC = %d, want 0", frame.Datagrams[0].WKC)
	}
}

// ---------------------------------------------------------------------------
// 扩展热路径基准测试 (Extended Hot-Path Benchmarks)
// ---------------------------------------------------------------------------

// BenchmarkL2BusCycleMultiSlave benchmarks L2Bus.Cycle with 3 slaves to
// measure multi-slave bus cycle performance.
// 基准测试 3 从站 L2Bus.Cycle 性能。
func BenchmarkL2BusCycleMultiSlave(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus := &L2Bus{}
		for j := 0; j < 3; j++ {
			slave := NewL2Slave()
			slave.BackingMemory[0x1000] = byte(0x10 + j)
			bus.Slaves = append(bus.Slaves, slave)
		}

		frame, _ := bus.New(256)
		dg, _ := frame.NewDatagram(4)
		dg.Header.Command = ecfr.APRD
		dg.Header.Addr32 = ecfr.PositionalAddr(0, 0x1000).Addr32()
		dg.Header.SetLast(true)

		bus.Cycle()
	}
}

// BenchmarkL2BusFullPipelineMultiSlave benchmarks full pipeline with 3 slaves
// using broadcast addressing (create→fill→cycle→verify).
// 基准测试 3 从站完整管线性能（创建→填充→循环→验证），使用广播寻址。
func BenchmarkL2BusFullPipelineMultiSlave(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus := &L2Bus{}
		for j := 0; j < 3; j++ {
			slave := NewL2Slave()
			slave.BackingMemory[0x1000] = byte(0x42 + byte(j))
			bus.Slaves = append(bus.Slaves, slave)
		}

		frame, _ := bus.New(256)
		dg, _ := frame.NewDatagram(4)
		dg.Header.Command = ecfr.BRD
		dg.Header.Addr32 = ecfr.DatagramAddressFromCommand(ecfr.PositionalAddr(0, 0x1000).Addr32(), ecfr.BRD).Addr32()
		dg.Header.SetLast(true)

		iframes, _ := bus.Cycle()
		if len(iframes) != 1 || iframes[0].Datagrams[0].WKC != 3 {
			b.Fatal("pipeline verification failed")
		}
	}
}
