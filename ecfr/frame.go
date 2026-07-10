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

// Overlay decodes a complete frame from d: header + all datagrams.
// 从 d 解码完整帧：帧头 + 所有数据报。
// byteLen cache is initialized for O(1) ByteLen() after overlay.
func (f *Frame) Overlay(d []byte) ([]byte, error) {
	b, err := f.Header.Overlay(d)
	if err != nil {
		return nil, err
	}

	dgbl := f.Header.FrameLength()
	if int(dgbl) > len(b) {
		return nil, fmt.Errorf("frame expected %d bytes, only have %d", dgbl, len(b))
	}

	for {
		dg := &Datagram{}
		b, err = dg.Overlay(b)
		if err != nil {
			return nil, err
		}

		f.Datagrams = append(f.Datagrams, dg)

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
		return nil, errors.New("ecat frame needs at least one datagram")
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