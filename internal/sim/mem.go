package sim

import (
	"github.com/anviod/EtherCAT/ecfr"
)

const (
	maxDatagramsLen = 1470
)

// Framer is the local interface for frame-based EtherCAT communication.
// It is defined here to avoid a circular dependency on the ecmd package.
// L2Bus satisfies ecmd.Framer through structural typing.
type Framer interface {
	New(maxdatalen int) (*ecfr.Frame, error)
	Cycle() ([]*ecfr.Frame, error)
	Close() error
}

// L2Bus represents a layer-2 EtherCAT bus with a collection of slaves.
// Frames are created through New(), processed through all slaves during
// Cycle(), and the bus is cleaned up with Close().
type L2Bus struct {
	oframes []*ecfr.Frame
	Slaves  []FrameProcessor
}

// New creates a new EtherCAT frame with the specified max datagram data length.
// 创建指定最大数据长度的 EtherCAT 帧。
func (b *L2Bus) New(maxdatalen int) (fr *ecfr.Frame, err error) {
	var vframe ecfr.Frame
	buf := make([]byte, maxDatagramsLen+ecfr.FrameOverheadLen)
	vframe, err = ecfr.PointFrameTo(buf)
	if err != nil {
		return
	}

	vframe.Header.SetType(1)

	fr = &vframe
	b.oframes = append(b.oframes, fr)
	return
}

// Cycle processes all queued frames through the slave chain: commit → copy → overlay → process.
// 处理所有排队帧通过从站链：提交 → 复制 → 解析 → 处理。
// Outgoing frame queue is cleared after Cycle returns.
func (b *L2Bus) Cycle() (iframes []*ecfr.Frame, err error) {
	defer func() {
		b.oframes = nil
	}()

	for _, oframe := range b.oframes {
		var obytes []byte

		obytes, err = oframe.Commit()
		if err != nil {
			return
		}

		// Each frame gets its own buffer — Frame.Overlay stores a reference
		// to the buffer, and Datagram objects reference it via unsafe.Pointer.
		// 每帧独立缓冲区——Frame.Overlay 持有 buffer 引用，Datagram 通过 unsafe.Pointer 引用。
		coframe := new(ecfr.Frame)
		cbytes := make([]byte, len(obytes))
		copy(cbytes, obytes)
		_, err = coframe.Overlay(cbytes)
		if err != nil {
			return
		}

		for _, slave := range b.Slaves {
			coframe = slave.ProcessFrame(coframe)
			if coframe == nil {
				break
			}
		}

		if coframe != nil {
			iframes = append(iframes, coframe)
		}
	}

	return
}

// Close closes the bus and releases any resources. Currently a no-op.
func (b *L2Bus) Close() error { return nil }
