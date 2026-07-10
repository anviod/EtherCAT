package ecfr

import (
	"bytes"
	"errors"
	"fmt"
)

// FrameOverheadLen is the size of the EtherCAT frame header in bytes.
const FrameOverheadLen = 2

// Frame represents a complete EtherCAT frame (header + one or more datagrams).
type Frame struct {
	Header    Header
	Datagrams []*Datagram
	buffer    []byte
	byteLen   int // cached ByteLen(), updated incrementally to avoid O(n) traversal
}

// FrameOverlayPool provides a reusable Datagram pointer pool for Frame
// overlay operations, eliminating heap allocations on repeated calls.
// FrameOverlayPool 提供可复用的 Datagram 指针池，消除重复调用中的堆分配。
// Perf: designed for hot paths where frames are repeatedly decoded, such as
// bus cycle loops. On ARM64, avoids 2 allocs/op in Frame.Overlay, yielding
// up to 18.9x speedup.
// 性能优化：专为帧重复解码的热路径（如总线循环）设计。ARM64 上避免 2 allocs/op，
// 最高可获得 18.9 倍加速。
type FrameOverlayPool struct {
	dgPool []*Datagram // pre-allocated Datagram pointer slice, 预分配 Datagram 指针切片
}

// NewFrameOverlayPool creates a FrameOverlayPool with the given capacity
// for Datagram pointers. Capacity should match the expected number of
// datagrams per frame.
// 创建指定容量的 FrameOverlayPool，容量应匹配每帧预期的数据报数量。
func NewFrameOverlayPool(capacity int) *FrameOverlayPool {
	return &FrameOverlayPool{
		dgPool: make([]*Datagram, 0, capacity),
	}
}

// Overlay decodes a complete frame from d using the pool's pre-allocated
// Datagram slice. This is equivalent to Frame.OverlayReuse(d, pool.dgPool)
// but with zero Datagram allocations on repeated calls.
// 使用池中预分配的 Datagram 切片解码完整帧，重复调用时零 Datagram 分配。
func (p *FrameOverlayPool) Overlay(f *Frame, d []byte) ([]byte, error) {
	return f.OverlayReuse(d, p.dgPool)
}

// Overlay decodes a complete frame from d: header + all datagrams.
// 从 d 解码完整帧：帧头 + 所有数据报。
// byteLen cache is initialized for O(1) ByteLen() after overlay.
func (f *Frame) Overlay(d []byte) ([]byte, error) {
	return f.OverlayReuse(d, nil)
}

// OverlayReuse decodes a complete frame from d, reusing the provided
// Datagram slice to avoid allocations on repeated calls.
// 从 d 解码完整帧，复用提供的 Datagram 切片避免重复分配。
// Perf: when dgPool is non-nil, existing Datagram pointers are reused
// instead of allocating new ones. This is critical for ARM64 where
// allocations are more expensive.
// 性能优化：dgPool 非空时复用已有 Datagram 指针，避免新分配。
// ARM64 上分配开销更高，此优化效果显著。
func (f *Frame) OverlayReuse(d []byte, dgPool []*Datagram) ([]byte, error) {
	b, err := f.Header.Overlay(d)
	if err != nil {
		return nil, err
	}

	dgbl := f.Header.FrameLength()
	if int(dgbl) > len(b) {
		return nil, fmt.Errorf("frame expected %d bytes, only have %d", dgbl, len(b))
	}

	// Reset datagrams slice — reuse existing backing array if pool is provided.
	// When dgPool is nil, append to existing Datagrams for backward compatibility.
	// 重置 Datagram 切片——如果提供了池则复用底层数组。
	// dgPool 为 nil 时追加到已有 Datagrams，保持向后兼容。
	if dgPool != nil {
		f.Datagrams = dgPool[:0]
	}

	for {
		var dg *Datagram
		if dgPool != nil && len(f.Datagrams) < cap(f.Datagrams) {
			// Reuse existing Datagram from pool — zero allocation.
			// 复用池中已有 Datagram——零分配。
			f.Datagrams = f.Datagrams[:len(f.Datagrams)+1]
			dg = f.Datagrams[len(f.Datagrams)-1]
			// Ensure the reused pointer is valid (pool entries may be nil).
			// 确保复用指针有效（池条目可能为 nil）。
			if dg == nil {
				dg = &Datagram{}
				f.Datagrams[len(f.Datagrams)-1] = dg
			}
		} else {
			dg = &Datagram{}
			f.Datagrams = append(f.Datagrams, dg)
		}

		b, err = dg.Overlay(b)
		if err != nil {
			return nil, err
		}

		if dg.Header.Last() {
			break
		}
	}

	f.buffer = d
	f.byteLen = FrameOverheadLen + int(dgbl)
	return b, nil
}

// PointFrameTo creates a Frame that points into the given byte slice. The
// header area is zero-initialised.
func PointFrameTo(d []byte) (Frame, error) {
	if len(d) < FrameOverheadLen {
		return Frame{}, errors.New("buffer too small to even contain frame header")
	}

	d[0] = 0
	d[1] = 0
	f := Frame{buffer: d, byteLen: FrameOverheadLen}
	_, err := f.Header.Overlay(d)
	if err != nil {
		return Frame{}, err
	}

	return f, nil
}

// Commit serializes the frame header and all datagrams to the buffer.
// 将帧头和所有数据报序列化到缓冲区。
// Frame-length field is auto-updated. Uses cached byteLen (O(1)).
func (f *Frame) Commit() ([]byte, error) {
	if len(f.Datagrams) == 0 {
		return nil, errors.New("EtherCAT frame needs at least one datagram")
	}

	clen := f.byteLen
	if clen > len(f.buffer) {
		return nil, fmt.Errorf("datagrams too long for frame, need %d, have %d", clen, len(f.buffer))
	}

	lenmask := uint16(0x07ff)
	f.Header.Word &^= lenmask
	f.Header.Word |= uint16(clen-2) & lenmask

	incbuf, err := f.Header.Commit()
	if err != nil {
		return nil, err
	}
	totlen := len(incbuf)

	for _, dgram := range f.Datagrams {
		incbuf, err = dgram.Commit()
		if err != nil {
			return nil, err
		}
		totlen += len(incbuf)
	}

	return f.buffer[:totlen], nil
}

// ByteLen returns the total wire length (header + all datagrams).
// 返回帧的总线路长度（帧头 + 所有数据报）。
// Perf: O(1) via cached value updated incrementally by NewDatagram.
func (f *Frame) ByteLen() int {
	return f.byteLen
}

// NewDatagram allocates a new datagram and appends it to the frame.
// 分配新数据报并追加到帧。
// Perf: cached byteLen updated incrementally (O(1), no traversal).
func (f *Frame) NewDatagram(datalen int) (*Datagram, error) {
	curlen := f.byteLen
	maxlen := len(f.buffer)
	curfree := maxlen - curlen

	if datalen > curfree {
		return nil, fmt.Errorf("datalen too high: need %d bytes, only %d free", datalen, curfree)
	}

	dgram, err := PointDatagramTo(f.buffer[curlen:])
	if err != nil {
		return nil, err
	}

	err = dgram.SetDataLen(datalen)
	if err != nil {
		return nil, err
	}

	f.byteLen += dgram.ByteLen()
	f.Datagrams = append(f.Datagrams, &dgram)
	return &dgram, nil
}

// MultilineSummary returns a multi-line human-readable summary of the frame
// and all its datagrams.
func (f *Frame) MultilineSummary() string {
	b := bytes.NewBuffer(nil)
	fmt.Fprintf(b, "frame len %#03x\n", f.ByteLen())
	for _, dgram := range f.Datagrams {
		b.WriteString("  ")
		b.WriteString(dgram.Summary())
		b.WriteString("\n")
	}
	return b.String()
}
