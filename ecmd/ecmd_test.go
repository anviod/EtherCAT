package ecmd

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/anviod/EtherCAT/ecfr"
)

// ─── Mock Implementations ─────────────────────────────────────────────────────

// mockCommander implements Commander for testing.
type mockCommander struct {
	mu       sync.Mutex
	commands []*mockCmd
	newErr   error
	cycleErr error
	// If set, Cycle() will set this working counter on all response datagrams.
	wc uint16
	// If set, Cycle() will set Arrived/Overlayed to false to simulate frame loss.
	noArrive bool
	// If set, Cycle() will set Overlayed to false.
	noOverlay bool
}

type mockCmd struct {
	ec      *ExecutingCommand
	datalen int
}

func (m *mockCommander) New(datalen int) (*ExecutingCommand, error) {
	if m.newErr != nil {
		return nil, m.newErr
	}

	buf := make([]byte, datalen+ecfr.DatagramOverheadLength)
	dg, err := ecfr.PointDatagramTo(buf)
	if err != nil {
		return nil, err
	}
	if err := dg.SetDataLen(datalen); err != nil {
		return nil, err
	}

	ec := &ExecutingCommand{
		DatagramOut: &dg,
	}

	m.mu.Lock()
	m.commands = append(m.commands, &mockCmd{ec: ec, datalen: datalen})
	m.mu.Unlock()

	return ec, nil
}

func (m *mockCommander) Cycle() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, cmd := range m.commands {
		if cmd.ec.Arrived {
			continue // already set (e.g., from previous cycle in retry loop)
		}

		if m.noArrive {
			cmd.ec.Arrived = false
			cmd.ec.Overlayed = false
			continue
		}

		dgo := cmd.ec.DatagramOut
		rsBuf := make([]byte, dgo.ByteLen())
		rsDg, err := ecfr.PointDatagramTo(rsBuf)
		if err != nil {
			return err
		}
		if err := rsDg.SetDataLen(int(dgo.Header.DataLength())); err != nil {
			return err
		}
		rsDg.Header.Command = dgo.Header.Command
		rsDg.Header.Addr32 = dgo.Header.Addr32
		rsDg.Header.Index = dgo.Header.Index
		copy(rsDg.Data, dgo.Data)

		wc := m.wc
		if wc == 0 {
			wc = 1
		}
		rsDg.WKC = wc

		cmd.ec.DatagramIn = &rsDg
		cmd.ec.Arrived = true
		if m.noOverlay {
			cmd.ec.Overlayed = false
		} else {
			cmd.ec.Overlayed = true
		}
	}

	return m.cycleErr
}

func (m *mockCommander) Close() error {
	return nil
}

func (m *mockCommander) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commands = nil
}

// mockFramer implements Framer for testing CommandFramer.
type mockFramer struct {
	mu       sync.Mutex
	frames   []*ecfr.Frame
	cycled   bool
	newErr   error
	cycleErr error
}

func (f *mockFramer) New(maxdatalen int) (*ecfr.Frame, error) {
	if f.newErr != nil {
		return nil, f.newErr
	}

	b := make([]byte, maxdatalen+ecfr.FrameOverheadLen)
	frame, err := ecfr.PointFrameTo(b)
	if err != nil {
		return nil, err
	}

	f.mu.Lock()
	f.frames = append(f.frames, &frame)
	f.mu.Unlock()

	return &frame, nil
}

func (f *mockFramer) Cycle() ([]*ecfr.Frame, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.cycled {
		return nil, errors.New("mockFramer already cycled")
	}
	f.cycled = true

	if f.cycleErr != nil {
		return nil, f.cycleErr
	}

	return f.frames, nil
}

// makeLenDgram creates a datagram with the given payload length, index, and last flag.
func makeLenDgram(plen int, index uint8, last bool) *ecfr.Datagram {
	ub := make([]byte, plen+ecfr.DatagramOverheadLength)
	dgram, err := ecfr.PointDatagramTo(ub)
	if err != nil {
		panic(err)
	}
	if err := dgram.SetDataLen(plen); err != nil {
		panic(err)
	}

	dgram.Header.Index = index
	dgram.Header.SetLast(last)

	return &dgram
}

// makeAddr creates a positional DatagramAddress.
func makeAddr(position uint16, offset uint16) ecfr.DatagramAddress {
	return ecfr.PositionalAddr(int16(position), offset)
}

// ─── WorkingCounterError Tests ────────────────────────────────────────────────

func TestWorkingCounterError(t *testing.T) {
	wce := WorkingCounterError{
		Command: ecfr.APRD,
		Addr32:  0x12345678,
		Want:    3,
		Have:    1,
	}

	errStr := wce.Error()
	if errStr == "" {
		t.Error("WorkingCounterError.Error() returned empty string")
	}

	// Verify it implements the error interface.
	var _ error = wce
}

func TestWorkingCounterErrorString(t *testing.T) {
	wce := WorkingCounterError{
		Command: ecfr.APRD,
		Addr32:  0x00000010,
		Want:    2,
		Have:    1,
	}
	expected := "working counter error, want 2, have 1 on APRD 0x00000010"
	if wce.Error() != expected {
		t.Errorf("expected %q, got %q", expected, wce.Error())
	}
}

// ─── Sentinel Error Tests ─────────────────────────────────────────────────────

func TestNoFrame(t *testing.T) {
	if NoFrame.Error() != "frame did not arrive" {
		t.Errorf("unexpected NoFrame message: %s", NoFrame.Error())
	}
}

func TestNoOverlay(t *testing.T) {
	if NoOverlay.Error() != "failed to overlay" {
		t.Errorf("unexpected NoOverlay message: %s", NoOverlay.Error())
	}
}

func TestIsNoFrame(t *testing.T) {
	if !IsNoFrame(NoFrame) {
		t.Error("IsNoFrame(NoFrame) should be true")
	}
	if IsNoFrame(errors.New("frame did not arrive")) {
		t.Error("IsNoFrame on a different error with same message should be false")
	}
	if IsNoFrame(NoOverlay) {
		t.Error("IsNoFrame(NoOverlay) should be false")
	}
	if IsNoFrame(nil) {
		t.Error("IsNoFrame(nil) should be false")
	}
}

func TestIsWorkingCounterError(t *testing.T) {
	if !IsWorkingCounterError(WorkingCounterError{}) {
		t.Error("IsWorkingCounterError on WorkingCounterError should be true")
	}
	if IsWorkingCounterError(errors.New("some error")) {
		t.Error("IsWorkingCounterError on regular error should be false")
	}
	if IsWorkingCounterError(nil) {
		t.Error("IsWorkingCounterError(nil) should be false")
	}
}

// ─── ChooseDefaultError Tests ─────────────────────────────────────────────────

func TestChooseDefaultError_NoFrame(t *testing.T) {
	ec := &ExecutingCommand{
		Arrived:   false,
		Overlayed: false,
	}
	err := ChooseDefaultError(ec)
	if err != NoFrame {
		t.Errorf("expected NoFrame, got %v", err)
	}
}

func TestChooseDefaultError_NoOverlay(t *testing.T) {
	ec := &ExecutingCommand{
		Arrived:   true,
		Overlayed: false,
	}
	err := ChooseDefaultError(ec)
	if err != NoOverlay {
		t.Errorf("expected NoOverlay, got %v", err)
	}
}

func TestChooseDefaultError_CmdError(t *testing.T) {
	customErr := errors.New("custom error")
	ec := &ExecutingCommand{
		Arrived:   true,
		Overlayed: true,
		Error:     customErr,
	}
	err := ChooseDefaultError(ec)
	if err != customErr {
		t.Errorf("expected custom error, got %v", err)
	}
}

func TestChooseDefaultError_NoError(t *testing.T) {
	ec := &ExecutingCommand{
		Arrived:   true,
		Overlayed: true,
	}
	err := ChooseDefaultError(ec)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// ─── ChooseWorkingCounterError Tests ──────────────────────────────────────────

func TestChooseWorkingCounterError_Match(t *testing.T) {
	dgIn := &ecfr.Datagram{}
	dgIn.WKC = 3
	dgOut := &ecfr.Datagram{}
	dgOut.Header.Command = ecfr.APRD

	ec := &ExecutingCommand{
		DatagramOut: dgOut,
		DatagramIn:  dgIn,
	}

	err := ChooseWorkingCounterError(ec, 3)
	if err != nil {
		t.Errorf("expected nil for matching counters, got %v", err)
	}
}

func TestChooseWorkingCounterError_Mismatch(t *testing.T) {
	dgIn := &ecfr.Datagram{}
	dgIn.WKC = 1
	dgOut := &ecfr.Datagram{}
	dgOut.Header.Command = ecfr.APRD
	dgOut.Header.Addr32 = 0x1000

	ec := &ExecutingCommand{
		DatagramOut: dgOut,
		DatagramIn:  dgIn,
	}

	err := ChooseWorkingCounterError(ec, 3)
	if err == nil {
		t.Error("expected error for mismatching counters")
	}
	if !IsWorkingCounterError(err) {
		t.Errorf("expected WorkingCounterError, got %T: %v", err, err)
	}

	wce := err.(WorkingCounterError)
	if wce.Want != 3 || wce.Have != 1 {
		t.Errorf("Want=3 Have=1, got Want=%d Have=%d", wce.Want, wce.Have)
	}
	if wce.Command != ecfr.APRD {
		t.Errorf("expected APRD, got %v", wce.Command)
	}
	if wce.Addr32 != 0x1000 {
		t.Errorf("expected 0x1000, got %#x", wce.Addr32)
	}
}

// ─── ExecuteRead Tests ────────────────────────────────────────────────────────

func TestExecuteRead8(t *testing.T) {
	mc := &mockCommanderWithData{data: []byte{0x42}}
	addr := makeAddr(0, 0x120)

	v, err := ExecuteRead8(mc, addr, 1)
	if err != nil {
		t.Fatalf("ExecuteRead8 failed: %v", err)
	}
	if v != 0x42 {
		t.Errorf("expected 0x42, got 0x%02x", v)
	}
}

func TestExecuteRead16(t *testing.T) {
	mc := &mockCommanderWithData{data: []byte{0x34, 0x12}}
	addr := makeAddr(0, 0x120)

	v, err := ExecuteRead16(mc, addr, 1)
	if err != nil {
		t.Fatalf("ExecuteRead16 failed: %v", err)
	}
	if v != 0x1234 {
		t.Errorf("expected 0x1234, got 0x%04x", v)
	}
}

func TestExecuteRead32(t *testing.T) {
	mc := &mockCommanderWithData{data: []byte{0x78, 0x56, 0x34, 0x12}}
	addr := makeAddr(0, 0x120)

	v, err := ExecuteRead32(mc, addr, 1)
	if err != nil {
		t.Fatalf("ExecuteRead32 failed: %v", err)
	}
	if v != 0x12345678 {
		t.Errorf("expected 0x12345678, got 0x%08x", v)
	}
}

func TestExecuteRead(t *testing.T) {
	mc := &mockCommanderWithData{data: []byte{0x01, 0x02, 0x03, 0x04}}
	addr := makeAddr(0, 0x100)

	data, err := ExecuteRead(mc, addr, 4, 1)
	if err != nil {
		t.Fatalf("ExecuteRead failed: %v", err)
	}
	if len(data) != 4 {
		t.Fatalf("expected 4 bytes, got %d", len(data))
	}
	if data[0] != 0x01 || data[1] != 0x02 || data[2] != 0x03 || data[3] != 0x04 {
		t.Errorf("unexpected data: % x", data)
	}
}

func TestExecuteReadOptions_Retry(t *testing.T) {
	mc := &mockCommanderWithData{data: []byte{0x42}, noArriveCount: 2}
	addr := makeAddr(0, 0x120)

	opts := Options{FramelossTries: 5}
	v, err := ExecuteRead8Options(mc, addr, 1, opts)
	if err != nil {
		t.Fatalf("ExecuteRead8Options failed: %v", err)
	}
	if v != 0x42 {
		t.Errorf("expected 0x42, got 0x%02x", v)
	}
}

func TestExecuteReadOptions_RetryExhausted(t *testing.T) {
	mc := &mockCommanderWithData{data: []byte{0x42}, noArriveCount: 10}
	addr := makeAddr(0, 0x120)

	opts := Options{FramelossTries: 3}
	_, err := ExecuteRead8Options(mc, addr, 1, opts)
	if err == nil {
		t.Error("expected error after exhausting retries")
	}
	if !IsNoFrame(err) {
		t.Errorf("expected NoFrame, got %v", err)
	}
}

func TestExecuteReadOptions_WCDeadline(t *testing.T) {
	// With a short deadline and persistent WC mismatch, the command should
	// retry until the deadline expires, then return the WC error.
	mc := &mockCommanderWithData{data: []byte{0x42}, wc: 2} // wc=2, expected=1
	addr := makeAddr(0, 0x120)

	opts := Options{WCDeadline: time.Now().Add(50 * time.Millisecond)}
	start := time.Now()
	_, err := ExecuteRead8Options(mc, addr, 1, opts)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected working counter error after deadline expiry")
	}
	if !IsWorkingCounterError(err) {
		t.Errorf("expected WorkingCounterError, got %T: %v", err, err)
	}
	// Verify it retried for at least a short duration (deadline forces retry).
	if elapsed < 10*time.Millisecond {
		t.Errorf("expected at least 10ms of retrying, got %v", elapsed)
	}
}

func TestExecuteReadOptions_WCDeadlineExceeded(t *testing.T) {
	mc := &mockCommanderWithData{data: []byte{0x42}, wc: 2} // wc=2, expected=1
	addr := makeAddr(0, 0x120)

	opts := Options{WCDeadline: time.Now().Add(-1 * time.Second)} // deadline in past
	_, err := ExecuteRead8Options(mc, addr, 1, opts)
	if err == nil {
		t.Error("expected working counter error after deadline")
	}
	if !IsWorkingCounterError(err) {
		t.Errorf("expected WorkingCounterError, got %T: %v", err, err)
	}
}

func TestExecuteRead_UnsupportedAddressType(t *testing.T) {
	mc := &mockCommanderWithData{}
	// Logical address type is not supported by ExecuteRead (only Positional, Fixed, Broadcast).
	addr := ecfr.DatagramAddressFromCommand(0x1000, ecfr.LRD)
	_, err := ExecuteRead(mc, addr, 4, 1)
	if err == nil {
		t.Error("expected error for unsupported address type")
	}
}

func TestExecuteRead8_DefaultOptions(t *testing.T) {
	mc := &mockCommanderWithData{data: []byte{0xAA}}
	addr := makeAddr(0, 0x130)

	v, err := ExecuteRead8(mc, addr, 1)
	if err != nil {
		t.Fatalf("ExecuteRead8 failed: %v", err)
	}
	if v != 0xAA {
		t.Errorf("expected 0xAA, got 0x%02x", v)
	}
}

// ─── ExecuteWrite Tests ───────────────────────────────────────────────────────

func TestExecuteWrite8(t *testing.T) {
	mc := &mockCommanderWithData{}
	addr := makeAddr(0, 0x120)

	err := ExecuteWrite8(mc, addr, 0x42, 1)
	if err != nil {
		t.Fatalf("ExecuteWrite8 failed: %v", err)
	}

	// Verify the data was written to the datagram.
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if len(mc.commands) == 0 {
		t.Fatal("no commands recorded")
	}
	cmd := mc.commands[0]
	data := cmd.ec.DatagramOut.Data
	if len(data) != 1 || data[0] != 0x42 {
		t.Errorf("expected [0x42], got % x", data)
	}
}

func TestExecuteWrite16(t *testing.T) {
	mc := &mockCommanderWithData{}
	addr := makeAddr(0, 0x120)

	err := ExecuteWrite16(mc, addr, 0x1234, 1)
	if err != nil {
		t.Fatalf("ExecuteWrite16 failed: %v", err)
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()
	cmd := mc.commands[0]
	data := cmd.ec.DatagramOut.Data
	if len(data) != 2 || data[0] != 0x34 || data[1] != 0x12 {
		t.Errorf("expected [0x34, 0x12], got % x", data)
	}
}

func TestExecuteWrite32(t *testing.T) {
	mc := &mockCommanderWithData{}
	addr := makeAddr(0, 0x120)

	err := ExecuteWrite32(mc, addr, 0x12345678, 1)
	if err != nil {
		t.Fatalf("ExecuteWrite32 failed: %v", err)
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()
	cmd := mc.commands[0]
	data := cmd.ec.DatagramOut.Data
	if len(data) != 4 || data[0] != 0x78 || data[1] != 0x56 || data[2] != 0x34 || data[3] != 0x12 {
		t.Errorf("expected [0x78, 0x56, 0x34, 0x12], got % x", data)
	}
}

func TestExecuteWrite(t *testing.T) {
	mc := &mockCommanderWithData{}
	addr := makeAddr(0, 0x100)

	data := []byte{0x01, 0x02, 0x03, 0x04}
	err := ExecuteWrite(mc, addr, data, 1)
	if err != nil {
		t.Fatalf("ExecuteWrite failed: %v", err)
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()
	cmd := mc.commands[0]
	outData := cmd.ec.DatagramOut.Data
	if len(outData) != 4 {
		t.Fatalf("expected 4 bytes, got %d", len(outData))
	}
	for i, b := range data {
		if outData[i] != b {
			t.Errorf("byte %d: expected 0x%02x, got 0x%02x", i, b, outData[i])
		}
	}
}

func TestExecuteWriteOptions_Retry(t *testing.T) {
	mc := &mockCommanderWithData{noArriveCount: 2}
	addr := makeAddr(0, 0x120)

	opts := Options{FramelossTries: 5}
	err := ExecuteWrite8Options(mc, addr, 0x42, 1, opts)
	if err != nil {
		t.Fatalf("ExecuteWrite8Options failed: %v", err)
	}
}

func TestExecuteWriteOptions_RetryExhausted(t *testing.T) {
	mc := &mockCommanderWithData{noArriveCount: 10}
	addr := makeAddr(0, 0x120)

	opts := Options{FramelossTries: 3}
	err := ExecuteWrite8Options(mc, addr, 0x42, 1, opts)
	if err == nil {
		t.Error("expected error after exhausting retries")
	}
	if !IsNoFrame(err) {
		t.Errorf("expected NoFrame, got %v", err)
	}
}

func TestExecuteWrite_UnsupportedAddressType(t *testing.T) {
	mc := &mockCommanderWithData{}
	addr := ecfr.DatagramAddressFromCommand(0x1000, ecfr.LWR)
	err := ExecuteWrite(mc, addr, []byte{0x01}, 1)
	if err == nil {
		t.Error("expected error for unsupported address type")
	}
}

func TestExecuteWrite_DefaultOptions(t *testing.T) {
	mc := &mockCommanderWithData{}
	addr := makeAddr(0, 0x130)

	err := ExecuteWrite8(mc, addr, 0xBB, 1)
	if err != nil {
		t.Fatalf("ExecuteWrite8 failed: %v", err)
	}
}

// ─── Options Tests ────────────────────────────────────────────────────────────

func TestOptions_DefaultFramelossTries(t *testing.T) {
	opts := Options{}
	if opts.getFramelossTries() != DefaultFramelossTries {
		t.Errorf("expected %d, got %d", DefaultFramelossTries, opts.getFramelossTries())
	}
}

func TestOptions_CustomFramelossTries(t *testing.T) {
	opts := Options{FramelossTries: 5}
	if opts.getFramelossTries() != 5 {
		t.Errorf("expected 5, got %d", opts.getFramelossTries())
	}
}

func TestOptions_WCDeadline(t *testing.T) {
	now := time.Now()
	opts := Options{WCDeadline: now}
	if !opts.getWCDeadline().Equal(now) {
		t.Error("WCDeadline mismatch")
	}
}

// ─── CommandFramer Tests ──────────────────────────────────────────────────────

func TestCommandFramerScheduling(t *testing.T) {
	type expectedDgram struct {
		len   int
		index uint8
		last  bool
	}

	type cfSchedulingPair struct {
		lens   []int
		expect [][]expectedDgram
	}

	pairs := []cfSchedulingPair{
		{
			lens: []int{6},
			expect: [][]expectedDgram{
				{{6, 0, true}},
			},
		},
		{
			lens: []int{22, CommandFramerMaxDatagramsLen - ecfr.DatagramOverheadLength},
			expect: [][]expectedDgram{
				{{22, 0, true}},
				{{CommandFramerMaxDatagramsLen - ecfr.DatagramOverheadLength, 1, true}},
			},
		},
		{
			lens: []int{128, 96},
			expect: [][]expectedDgram{
				{{128, 0, false}, {96, 0, true}},
			},
		},
		{
			lens: []int{140, 65, 1400},
			expect: [][]expectedDgram{
				{{140, 0, false}, {65, 0, true}},
				{{1400, 1, true}},
			},
		},
	}

	for i, pair := range pairs {
		f := &mockFramer{}
		cf := NewCommandFramer(f)

		for _, l := range pair.lens {
			_, err := cf.New(l)
			if err != nil {
				t.Fatalf("case %d: New(%d) failed: %v", i, l, err)
			}
		}

		err := cf.Cycle()
		if err != nil {
			t.Fatalf("case %d: Cycle() failed: %v", i, err)
		}

		if len(f.frames) != len(pair.expect) {
			t.Fatalf("case %d: expected %d frames, got %d", i, len(pair.expect), len(f.frames))
		}

		for j, frame := range f.frames {
			expected := pair.expect[j]
			if len(frame.Datagrams) != len(expected) {
				t.Fatalf("case %d, frame %d: expected %d datagrams, got %d", i, j, len(expected), len(frame.Datagrams))
			}

			for k, dgram := range frame.Datagrams {
				exp := expected[k]
				if dgram.Header.DataLength() != uint16(exp.len) {
					t.Errorf("case %d, frame %d, dgram %d: expected len %d, got %d",
						i, j, k, exp.len, dgram.Header.DataLength())
				}
				if dgram.Header.Index != exp.index {
					t.Errorf("case %d, frame %d, dgram %d: expected index %d, got %d",
						i, j, k, exp.index, dgram.Header.Index)
				}
				if dgram.Header.Last() != exp.last {
					t.Errorf("case %d, frame %d, dgram %d: expected last=%v, got %v",
						i, j, k, exp.last, dgram.Header.Last())
				}
			}
		}
	}
}

func TestCommandFramerDatalenExceedsMax(t *testing.T) {
	f := &mockFramer{}
	cf := NewCommandFramer(f)

	_, err := cf.New(CommandFramerMaxDatagramsLen + 1)
	if err == nil {
		t.Error("expected error for datalen exceeding max")
	}
}

func TestCommandFramerCycleMatching(t *testing.T) {
	f := &mockFramer{}
	cf := NewCommandFramer(f)

	// Create two commands.
	cmd1, err := cf.New(4)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	cmd2, err := cf.New(4)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Set different commands on the outgoing datagrams.
	cmd1.DatagramOut.Header.Command = ecfr.APRD
	cmd1.DatagramOut.Header.Addr32 = 0x1000
	cmd2.DatagramOut.Header.Command = ecfr.APWR
	cmd2.DatagramOut.Header.Addr32 = 0x2000

	// Cycle (mockFramer returns the same frames).
	err = cf.Cycle()
	if err != nil {
		t.Fatalf("Cycle failed: %v", err)
	}

	// Verify commands were matched.
	if !cmd1.Arrived {
		t.Error("cmd1 should have arrived")
	}
	if !cmd1.Overlayed {
		t.Error("cmd1 should be overlaid")
	}
	if !cmd2.Arrived {
		t.Error("cmd2 should have arrived")
	}
	if !cmd2.Overlayed {
		t.Error("cmd2 should be overlaid")
	}

	// Verify the incoming datagrams have the correct command type.
	if cmd1.DatagramIn.Header.Command != ecfr.APRD {
		t.Errorf("cmd1 DatagramIn.Header.Command: expected APRD, got %v", cmd1.DatagramIn.Header.Command)
	}
	if cmd2.DatagramIn.Header.Command != ecfr.APWR {
		t.Errorf("cmd2 DatagramIn.Header.Command: expected APWR, got %v", cmd2.DatagramIn.Header.Command)
	}
}

func TestCommandFramerFrameBoundary(t *testing.T) {
	// Test that datagrams are packed efficiently across frames.
	firstLen := CommandFramerMaxDatagramsLen - ecfr.DatagramOverheadLength - 10 - ecfr.DatagramOverheadLength
	secondLen := 10

	f := &mockFramer{}
	cf := NewCommandFramer(f)

	_, err := cf.New(firstLen)
	if err != nil {
		t.Fatalf("New(%d) failed: %v", firstLen, err)
	}
	_, err = cf.New(secondLen)
	if err != nil {
		t.Fatalf("New(%d) failed: %v", secondLen, err)
	}

	err = cf.Cycle()
	if err != nil {
		t.Fatalf("Cycle failed: %v", err)
	}

	// Both should fit in one frame.
	if len(f.frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(f.frames))
	}
	if len(f.frames[0].Datagrams) != 2 {
		t.Errorf("expected 2 datagrams in frame, got %d", len(f.frames[0].Datagrams))
	}
}

func TestCommandFramerClose(t *testing.T) {
	f := &mockFramer{}
	cf := NewCommandFramer(f)
	err := cf.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

// ─── Multiplexer Tests ────────────────────────────────────────────────────────

func TestMultiplexerBasic(t *testing.T) {
	mc := &mockCommanderWithData{data: []byte{0x42}}
	mux, err := NewMultiplexer(mc)
	if err != nil {
		t.Fatalf("NewMultiplexer failed: %v", err)
	}
	defer mux.Close()

	ch, err := mux.OpenCommander()
	if err != nil {
		t.Fatalf("OpenCommander failed: %v", err)
	}

	addr := makeAddr(0, 0x120)

	// Trigger the mux cycle in a goroutine.
	// The mux.Cycle() will complete when all channels (including ch) have called
	// their Cycle() methods, which happens inside ExecuteRead8.
	var cycleErr error
	done := make(chan struct{})
	go func() {
		cycleErr = mux.Cycle()
		close(done)
	}()

	v, err := ExecuteRead8(ch, addr, 1)
	if err != nil {
		t.Fatalf("ExecuteRead8 on mux channel failed: %v", err)
	}
	<-done
	if cycleErr != nil {
		t.Fatalf("mux Cycle failed: %v", cycleErr)
	}
	if v != 0x42 {
		t.Errorf("expected 0x42, got 0x%02x", v)
	}
}

func TestMultiplexerMultiChannel(t *testing.T) {
	mc := &mockCommanderWithData{data: []byte{0x01, 0x02, 0x03, 0x04}}
	mux, err := NewMultiplexer(mc)
	if err != nil {
		t.Fatalf("NewMultiplexer failed: %v", err)
	}
	defer mux.Close()

	ch1, err := mux.OpenCommander()
	if err != nil {
		t.Fatalf("OpenCommander ch1 failed: %v", err)
	}
	ch2, err := mux.OpenCommander()
	if err != nil {
		t.Fatalf("OpenCommander ch2 failed: %v", err)
	}

	// Step 1: Both channels create commands via New().
	_, err = ch1.New(1)
	if err != nil {
		t.Fatalf("ch1.New failed: %v", err)
	}
	_, err = ch2.New(1)
	if err != nil {
		t.Fatalf("ch2.New failed: %v", err)
	}

	// Step 2: Trigger the mux cycle in a goroutine.
	var cycleErr error
	done := make(chan struct{})
	go func() {
		cycleErr = mux.Cycle()
		close(done)
	}()

	// Step 3: Both channels call Cycle() concurrently.
	var wg2 sync.WaitGroup
	errCh := make(chan error, 2)
	wg2.Add(2)
	go func() {
		defer wg2.Done()
		if e := ch1.Cycle(); e != nil {
			errCh <- e
		}
	}()
	go func() {
		defer wg2.Done()
		if e := ch2.Cycle(); e != nil {
			errCh <- e
		}
	}()
	wg2.Wait()
	close(errCh)
	for e := range errCh {
		t.Fatalf("Cycle failed: %v", e)
	}
	<-done
	if cycleErr != nil {
		t.Fatalf("mux Cycle failed: %v", cycleErr)
	}
}

func TestMultiplexerConcurrent(t *testing.T) {
	mc := &mockCommanderWithData{data: []byte{0x42}}
	mux, err := NewMultiplexer(mc)
	if err != nil {
		t.Fatalf("NewMultiplexer failed: %v", err)
	}
	defer mux.Close()

	const numChannels = 10
	var wg sync.WaitGroup
	errorsCh := make(chan error, numChannels)
	channels := make([]Commander, numChannels)

	// Open all channels first.
	for i := 0; i < numChannels; i++ {
		ch, err := mux.OpenCommander()
		if err != nil {
			t.Fatalf("OpenCommander %d failed: %v", i, err)
		}
		channels[i] = ch
	}

	// Step 1: All channels call New() concurrently.
	for i := 0; i < numChannels; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := channels[idx].New(1)
			if err != nil {
				errorsCh <- fmt.Errorf("goroutine %d: New failed: %v", idx, err)
			}
		}(i)
	}
	wg.Wait()

	// Step 2: Trigger the mux cycle.
	var cycleErr error
	cycleDone := make(chan struct{})
	go func() {
		cycleErr = mux.Cycle()
		close(cycleDone)
	}()

	// Step 3: All channels call Cycle() concurrently.
	for i := 0; i < numChannels; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := channels[idx].Cycle()
			if err != nil {
				errorsCh <- fmt.Errorf("goroutine %d: Cycle failed: %v", idx, err)
			}
		}(i)
	}
	wg.Wait()
	<-cycleDone
	close(errorsCh)

	if cycleErr != nil {
		t.Fatalf("mux Cycle failed: %v", cycleErr)
	}
	for err := range errorsCh {
		t.Error(err)
	}
}

func TestMultiplexerClose(t *testing.T) {
	mc := &mockCommanderWithData{}
	mux, err := NewMultiplexer(mc)
	if err != nil {
		t.Fatalf("NewMultiplexer failed: %v", err)
	}

	err = mux.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Operations after close should fail.
	_, err = mux.OpenCommander()
	if err == nil {
		t.Error("OpenCommander after Close should fail")
	}
}

func TestMultiplexerMultipleCycles(t *testing.T) {
	mc := &mockCommanderWithData{data: []byte{0x01, 0x02, 0x03, 0x04}}
	mux, err := NewMultiplexer(mc)
	if err != nil {
		t.Fatalf("NewMultiplexer failed: %v", err)
	}
	defer mux.Close()

	ch, err := mux.OpenCommander()
	if err != nil {
		t.Fatalf("OpenCommander failed: %v", err)
	}

	addr := makeAddr(0, 0x120)

	// First cycle.
	var cycleErr error
	done := make(chan struct{})
	go func() {
		cycleErr = mux.Cycle()
		close(done)
	}()
	_, err = ExecuteRead8(ch, addr, 1)
	if err != nil {
		t.Fatalf("first ExecuteRead8 failed: %v", err)
	}
	<-done
	if cycleErr != nil {
		t.Fatalf("first mux Cycle failed: %v", cycleErr)
	}

	// Second cycle.
	done = make(chan struct{})
	go func() {
		cycleErr = mux.Cycle()
		close(done)
	}()
	_, err = ExecuteRead8(ch, addr, 1)
	if err != nil {
		t.Fatalf("second ExecuteRead8 failed: %v", err)
	}
	<-done
	if cycleErr != nil {
		t.Fatalf("second mux Cycle failed: %v", cycleErr)
	}
}

// ─── mockCommanderWithData ────────────────────────────────────────────────────

// mockCommanderWithData is a Commander mock that returns pre-configured data.
type mockCommanderWithData struct {
	mu            sync.Mutex
	commands      []*mockCmd
	data          []byte
	wc            uint16
	noArriveCount int
	cycleCount    int
}

func (m *mockCommanderWithData) New(datalen int) (*ExecutingCommand, error) {
	buf := make([]byte, datalen+ecfr.DatagramOverheadLength)
	dg, err := ecfr.PointDatagramTo(buf)
	if err != nil {
		return nil, err
	}
	if err := dg.SetDataLen(datalen); err != nil {
		return nil, err
	}

	ec := &ExecutingCommand{
		DatagramOut: &dg,
	}

	m.mu.Lock()
	m.commands = append(m.commands, &mockCmd{ec: ec, datalen: datalen})
	m.mu.Unlock()

	return ec, nil
}

func (m *mockCommanderWithData) Cycle() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cycleCount++

	for _, cmd := range m.commands {
		if cmd.ec.Arrived {
			continue
		}

		if m.cycleCount <= m.noArriveCount {
			cmd.ec.Arrived = false
			cmd.ec.Overlayed = false
			continue
		}

		dgo := cmd.ec.DatagramOut
		n := int(dgo.Header.DataLength())
		rsBuf := make([]byte, n+ecfr.DatagramOverheadLength)
		rsDg, err := ecfr.PointDatagramTo(rsBuf)
		if err != nil {
			return err
		}
		if err := rsDg.SetDataLen(n); err != nil {
			return err
		}
		rsDg.Header.Command = dgo.Header.Command
		rsDg.Header.Addr32 = dgo.Header.Addr32
		rsDg.Header.Index = dgo.Header.Index

		// For read commands, set the pre-configured data.
		if dgo.Header.Command.DoesRead() && len(m.data) >= n {
			copy(rsDg.Data, m.data[:n])
		}

		wc := m.wc
		if wc == 0 {
			wc = 1
		}
		rsDg.WKC = wc

		cmd.ec.DatagramIn = &rsDg
		cmd.ec.Arrived = true
		cmd.ec.Overlayed = true
	}

	return nil
}

func (m *mockCommanderWithData) Close() error {
	return nil
}

// ─── Benchmark Tests ──────────────────────────────────────────────────────────

func BenchmarkExecuteRead8(b *testing.B) {
	mc := &mockCommanderWithData{data: []byte{0x42}}
	addr := makeAddr(0, 0x120)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mc.cycleCount = 0
		mc.commands = nil
		_, _ = ExecuteRead8(mc, addr, 1)
	}
}

func BenchmarkExecuteWrite8(b *testing.B) {
	mc := &mockCommanderWithData{}
	addr := makeAddr(0, 0x120)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mc.cycleCount = 0
		mc.commands = nil
		_ = ExecuteWrite8(mc, addr, 0x42, 1)
	}
}

func BenchmarkCommandFramerNew(b *testing.B) {
	f := &mockFramer{}
	cf := NewCommandFramer(f)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cf.New(4)
	}
}

func BenchmarkCommandFramerCycle(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		f := &mockFramer{}
		cf := NewCommandFramer(f)
		_, _ = cf.New(4)
		b.StartTimer()

		_ = cf.Cycle()
	}
}

func BenchmarkMultiplexerCycle(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		mc := &mockCommanderWithData{data: []byte{0x42}}
		mux, _ := NewMultiplexer(mc)
		ch, _ := mux.OpenCommander()
		addr := makeAddr(0, 0x120)
		b.StartTimer()

		// Trigger the mux cycle in a goroutine.
		go func() {
			mux.Cycle()
		}()

		_, _ = ExecuteRead8(ch, addr, 1)
		b.StopTimer()
		mux.Close()
	}
}

// ─── Batch Boundary Tests ─────────────────────────────────────────────────────

// TestCommandFramerMaxDatagrams verifies the maximum number of datagrams
// that can be packed into a single frame, and that overflow correctly
// creates a new frame.
//
// Each 0-byte datagram consumes DatagramOverheadLength (12) bytes.
// CommandFramerMaxDatagramsLen = 1470, so max datagrams = 1470 / 12 = 122.
func TestCommandFramerMaxDatagrams(t *testing.T) {
	const maxDatagrams = 122

	// Case 1: exactly maxDatagrams should fit in one frame.
	f := &mockFramer{}
	cf := NewCommandFramer(f)

	for i := 0; i < maxDatagrams; i++ {
		_, err := cf.New(0)
		if err != nil {
			t.Fatalf("New(%d) failed: %v", i, err)
		}
	}

	err := cf.Cycle()
	if err != nil {
		t.Fatalf("Cycle failed: %v", err)
	}

	if len(f.frames) != 1 {
		t.Fatalf("expected 1 frame for %d datagrams, got %d", maxDatagrams, len(f.frames))
	}
	if len(f.frames[0].Datagrams) != maxDatagrams {
		t.Errorf("expected %d datagrams in frame, got %d", maxDatagrams, len(f.frames[0].Datagrams))
	}

	// Verify Last flag is set correctly: only the last datagram has Last=true.
	for i, dg := range f.frames[0].Datagrams {
		if i == len(f.frames[0].Datagrams)-1 {
			if !dg.Header.Last() {
				t.Errorf("datagram %d: expected Last=true", i)
			}
		} else {
			if dg.Header.Last() {
				t.Errorf("datagram %d: expected Last=false", i)
			}
		}
	}

	// Case 2: maxDatagrams+1 should overflow to two frames.
	f2 := &mockFramer{}
	cf2 := NewCommandFramer(f2)
	for i := 0; i < maxDatagrams+1; i++ {
		_, err := cf2.New(0)
		if err != nil {
			t.Fatalf("cf2.New(%d) failed: %v", i, err)
		}
	}
	err = cf2.Cycle()
	if err != nil {
		t.Fatalf("cf2.Cycle failed: %v", err)
	}
	if len(f2.frames) != 2 {
		t.Fatalf("expected 2 frames for %d datagrams, got %d", maxDatagrams+1, len(f2.frames))
	}

	// First frame should have maxDatagrams, second frame should have 1.
	if len(f2.frames[0].Datagrams) != maxDatagrams {
		t.Errorf("frame 0: expected %d datagrams, got %d", maxDatagrams, len(f2.frames[0].Datagrams))
	}
	if len(f2.frames[1].Datagrams) != 1 {
		t.Errorf("frame 1: expected 1 datagram, got %d", len(f2.frames[1].Datagrams))
	}

	// Verify frame indices are correct.
	if f2.frames[0].Datagrams[0].Header.Index != 0 {
		t.Errorf("frame 0: expected index 0, got %d", f2.frames[0].Datagrams[0].Header.Index)
	}
	if f2.frames[1].Datagrams[0].Header.Index != 1 {
		t.Errorf("frame 1: expected index 1, got %d", f2.frames[1].Datagrams[0].Header.Index)
	}
}

// TestCommandFramerMultiFrameCycle verifies that multiple frames created
// across one or more cycles are correctly matched to incoming responses.
func TestCommandFramerMultiFrameCycle(t *testing.T) {
	// Sub-test 1: datagrams spanning two frames in a single Cycle.
	t.Run("SingleCycleTwoFrames", func(t *testing.T) {
		f := &mockFramer{}
		cf := NewCommandFramer(f)

		// First frame: one large datagram that fills the frame.
		firstLen := CommandFramerMaxDatagramsLen - ecfr.DatagramOverheadLength
		cmd1, err := cf.New(firstLen)
		if err != nil {
			t.Fatalf("New(%d) failed: %v", firstLen, err)
		}
		cmd1.DatagramOut.Header.Command = ecfr.APRD
		cmd1.DatagramOut.Header.Addr32 = 0x1000

		// Second frame: two small datagrams (first overflows to new frame).
		cmd2, err := cf.New(4)
		if err != nil {
			t.Fatalf("New(4) failed: %v", err)
		}
		cmd2.DatagramOut.Header.Command = ecfr.APWR
		cmd2.DatagramOut.Header.Addr32 = 0x2000

		cmd3, err := cf.New(4)
		if err != nil {
			t.Fatalf("New(4) failed: %v", err)
		}
		cmd3.DatagramOut.Header.Command = ecfr.APRD
		cmd3.DatagramOut.Header.Addr32 = 0x3000

		err = cf.Cycle()
		if err != nil {
			t.Fatalf("Cycle failed: %v", err)
		}

		if len(f.frames) != 2 {
			t.Fatalf("expected 2 frames, got %d", len(f.frames))
		}

		// All commands should have arrived.
		if !cmd1.Arrived {
			t.Error("cmd1 should have arrived")
		}
		if !cmd2.Arrived {
			t.Error("cmd2 should have arrived")
		}
		if !cmd3.Arrived {
			t.Error("cmd3 should have arrived")
		}

		// Verify correct command matching.
		if cmd1.DatagramIn.Header.Command != ecfr.APRD {
			t.Errorf("cmd1: expected APRD, got %v", cmd1.DatagramIn.Header.Command)
		}
		if cmd2.DatagramIn.Header.Command != ecfr.APWR {
			t.Errorf("cmd2: expected APWR, got %v", cmd2.DatagramIn.Header.Command)
		}
		if cmd3.DatagramIn.Header.Command != ecfr.APRD {
			t.Errorf("cmd3: expected APRD, got %v", cmd3.DatagramIn.Header.Command)
		}

		// Verify frame indices.
		if f.frames[0].Datagrams[0].Header.Index != 0 {
			t.Errorf("frame 0 index: expected 0, got %d", f.frames[0].Datagrams[0].Header.Index)
		}
		if f.frames[1].Datagrams[0].Header.Index != 1 {
			t.Errorf("frame 1 index: expected 1, got %d", f.frames[1].Datagrams[0].Header.Index)
		}
	})

	// Sub-test 2: multiple independent cycles, each with its own frame.
	// The mockFramer is designed for single-use, so each cycle uses a fresh
	// mockFramer and CommandFramer to simulate independent bus cycles.
	t.Run("MultipleCycles", func(t *testing.T) {
		// First cycle: one command.
		f1 := &mockFramer{}
		cf1 := NewCommandFramer(f1)
		cmd1, err := cf1.New(4)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}
		cmd1.DatagramOut.Header.Command = ecfr.APRD
		cmd1.DatagramOut.Header.Addr32 = 0x1000

		err = cf1.Cycle()
		if err != nil {
			t.Fatalf("first Cycle failed: %v", err)
		}

		if !cmd1.Arrived {
			t.Error("cmd1 should have arrived after first cycle")
		}

		// Second cycle: another command with a fresh CommandFramer.
		f2 := &mockFramer{}
		cf2 := NewCommandFramer(f2)
		cmd2, err := cf2.New(4)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}
		cmd2.DatagramOut.Header.Command = ecfr.APWR
		cmd2.DatagramOut.Header.Addr32 = 0x2000

		err = cf2.Cycle()
		if err != nil {
			t.Fatalf("second Cycle failed: %v", err)
		}

		if !cmd2.Arrived {
			t.Error("cmd2 should have arrived after second cycle")
		}
		if cmd2.DatagramIn.Header.Command != ecfr.APWR {
			t.Errorf("cmd2: expected APWR, got %v", cmd2.DatagramIn.Header.Command)
		}
	})
}

// ─── Batch Benchmark Tests ────────────────────────────────────────────────────

// BenchmarkBatchReadWrite benchmarks batch read and write operations executed
// in a single Cycle via the CommandFramer.
func BenchmarkBatchReadWrite(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		f := &mockFramer{}
		cf := NewCommandFramer(f)
		addr := makeAddr(0, 0x120)

		// Read command.
		cmd1, _ := cf.New(4)
		cmd1.DatagramOut.Header.Command = ecfr.APRD
		cmd1.DatagramOut.Header.Addr32 = addr.Addr32()

		// Write command.
		cmd2, _ := cf.New(4)
		cmd2.DatagramOut.Header.Command = ecfr.APWR
		cmd2.DatagramOut.Header.Addr32 = addr.Addr32()
		copy(cmd2.DatagramOut.Data, []byte{0x01, 0x02, 0x03, 0x04})

		// Another read command.
		cmd3, _ := cf.New(1)
		cmd3.DatagramOut.Header.Command = ecfr.APRD
		cmd3.DatagramOut.Header.Addr32 = addr.Addr32()

		b.StartTimer()

		_ = cf.Cycle()
	}
}

// BenchmarkCommandFramerBatch benchmarks packing multiple commands into the same
// frame and cycling them in a single operation.
func BenchmarkCommandFramerBatch(b *testing.B) {
	b.ReportAllocs()

	const numCommands = 10

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		f := &mockFramer{}
		cf := NewCommandFramer(f)

		// Pack many small commands into one frame.
		for j := 0; j < numCommands; j++ {
			cmd, _ := cf.New(4)
			cmd.DatagramOut.Header.Command = ecfr.APRD
			cmd.DatagramOut.Header.Addr32 = 0x1000
		}

		b.StartTimer()
		_ = cf.Cycle()
	}
}

// BenchmarkExecuteWriteStackAlloc benchmarks the stack-allocated write operation
// (ExecuteWrite8) to verify zero heap allocations from the stack-allocated
// byte slice optimization.
func BenchmarkExecuteWriteStackAlloc(b *testing.B) {
	b.ReportAllocs()
	mc := &mockCommanderWithData{}
	addr := makeAddr(0, 0x120)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mc.cycleCount = 0
		mc.commands = nil
		_ = ExecuteWrite8(mc, addr, 0x42, 1)
	}
}
