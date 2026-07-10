package ecmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/anviod/EtherCAT/ecfr"
	"github.com/anviod/EtherCAT/internal/marshalling"
)

// Commander defines the interface for executing EtherCAT commands.
type Commander interface {
	// New creates a new executing command with the given data length.
	New(datalen int) (*ExecutingCommand, error)
	// Cycle performs one round of command execution.
	Cycle() error
	// Close closes the commander and releases resources.
	Close() error
}

// ExecutingCommand represents a command that is in the process of being executed.
type ExecutingCommand struct {
	DatagramOut *ecfr.Datagram
	DatagramIn  *ecfr.Datagram
	Arrived     bool
	Overlayed   bool
	Error       error
}

// WorkingCounterError represents an error when the working counter does not match expectations.
type WorkingCounterError struct {
	Command    ecfr.CommandType
	Addr32     uint32
	Want, Have uint16
}

// Error implements the error interface.
func (e WorkingCounterError) Error() string {
	return fmt.Sprintf("working counter error, want %d, have %d on %v %#08x",
		e.Want, e.Have, e.Command, e.Addr32)
}

// Options contains configuration options for command execution.
type Options struct {
	// FramelossTries is the number of retries when a frame is lost.
	// If 0, DefaultFramelossTries (3) is used.
	FramelossTries int
	// WCDeadline is the deadline by which a working counter match must occur.
	// If zero, no deadline is enforced.
	WCDeadline time.Time
}

// Default values for options.
const (
	DefaultFramelossTries = 3
)

// Sentinel errors.
var (
	// ErrNoFrame indicates that a frame did not arrive within the retry limit.
	ErrNoFrame = errors.New("frame did not arrive")
	// ErrNoOverlay indicates that the incoming datagram could not be overlaid.
	ErrNoOverlay = errors.New("failed to overlay")
)

// getFramelossTries returns the effective frame loss retry count.
func (o Options) getFramelossTries() int {
	if o.FramelossTries == 0 {
		return DefaultFramelossTries
	}
	return o.FramelossTries
}

// getWCDeadline returns the working counter deadline.
func (o Options) getWCDeadline() time.Time { return o.WCDeadline }

// ChooseDefaultError selects the appropriate error based on command state.
func ChooseDefaultError(cmd *ExecutingCommand) error {
	if !cmd.Arrived {
		return ErrNoFrame
	}

	if !cmd.Overlayed {
		return ErrNoOverlay
	}

	return cmd.Error
}

// IsNoFrame checks if the error is an ErrNoFrame error.
func IsNoFrame(err error) bool {
	return err == ErrNoFrame
}

// IsWorkingCounterError checks if the error is a WorkingCounterError.
func IsWorkingCounterError(err error) bool {
	_, ok := err.(WorkingCounterError)
	return ok
}

// ChooseWorkingCounterError creates a WorkingCounterError if the working counter
// doesn't match the expected value.
func ChooseWorkingCounterError(ec *ExecutingCommand, expwc uint16) error {
	havewc := ec.DatagramIn.WKC
	if expwc != havewc {
		return WorkingCounterError{
			Command: ec.DatagramOut.Header.Command,
			Addr32:  ec.DatagramOut.Header.Addr32,
			Want:    expwc,
			Have:    havewc,
		}
	}

	return nil
}

// ExecuteRead8 reads a single byte from the given address.
func ExecuteRead8(c Commander, addr ecfr.DatagramAddress, expwc uint16) (uint8, error) {
	return ExecuteRead8Options(c, addr, expwc, Options{})
}

// ExecuteRead8Options reads a single byte with custom options.
func ExecuteRead8Options(c Commander, addr ecfr.DatagramAddress, expwc uint16, opts Options) (uint8, error) {
	ds, err := ExecuteReadOptions(c, addr, 1, expwc, opts)
	if err != nil {
		return 0, err
	}
	return marshalling.XGetUint8(ds), nil
}

// ExecuteRead16 reads a 16-bit word from the given address.
func ExecuteRead16(c Commander, addr ecfr.DatagramAddress, expwc uint16) (uint16, error) {
	return ExecuteRead16Options(c, addr, expwc, Options{})
}

// ExecuteRead16Options reads a 16-bit word with custom options.
func ExecuteRead16Options(c Commander, addr ecfr.DatagramAddress, expwc uint16, opts Options) (uint16, error) {
	ds, err := ExecuteReadOptions(c, addr, 2, expwc, opts)
	if err != nil {
		return 0, err
	}
	return marshalling.XGetUint16(ds), nil
}

// ExecuteRead32 reads a 32-bit dword from the given address.
func ExecuteRead32(c Commander, addr ecfr.DatagramAddress, expwc uint16) (uint32, error) {
	return ExecuteRead32Options(c, addr, expwc, Options{})
}

// ExecuteRead32Options reads a 32-bit dword with custom options.
func ExecuteRead32Options(c Commander, addr ecfr.DatagramAddress, expwc uint16, opts Options) (uint32, error) {
	ds, err := ExecuteReadOptions(c, addr, 4, expwc, opts)
	if err != nil {
		return 0, err
	}
	return marshalling.XGetUint32(ds), nil
}

// ExecuteRead reads n bytes from the given address.
func ExecuteRead(c Commander, addr ecfr.DatagramAddress, n int, expwc uint16) ([]byte, error) {
	return ExecuteReadOptions(c, addr, n, expwc, Options{})
}

// ExecuteReadOptions reads n bytes with custom options.
// 读取 n 字节（带自定义选项）。
// Hot path: creates command → sets addr → Cycle() → returns data.
func ExecuteReadOptions(c Commander, addr ecfr.DatagramAddress, n int, expwc uint16, opts Options) ([]byte, error) {
	nFrameLoss := 0

	var ct ecfr.CommandType
	switch addr.Type() {
	case ecfr.Positional:
		ct = ecfr.APRD
	case ecfr.Fixed:
		ct = ecfr.FPRD
	case ecfr.Broadcast:
		ct = ecfr.BRD
	default:
		return nil, fmt.Errorf("ExecuteReadOptions: unsupported address type %v", addr.Type())
	}

	for {
		ec, err := c.New(n)
		if err != nil {
			return nil, err
		}

		dgo := ec.DatagramOut
		dgo.Header.Command = ct
		dgo.Header.Addr32 = addr.Addr32()

		err = c.Cycle()
		if err != nil {
			return nil, err
		}

		err = ChooseDefaultError(ec)
		if err != nil {
			if IsNoFrame(err) {
				nFrameLoss++
				if nFrameLoss < opts.getFramelossTries() {
					continue
				}
			}
			return nil, err
		}

		err = ChooseWorkingCounterError(ec, expwc)
		if err != nil {
			now := time.Now()
			if !opts.getWCDeadline().IsZero() && now.Before(opts.getWCDeadline()) {
				continue
			}
			return nil, err
		}

		return ec.DatagramIn.Data, nil
	}
}

// ExecuteWrite8 writes a single byte to the given address.
func ExecuteWrite8(c Commander, addr ecfr.DatagramAddress, v uint8, expwc uint16) error {
	return ExecuteWrite8Options(c, addr, v, expwc, Options{})
}

// ExecuteWrite8Options writes a single byte with custom options.
// 写入单字节（带自定义选项）。
// Perf: stack-allocated array, zero heap allocation.
func ExecuteWrite8Options(c Commander, addr ecfr.DatagramAddress, v uint8, expwc uint16, opts Options) error {
	var ws [1]byte
	marshalling.PutUint8(ws[:], v)
	return ExecuteWriteOptions(c, addr, ws[:], expwc, opts)
}

// ExecuteWrite16 writes a 16-bit word to the given address.
func ExecuteWrite16(c Commander, addr ecfr.DatagramAddress, v uint16, expwc uint16) error {
	return ExecuteWrite16Options(c, addr, v, expwc, Options{})
}

// ExecuteWrite16Options writes a 16-bit word with custom options.
// 写入 16 位字（带自定义选项）。
// Perf: stack-allocated array, zero heap allocation.
func ExecuteWrite16Options(c Commander, addr ecfr.DatagramAddress, v uint16, expwc uint16, opts Options) error {
	var ws [2]byte
	marshalling.PutUint16(ws[:], v)
	return ExecuteWriteOptions(c, addr, ws[:], expwc, opts)
}

// ExecuteWrite32 writes a 32-bit dword to the given address.
func ExecuteWrite32(c Commander, addr ecfr.DatagramAddress, v uint32, expwc uint16) error {
	return ExecuteWrite32Options(c, addr, v, expwc, Options{})
}

// ExecuteWrite32Options writes a 32-bit dword with custom options.
// 写入 32 位双字（带自定义选项）。
// Perf: stack-allocated array, zero heap allocation.
func ExecuteWrite32Options(c Commander, addr ecfr.DatagramAddress, v uint32, expwc uint16, opts Options) error {
	var ws [4]byte
	marshalling.PutUint32(ws[:], v)
	return ExecuteWriteOptions(c, addr, ws[:], expwc, opts)
}

// ExecuteWrite writes n bytes to the given address.
func ExecuteWrite(c Commander, addr ecfr.DatagramAddress, w []byte, expwc uint16) error {
	return ExecuteWriteOptions(c, addr, w, expwc, Options{})
}

// ExecuteWriteOptions writes n bytes with custom options.
// 写入 n 字节（带自定义选项）。
// Hot path: creates command → sets addr → Cycle() → checks WKC.
func ExecuteWriteOptions(c Commander, addr ecfr.DatagramAddress, w []byte, expwc uint16, opts Options) error {
	nFrameLoss := 0

	var ct ecfr.CommandType
	switch addr.Type() {
	case ecfr.Positional:
		ct = ecfr.APWR
	case ecfr.Fixed:
		ct = ecfr.FPWR
	case ecfr.Broadcast:
		ct = ecfr.BWR
	default:
		return fmt.Errorf("ExecuteWriteOptions: unsupported address type %v", addr.Type())
	}

	for {
		ec, err := c.New(len(w))
		if err != nil {
			return err
		}

		dgo := ec.DatagramOut
		copy(dgo.Data, w)

		dgo.Header.Command = ct
		dgo.Header.Addr32 = addr.Addr32()

		err = c.Cycle()
		if err != nil {
			return err
		}

		err = ChooseDefaultError(ec)
		if err != nil {
			if IsNoFrame(err) {
				nFrameLoss++
				if nFrameLoss < opts.getFramelossTries() {
					continue
				}
			}
			return err
		}

		err = ChooseWorkingCounterError(ec, expwc)
		if err != nil {
			now := time.Now()
			if !opts.getWCDeadline().IsZero() && now.Before(opts.getWCDeadline()) {
				continue
			}
			return err
		}

		return nil
	}
}
