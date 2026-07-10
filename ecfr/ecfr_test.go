package ecfr

import (
	"bytes"
	"testing"
)

// ---------------------------------------------------------------------------
// Test CommandType DoesRead/DoesWrite
// ---------------------------------------------------------------------------

func TestCommandTypeDoesReadDoesWrite(t *testing.T) {
	testCases := []struct {
		ct    CommandType
		read  bool
		write bool
		name  string
	}{
		{NOP, false, false, "NOP"},
		{APRD, true, false, "APRD"},
		{APWR, false, true, "APWR"},
		{APRW, true, true, "APRW"},
		{FPRD, true, false, "FPRD"},
		{FPWR, false, true, "FPWR"},
		{FPRW, true, true, "FPRW"},
		{BRD, true, false, "BRD"},
		{BWR, false, true, "BWR"},
		{BRW, true, true, "BRW"},
		{LRD, true, false, "LRD"},
		{LWR, false, true, "LWR"},
		{LRW, true, true, "LRW"},
		{ARMW, true, true, "ARMW"},
		{FRMW, true, true, "FRMW"},
	}

	for _, tc := range testCases {
		if tc.ct.DoesRead() != tc.read {
			t.Errorf("%v.DoesRead() = %v, want %v", tc.name, tc.ct.DoesRead(), tc.read)
		}
		if tc.ct.DoesWrite() != tc.write {
			t.Errorf("%v.DoesWrite() = %v, want %v", tc.name, tc.ct.DoesWrite(), tc.write)
		}
		if tc.ct.String() != tc.name {
			t.Errorf("%v.String() = %q, want %q", tc.ct, tc.ct.String(), tc.name)
		}
	}

	unknown := CommandType(255)
	if unknown.String() != "CommandType(255)" {
		t.Errorf("CommandType(255).String() = %q, want CommandType(255)", unknown.String())
	}
}

// ---------------------------------------------------------------------------
// Test DatagramAddress
// ---------------------------------------------------------------------------

func TestDatagramAddressPositional(t *testing.T) {
	da := PositionalAddr(100, 0x100)
	if da.Type() != Positional {
		t.Errorf("got type %v, want Positional", da.Type())
	}
	if da.PositionOrAddress() != 100 {
		t.Errorf("position = %d, want 100", da.PositionOrAddress())
	}
	if da.Offset() != 0x100 {
		t.Errorf("offset = 0x%04x, want 0x0100", da.Offset())
	}
	if !da.IsPhysical() {
		t.Error("IsPhysical should be true for Positional")
	}
}

func TestDatagramAddressFixed(t *testing.T) {
	da := FixedAddr(0x1234, 0x5678)
	if da.Type() != Fixed {
		t.Errorf("got type %v, want Fixed", da.Type())
	}
	if da.PositionOrAddress() != 0x1234 {
		t.Errorf("station addr = 0x%04x, want 0x1234", da.PositionOrAddress())
	}
	if da.Offset() != 0x5678 {
		t.Errorf("offset = 0x%04x, want 0x5678", da.Offset())
	}
	if !da.IsPhysical() {
		t.Error("IsPhysical should be true for Fixed")
	}
}

func TestDatagramAddressSetOffset(t *testing.T) {
	da := PositionalAddr(1, 2)
	da.SetOffset(0x1234)
	if da.Offset() != 0x1234 {
		t.Errorf("after SetOffset: got 0x%04x, want 0x1234", da.Offset())
	}
	if da.PositionOrAddress() != 1 {
		t.Errorf("after SetOffset: position changed to %d, want 1", da.PositionOrAddress())
	}
}

func TestDatagramAddressIncrementSlaveAddr(t *testing.T) {
	da := PositionalAddr(5, 0x1234)
	da.IncrementSlaveAddr()
	if da.PositionOrAddress() != 6 {
		t.Errorf("after increment: position = %d, want 6", da.PositionOrAddress())
	}
	if da.Offset() != 0x1234 {
		t.Errorf("after increment: offset = 0x%04x, want 0x1234", da.Offset())
	}
}

func TestDatagramAddressFromCommand(t *testing.T) {
	testCases := []struct {
		ct   CommandType
		want DatagramAddressType
	}{
		{NOP, UninitializedDatagramAddressType},
		{APRD, Positional},
		{APWR, Positional},
		{FPRD, Fixed},
		{FPWR, Fixed},
		{BRD, Broadcast},
		{BWR, Broadcast},
		{LRD, Logical},
		{LWR, Logical},
	}

	for _, tc := range testCases {
		da := DatagramAddressFromCommand(0x12345678, tc.ct)
		if da.Type() != tc.want {
			t.Errorf("Command %v got type %v, want %v", tc.ct, da.Type(), tc.want)
		}
		if da.Addr32() != 0x12345678 {
			t.Errorf("Command %v got addr 0x%08x, want 0x12345678", tc.ct, da.Addr32())
		}
	}
}

// ---------------------------------------------------------------------------
// Test DatagramHeader overlay/commit roundtrip
// ---------------------------------------------------------------------------

func TestDatagramHeader_TooShort(t *testing.T) {
	buf := make([]byte, 9)
	dh := DatagramHeader{}
	_, err := dh.Overlay(buf)
	if err == nil {
		t.Fatal("expected error for 9-byte buffer, got none")
	}
}

func TestDatagramHeader_Roundtrip(t *testing.T) {
	buf := make([]byte, 10)

	dh := DatagramHeader{
		Command:   APRW,
		Index:     0x12,
		Addr32:    0x12345678,
		LenWord:   0x123 | (1 << 15), // not last
		Interrupt: 0x5678,
	}
	dh.buffer = buf

	_, err := dh.Commit()
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	var dh2 DatagramHeader
	rem, err := dh2.Overlay(buf)
	if err != nil {
		t.Fatalf("Overlay failed: %v", err)
	}
	if len(rem) != 0 {
		t.Errorf("expected 0 remaining bytes, got %d", len(rem))
	}

	if dh2.Command != dh.Command {
		t.Errorf("Command: got %v, want %v", dh2.Command, dh.Command)
	}
	if dh2.Index != dh.Index {
		t.Errorf("Index: got 0x%02x, want 0x%02x", dh2.Index, dh.Index)
	}
	if dh2.Addr32 != dh.Addr32 {
		t.Errorf("Addr32: got 0x%08x, want 0x%08x", dh2.Addr32, dh.Addr32)
	}
	if dh2.LenWord != dh.LenWord {
		t.Errorf("LenWord: got 0x%04x, want 0x%04x", dh2.LenWord, dh.LenWord)
	}
	if dh2.Interrupt != dh.Interrupt {
		t.Errorf("Interrupt: got 0x%04x, want 0x%04x", dh2.Interrupt, dh.Interrupt)
	}

	if dh2.DataLength() != 0x123 {
		t.Errorf("DataLength: got %d, want 0x123", dh2.DataLength())
	}

	if dh2.Last() {
		t.Error("expected !Last (bit 15 set), got Last")
	}

	dh2.SetLast(true)
	if !dh2.Last() {
		t.Error("SetLast(true) but still !Last")
	}
	if (dh2.LenWord & (1 << 15)) != 0 {
		t.Error("bit 15 should be clear after SetLast(true)")
	}

	dh2.SetLast(false)
	if dh2.Last() {
		t.Error("SetLast(false) but still Last")
	}
	if (dh2.LenWord & (1 << 15)) == 0 {
		t.Error("bit 15 should be set after SetLast(false)")
	}
}

// ---------------------------------------------------------------------------
// Test Datagram overlay/commit roundtrip
// ---------------------------------------------------------------------------

func TestDatagramOverlay_TooShortHeader(t *testing.T) {
	buf := make([]byte, 11)
	dg := Datagram{}
	_, err := dg.Overlay(buf)
	if err == nil {
		t.Fatal("expected error for header too short, got none")
	}
}

func TestDatagramOverlay_TooShortData(t *testing.T) {
	buf := make([]byte, 12)
	// write header saying we have 4 bytes of data (10+4+2=16 needed), but we only have 2 bytes
	buf[0] = byte(APRD)
	// bytes 1-5: Index, Addr32 (zero)
	// LenWord at bytes 6-7: data length 4
	buf[6] = 4
	buf[7] = 0

	dg := Datagram{}
	_, err := dg.Overlay(buf)
	if err == nil {
		t.Fatal("expected error for data too short, got none")
	}
}

func TestDatagram_Roundtrip(t *testing.T) {
	buf := make([]byte, 10+4+2)

	dg, err := PointDatagramTo(buf)
	if err != nil {
		t.Fatalf("PointDatagramTo failed: %v", err)
	}

	dg.Header.Command = APRD
	dg.Header.Index = 0x01
	dg.Header.Addr32 = 0x00000100
	dg.Header.Interrupt = 0

	err = dg.SetDataLen(4)
	if err != nil {
		t.Fatalf("SetDataLen(4) failed: %v", err)
	}
	copy(dg.Data, []byte{0x11, 0x22, 0x33, 0x44})
	dg.WKC = 0x0001

	committed, err := dg.Commit()
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	if len(committed) != 16 {
		t.Errorf("committed length %d, want 16", len(committed))
	}

	var dg2 Datagram
	rem, err := dg2.Overlay(buf)
	if err != nil {
		t.Fatalf("Overlay failed: %v", err)
	}
	if len(rem) != 0 {
		t.Errorf("%d remaining bytes, want 0", len(rem))
	}

	if dg2.Header.Command != dg.Header.Command {
		t.Errorf("Command mismatch: got %v, want %v", dg2.Header.Command, dg.Header.Command)
	}
	if dg2.Header.DataLength() != 4 {
		t.Errorf("DataLength %d, want 4", dg2.Header.DataLength())
	}
	if len(dg2.Data) != 4 {
		t.Errorf("len(dg2.Data) = %d, want 4", len(dg2.Data))
	}
	if !bytes.Equal(dg2.Data, dg.Data) {
		t.Errorf("Data mismatch: got %v, want %v", dg2.Data, dg.Data)
	}
	if dg2.WKC != dg.WKC {
		t.Errorf("WKC mismatch: got 0x%04x, want 0x%04x", dg2.WKC, dg.WKC)
	}
}

// Test the bug fix: commit error from header should be propagated
func TestDatagramCommit_HeaderErrorPropagated(t *testing.T) {
	// Create a datagram with buffer shorter than header (10 bytes)
	buf := make([]byte, 5)
	dg := Datagram{
		buffer: buf,
		Header: DatagramHeader{buffer: buf},
	}

	_, err := dg.Commit()
	if err == nil {
		t.Fatal("expected error from Header.Commit() (or during datagram commit), got none")
	}
}

func TestDatagramSetDataLen(t *testing.T) {
	buf := make([]byte, 12+100)
	dg, err := PointDatagramTo(buf)
	if err != nil {
		t.Fatalf("PointDatagramTo failed: %v", err)
	}

	err = dg.SetDataLen(32)
	if err != nil {
		t.Fatalf("SetDataLen(32) failed: %v", err)
	}
	if len(dg.Data) != 32 {
		t.Errorf("len(dg.Data) = %d, want 32", len(dg.Data))
	}
	if dg.ByteLen() != 12+32 {
		t.Errorf("ByteLen = %d, want %d", dg.ByteLen(), 12+32)
	}
	if int(dg.Header.DataLength()) != 32 {
		t.Errorf("Header.DataLength() = %d, want 32", dg.Header.DataLength())
	}

	// Try to exceed maximum length
	err = dg.SetDataLen(5000)
	if err == nil {
		t.Error("expected error for length exceeding max datagram length, got none")
	}
}

// ---------------------------------------------------------------------------
// Test Header (frame header)
// ---------------------------------------------------------------------------

func TestHeader(t *testing.T) {
	buf := make([]byte, 2)
	h := Header{Word: (0x123 << 0) | (0x5 << 12)}
	h.buffer = buf

	_, err := h.Commit()
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	var h2 Header
	rem, err := h2.Overlay(buf)
	if err != nil {
		t.Fatalf("Overlay failed: %v", err)
	}
	if len(rem) != 0 {
		t.Errorf("%d remaining bytes, want 0", len(rem))
	}

	if h2.FrameLength() != 0x123 {
		t.Errorf("FrameLength = 0x%03x, want 0x123", h2.FrameLength())
	}
	if h2.Type() != 0x5 {
		t.Errorf("Type = 0x%x, want 0x5", h2.Type())
	}

	h2.SetType(0xa)
	if h2.Type() != 0xa {
		t.Errorf("after SetType(0xa): got 0x%x, want 0xa", h2.Type())
	}
}

func TestHeader_TooShort(t *testing.T) {
	h := Header{}
	_, err := h.Overlay(make([]byte, 1))
	if err == nil {
		t.Error("expected error for 1-byte buffer, got none")
	}
}

// ---------------------------------------------------------------------------
// Test Frame overlay/commit/newdatagram
// ---------------------------------------------------------------------------

func TestFrameNewDatagram(t *testing.T) {
	buf := make([]byte, 200)
	f, err := PointFrameTo(buf)
	if err != nil {
		t.Fatalf("PointFrameTo failed: %v", err)
	}

	dg, err := f.NewDatagram(32)
	if err != nil {
		t.Fatalf("NewDatagram(32) failed: %v", err)
	}
	if len(f.Datagrams) != 1 {
		t.Errorf("len(f.Datagrams) = %d, want 1", len(f.Datagrams))
	}
	if dg == nil {
		t.Fatal("dg is nil")
	}
	if len(dg.Data) != 32 {
		t.Errorf("len(dg.Data) = %d, want 32", len(dg.Data))
	}

	totalLen := f.ByteLen()
	if totalLen != 2+(12+32) {
		t.Errorf("ByteLen = %d, want %d", totalLen, 2+12+32)
	}

	// BUG FIX: previous code panicked when out of space. Now we just get an error.
	_, err = f.NewDatagram(200)
	if err == nil {
		t.Error("expected error when datalen exceeds available space (BUG FIX check), got none")
	}
}

func TestFrameEmpty(t *testing.T) {
	buf := make([]byte, 200)
	f, err := PointFrameTo(buf)
	if err != nil {
		t.Fatalf("PointFrameTo failed: %v", err)
	}

	_, err = f.Commit()
	if err == nil {
		t.Error("expected error when frame has no datagrams, got none")
	}
}

func TestFrameRoundtripOneDatagram(t *testing.T) {
	buf := make([]byte, 512)

	f, err := PointFrameTo(buf)
	if err != nil {
		t.Fatalf("PointFrameTo failed: %v", err)
	}

	dg, err := f.NewDatagram(16)
	if err != nil {
		t.Fatalf("NewDatagram failed: %v", err)
	}
	dg.Header.Command = LWR
	dg.Header.Index = 0x55
	dg.Header.Addr32 = 0x12345678
	for i := range dg.Data {
		dg.Data[i] = byte(i)
	}
	dg.WKC = 0
	dg.Header.SetLast(true)

	committed, err := f.Commit()
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Decode again
	var f2 Frame
	_, err = f2.Overlay(committed)
	if err != nil {
		t.Fatalf("Overlay failed: %v", err)
	}

	if len(f2.Datagrams) != 1 {
		t.Fatalf("got %d datagrams, want 1", len(f2.Datagrams))
	}
	dg2 := f2.Datagrams[0]
	if dg2.Header.Command != LWR {
		t.Errorf("command %v, want LWR", dg2.Header.Command)
	}
	if dg2.Header.Index != 0x55 {
		t.Errorf("index 0x%02x, want 0x55", dg2.Header.Index)
	}
	if len(dg2.Data) != 16 {
		t.Errorf("data length %d, want 16", len(dg2.Data))
	}
	if !dg2.Header.Last() {
		t.Error("expected last datagram to have Last()=true")
	}
}

// ---------------------------------------------------------------------------
// Test Ethernet frame (eth.go)
// ---------------------------------------------------------------------------

func TestETHFrameDecoding(t *testing.T) {
	buf := make([]byte, 64) // min is 64
	hdr := []byte{
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, // dest
		0x11, 0x12, 0x13, 0x14, 0x15, 0x16, // src
		0x88, 0xa4, // EtherCAT type 0x88a4 big-endian
	}
	copy(buf, hdr)

	ef, err := OverlayETHFrame(buf)
	if err != nil {
		t.Fatalf("OverlayETHFrame failed: %v", err)
	}

	wantDest := ETHAddr{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	wantSrc := ETHAddr{0x11, 0x12, 0x13, 0x14, 0x15, 0x16}

	if ef.Destination != wantDest {
		t.Errorf("destination %v, want %v", ef.Destination, wantDest)
	}
	if ef.Source != wantSrc {
		t.Errorf("source %v, want %v", ef.Source, wantSrc)
	}
	if ef.Type != 0x88a4 {
		t.Errorf("type 0x%04x, want 0x88a4", ef.Type)
	}
	if ef.GetHeaderLen() != 14 {
		t.Errorf("header len %d, want 14", ef.GetHeaderLen())
	}

	payload := ef.GetPayload()
	if len(payload) != 64-14-4 {
		t.Errorf("payload length %d, want %d", len(payload), 64-14-4)
	}
}

func TestETHFrameTooSmall(t *testing.T) {
	buf := make([]byte, 40)
	_, err := OverlayETHFrame(buf)
	if err == nil {
		t.Error("expected error for buffer smaller than 64 bytes, got none")
	}
}

// BUG FIX check: WriteDown correctly writes two bytes to pos and pos+1
func TestETHFrameWriteDownBugFix(t *testing.T) {
	buf := make([]byte, 64)
	ef, err := OverlayETHFrame(buf)
	if err != nil {
		t.Fatalf("OverlayETHFrame failed: %v", err)
	}

	ef.Destination = ETHAddr{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	ef.Source = ETHAddr{0x11, 0x12, 0x13, 0x14, 0x15, 0x16}
	ef.Type = 0x88a4

	err = ef.WriteDown()
	if err != nil {
		t.Fatalf("WriteDown failed: %v", err)
	}

	// Check that both bytes were written correctly to positions 12 and 13
	if buf[12] != 0x88 {
		t.Errorf("buf[12] = 0x%02x, want 0x88 (high byte of 0x88a4)", buf[12])
	}
	if buf[13] != 0xa4 {
		t.Errorf("buf[13] = 0x%02x, want 0xa4 (low byte of 0x88a4)", buf[13])
	}

	// BUG was both bytes written to pos (12), so let's check pos+1 wasn't overwritten wrong
	// Original bug: both 0x88 and 0xa4 would be written to pos 12, pos 13 remains zero
	// Now it's fixed, so we expect 0xa4 at 13.
}

func TestETHFrameSetPayloadLen(t *testing.T) {
	buf := make([]byte, 1522) // max without VLAN
	ef, err := OverlayETHFrame(buf)
	if err != nil {
		t.Fatalf("Overlay failed: %v", err)
	}

	err = ef.SetPayloadLen(1500)
	if err != nil {
		t.Fatalf("SetPayloadLen(1500) failed: %v", err)
	}
	if len(ef.GetPayload()) != 1500 {
		t.Errorf("got payload len %d, want 1500", len(ef.GetPayload()))
	}

	// Too small
	err = ef.SetPayloadLen(40)
	if err == nil {
		t.Error("expected error for payload too small, got none")
	}

	// Too big for max no VLAN
	err = ef.SetPayloadLen(1505)
	if err == nil {
		t.Error("expected error for payload exceeding maximum, got none")
	}
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkDatagramOverlay(b *testing.B) {
	buf := make([]byte, 10+32+2)
	// Set up header with 32 bytes data
	buf[0] = byte(APRD)
	buf[6] = 32 // LenWord low byte = data length
	buf[7] = 0

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var dg Datagram
		dg.Overlay(buf)
	}
}

func BenchmarkDatagramCommit(b *testing.B) {
	buf := make([]byte, 10+32+2)
	dg, _ := PointDatagramTo(buf)
	dg.Header.Command = APRD
	_ = dg.SetDataLen(32)
	dg.WKC = 1

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		dg.Commit()
	}
}

func BenchmarkFrameOverlay(b *testing.B) {
	buf := make([]byte, 2+10+32+2)
	// Frame length = 44
	buf[0] = 44 - 2
	buf[1] = 0
	// datagram header: LenWord at offset 6-7 from start of datagram
	buf[2+0] = byte(APRD)
	buf[2+6] = 32 // LenWord low byte = data length
	buf[2+7] = 0

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var f Frame
		f.Overlay(buf)
	}
}

func BenchmarkFrameNewDatagram(b *testing.B) {
	buf := make([]byte, 1500)
	f, _ := PointFrameTo(buf)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		f.Datagrams = nil
		f.NewDatagram(32)
	}
}

func BenchmarkETHFrameWriteDown(b *testing.B) {
	buf := make([]byte, 64)
	ef, _ := OverlayETHFrame(buf)
	ef.Destination = ETHAddr{}
	ef.Source = ETHAddr{}
	ef.Type = 0x88a4

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ef.WriteDown()
	}
}

// ---------------------------------------------------------------------------
// 全链路性能基准测试
// ---------------------------------------------------------------------------

// BenchmarkFullPipeline 完整 encode -> decode -> verify 循环
func BenchmarkFullPipeline(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		buf := make([]byte, 2+10+2047+2)
		f, _ := PointFrameTo(buf)
		dg, _ := f.NewDatagram(2047)
		dg.Header.Command = LWR
		dg.Header.Addr32 = 0x12345678
		dg.Header.Index = 0x55
		for j := range dg.Data {
			dg.Data[j] = byte(j & 0xFF)
		}
		dg.WKC = 0x0011
		dg.Header.SetLast(true)
		b.StartTimer()

		// Encode
		committed, err := f.Commit()
		if err != nil {
			b.Fatalf("Commit failed: %v", err)
		}

		// Decode
		var f2 Frame
		_, err = f2.Overlay(committed)
		if err != nil {
			b.Fatalf("Overlay failed: %v", err)
		}

		// Verify
		if len(f2.Datagrams) != 1 {
			b.Fatal("wrong datagram count")
		}
		dg2 := f2.Datagrams[0]
		if dg2.Header.Command != LWR {
			b.Fatal("command mismatch")
		}
		if dg2.Header.DataLength() != 2047 {
			b.Fatal("data length mismatch")
		}
		if dg2.WKC != 0x0011 {
			b.Fatal("WKC mismatch")
		}
	}
}

// BenchmarkFrameOverlayMultiDatagram 多数据报帧 overlay
func BenchmarkFrameOverlayMultiDatagram(b *testing.B) {
	b.ReportAllocs()

	// 构造一个包含 3 个数据报的帧
	const nDatagrams = 3
	const dataLen = 32

	b.StopTimer()
	buf := make([]byte, 2+nDatagrams*(10+dataLen+2))
	f, _ := PointFrameTo(buf)
	for k := 0; k < nDatagrams; k++ {
		dg, _ := f.NewDatagram(dataLen)
		dg.Header.Command = APRD
		dg.Header.Addr32 = uint32(k * 0x1000)
		dg.WKC = uint16(k)
		for j := range dg.Data {
			dg.Data[j] = byte((k << 4) | (j & 0x0F))
		}
		dg.Header.SetLast(k == nDatagrams-1)
	}
	committed, _ := f.Commit()
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		var f2 Frame
		_, err := f2.Overlay(committed)
		if err != nil {
			b.Fatalf("Overlay failed: %v", err)
		}
		if len(f2.Datagrams) != nDatagrams {
			b.Fatalf("got %d datagrams, want %d", len(f2.Datagrams), nDatagrams)
		}
	}
}

// BenchmarkDatagramOverlayMaxData 最大数据长度(2047 字节) overlay
func BenchmarkDatagramOverlayMaxData(b *testing.B) {
	const maxData = 2047

	b.ReportAllocs()

	b.StopTimer()
	buf := make([]byte, 10+maxData+2)
	buf[0] = byte(APRD)
	buf[6] = byte(maxData & 0xFF)
	buf[7] = byte((maxData >> 8) & 0xFF)
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		var dg Datagram
		_, err := dg.Overlay(buf)
		if err != nil {
			b.Fatalf("Overlay failed: %v", err)
		}
		if len(dg.Data) != maxData {
			b.Fatalf("got data len %d, want %d", len(dg.Data), maxData)
		}
	}
}

// BenchmarkDatagramBoundary 边界条件测试：反复构造并解析长度刚好为 0 的 datagram
func BenchmarkDatagramBoundary(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		buf := make([]byte, 10+0+2)
		dg, _ := PointDatagramTo(buf)
		dg.Header.Command = NOP
		_ = dg.SetDataLen(0)
		dg.Header.SetLast(true)
		committed, _ := dg.Commit()
		b.StartTimer()

		var dg2 Datagram
		_, err := dg2.Overlay(committed)
		if err != nil {
			b.Fatalf("Overlay failed: %v", err)
		}
		if dg2.Header.DataLength() != 0 {
			b.Fatalf("got data len %d, want 0", dg2.Header.DataLength())
		}
	}
}

// ---------------------------------------------------------------------------
// 边界测试
// ---------------------------------------------------------------------------

// TestDatagramMaxDataLength 最大数据长度 2047
func TestDatagramMaxDataLength(t *testing.T) {
	buf := make([]byte, 10+2047+2)

	dg, err := PointDatagramTo(buf)
	if err != nil {
		t.Fatalf("PointDatagramTo failed: %v", err)
	}

	err = dg.SetDataLen(2047)
	if err != nil {
		t.Fatalf("SetDataLen(2047) failed: %v", err)
	}

	if len(dg.Data) != 2047 {
		t.Errorf("len(dg.Data) = %d, want 2047", len(dg.Data))
	}
	if dg.ByteLen() != 12+2047 {
		t.Errorf("ByteLen = %d, want %d", dg.ByteLen(), 12+2047)
	}
	if int(dg.Header.DataLength()) != 2047 {
		t.Errorf("Header.DataLength() = %d, want 2047", dg.Header.DataLength())
	}

	// 写入数据并验证 Commit + Overlay 往返
	dg.Header.Command = FPRW
	dg.Header.Addr32 = 0xDEADBEEF
	dg.Header.Index = 0xAA
	dg.Header.Interrupt = 1
	for j := range dg.Data {
		dg.Data[j] = byte(j & 0xFF)
	}
	dg.WKC = 0xABCD
	dg.Header.SetLast(true)

	committed, err := dg.Commit()
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	if len(committed) != 10+2047+2 {
		t.Errorf("committed length %d, want %d", len(committed), 10+2047+2)
	}

	var dg2 Datagram
	rem, err := dg2.Overlay(committed)
	if err != nil {
		t.Fatalf("Overlay failed: %v", err)
	}
	if len(rem) != 0 {
		t.Errorf("%d remaining bytes, want 0", len(rem))
	}
	if dg2.Header.Command != FPRW {
		t.Errorf("Command = %v, want FPRW", dg2.Header.Command)
	}
	if dg2.Header.DataLength() != 2047 {
		t.Errorf("DataLength = %d, want 2047", dg2.Header.DataLength())
	}
	if dg2.WKC != 0xABCD {
		t.Errorf("WKC = 0x%04x, want 0xABCD", dg2.WKC)
	}
	if !bytes.Equal(dg2.Data, dg.Data) {
		t.Errorf("Data mismatch: first byte got 0x%02x, want 0x%02x", dg2.Data[0], dg.Data[0])
	}
	if dg2.Header.Addr32 != 0xDEADBEEF {
		t.Errorf("Addr32 = 0x%08x, want 0xDEADBEEF", dg2.Header.Addr32)
	}
	if dg2.Header.Index != 0xAA {
		t.Errorf("Index = 0x%02x, want 0xAA", dg2.Header.Index)
	}
	if !dg2.Header.Last() {
		t.Error("expected Last() = true")
	}
}

// TestDatagramZeroDataLength 零数据长度
func TestDatagramZeroDataLength(t *testing.T) {
	buf := make([]byte, 10+0+2)

	dg, err := PointDatagramTo(buf)
	if err != nil {
		t.Fatalf("PointDatagramTo failed: %v", err)
	}

	err = dg.SetDataLen(0)
	if err != nil {
		t.Fatalf("SetDataLen(0) failed: %v", err)
	}

	if len(dg.Data) != 0 {
		t.Errorf("len(dg.Data) = %d, want 0", len(dg.Data))
	}
	if dg.ByteLen() != 12 {
		t.Errorf("ByteLen = %d, want 12", dg.ByteLen())
	}
	if int(dg.Header.DataLength()) != 0 {
		t.Errorf("Header.DataLength() = %d, want 0", dg.Header.DataLength())
	}

	// Commit + Overlay 往返
	dg.Header.Command = BRD
	dg.Header.Addr32 = 0x00000000
	dg.WKC = 0x0001
	dg.Header.SetLast(true)

	committed, err := dg.Commit()
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	if len(committed) != 12 {
		t.Errorf("committed length %d, want 12", len(committed))
	}

	var dg2 Datagram
	rem, err := dg2.Overlay(committed)
	if err != nil {
		t.Fatalf("Overlay failed: %v", err)
	}
	if len(rem) != 0 {
		t.Errorf("%d remaining bytes, want 0", len(rem))
	}
	if dg2.Header.Command != BRD {
		t.Errorf("Command = %v, want BRD", dg2.Header.Command)
	}
	if dg2.Header.DataLength() != 0 {
		t.Errorf("DataLength = %d, want 0", dg2.Header.DataLength())
	}
	if dg2.WKC != 0x0001 {
		t.Errorf("WKC = 0x%04x, want 0x0001", dg2.WKC)
	}
}

// TestFrameFullCapacity 帧容量边界
func TestFrameFullCapacity(t *testing.T) {
	// 使用一个较小的缓冲区，测试刚好填满的情况
	const bufSize = 100
	buf := make([]byte, bufSize)

	f, err := PointFrameTo(buf)
	if err != nil {
		t.Fatalf("PointFrameTo failed: %v", err)
	}

	// 添加一个数据报，恰好填满缓冲区
	dg, err := f.NewDatagram(bufSize - 2 - 12) // 2 = frame header, 12 = datagram overhead
	if err != nil {
		t.Fatalf("NewDatagram failed: %v", err)
	}
	dg.Header.Command = APRD
	dg.Header.Addr32 = 0x00001000
	dg.WKC = 0
	dg.Header.SetLast(true)
	for j := range dg.Data {
		dg.Data[j] = byte(j)
	}

	expectedTotal := bufSize
	if f.ByteLen() != expectedTotal {
		t.Errorf("ByteLen = %d, want %d", f.ByteLen(), expectedTotal)
	}

	committed, err := f.Commit()
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	if len(committed) != expectedTotal {
		t.Errorf("committed length %d, want %d", len(committed), expectedTotal)
	}

	// 尝试再添加一个数据报，应该失败
	_, err = f.NewDatagram(1)
	if err == nil {
		t.Error("expected error when adding datagram to full frame, got none")
	}

	// 解码并验证
	var f2 Frame
	_, err = f2.Overlay(committed)
	if err != nil {
		t.Fatalf("Overlay failed: %v", err)
	}
	if len(f2.Datagrams) != 1 {
		t.Fatalf("got %d datagrams, want 1", len(f2.Datagrams))
	}
	if f2.ByteLen() != expectedTotal {
		t.Errorf("decoded ByteLen = %d, want %d", f2.ByteLen(), expectedTotal)
	}
}

// TestByteLenCache 验证 ByteLen 缓存一致性
func TestByteLenCache(t *testing.T) {
	// 测试 Frame.ByteLen() 缓存与逐项计算一致
	buf := make([]byte, 512)

	f, err := PointFrameTo(buf)
	if err != nil {
		t.Fatalf("PointFrameTo failed: %v", err)
	}

	// 初始 ByteLen 应该等于 FrameOverheadLen
	if f.ByteLen() != FrameOverheadLen {
		t.Errorf("initial ByteLen = %d, want %d", f.ByteLen(), FrameOverheadLen)
	}

	// 添加多个不同长度的数据报，验证缓存一致性
	dataLens := []int{16, 32, 8, 64}
	var expectedTotal = FrameOverheadLen

	for _, dl := range dataLens {
		dg, err := f.NewDatagram(dl)
		if err != nil {
			t.Fatalf("NewDatagram(%d) failed: %v", dl, err)
		}
		dg.Header.Command = APRD
		dg.Header.Addr32 = uint32(dl * 0x1000)
		dg.WKC = uint16(dl)
		for j := range dg.Data {
			dg.Data[j] = byte(j & 0xFF)
		}
		expectedTotal += dl + DatagramOverheadLength
	}

	// 设置 Last 标志：只有最后一个数据报是 Last，前面的都标记为非 Last
	for i := 0; i < len(f.Datagrams)-1; i++ {
		f.Datagrams[i].Header.SetLast(false)
	}
	if len(f.Datagrams) > 0 {
		f.Datagrams[len(f.Datagrams)-1].Header.SetLast(true)
	}

	// 检查 Frame.ByteLen() 缓存
	if f.ByteLen() != expectedTotal {
		t.Errorf("ByteLen cache = %d, want %d", f.ByteLen(), expectedTotal)
	}

	// Commit 后重新 Overlay，验证解码后的 ByteLen
	committed, err := f.Commit()
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	var f2 Frame
	_, err = f2.Overlay(committed)
	if err != nil {
		t.Fatalf("Overlay failed: %v", err)
	}

	// 解码后的 ByteLen 应该等于提交长度
	if f2.ByteLen() != len(committed) {
		t.Errorf("decoded ByteLen = %d, want %d", f2.ByteLen(), len(committed))
	}
	if f2.ByteLen() != expectedTotal {
		t.Errorf("decoded ByteLen = %d, want expected %d", f2.ByteLen(), expectedTotal)
	}

	// 验证数据报数量一致
	if len(f2.Datagrams) != len(dataLens) {
		t.Fatalf("got %d datagrams, want %d", len(f2.Datagrams), len(dataLens))
	}
}
