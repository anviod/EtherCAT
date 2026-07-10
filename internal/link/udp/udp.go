package udp

import (
	"net"
	"time"

	"github.com/anviod/EtherCAT/ecfr"

	"golang.org/x/net/ipv4"
)

const (
	// EthercatUDPPort is the standard EtherCAT UDP port number.
	EthercatUDPPort = 0x88A4
)

const (
	udpReceiveBuflen = 1500
	maxDatagramsLen  = 1470
)

// outgoingFrame pairs an Ethernet frame with its EtherCAT payload frame
// so that both can be written and sent together during Cycle().
type outgoingFrame struct {
	ethFrame *ecfr.ETHFrame
	frame    *ecfr.Frame
}

// UDPFramer implements ecmd.Framer using UDP multicast transport.
type UDPFramer struct {
	oframes []outgoingFrame

	sock      *net.UDPConn
	conn      *ipv4.PacketConn
	iface     *net.Interface
	group     net.IP
	dst       *net.UDPAddr
	cycletime time.Duration

	cycnum int
}

// NewUDPFramer creates a new UDP multicast EtherCAT framer.
//
// It binds a UDP socket to EthercatUDPPort, joins the given multicast group
// on the specified interface, and configures the ipv4.PacketConn for
// multicast send/receive. Loopback is disabled.
func NewUDPFramer(iface *net.Interface, group net.IP, cycletime time.Duration) (*UDPFramer, error) {
	f := &UDPFramer{
		group:     group,
		iface:     iface,
		cycletime: cycletime,
	}

	laddr := &net.UDPAddr{IP: net.IPv4(0, 0, 0, 0), Port: EthercatUDPPort}
	f.dst = &net.UDPAddr{IP: f.group, Port: EthercatUDPPort}

	var err error
	f.sock, err = net.ListenUDP("udp4", laddr)
	if err != nil {
		return nil, err
	}

	f.conn = ipv4.NewPacketConn(f.sock)

	if err = f.conn.SetMulticastInterface(f.iface); err != nil {
		f.sock.Close()
		return nil, err
	}

	if err = f.conn.JoinGroup(iface, &net.UDPAddr{IP: group}); err != nil {
		f.sock.Close()
		return nil, err
	}

	if err = f.conn.SetMulticastLoopback(false); err != nil {
		f.sock.Close()
		return nil, err
	}

	return f, nil
}

// New creates a new EtherCAT frame wrapped in an Ethernet frame.
//
// It allocates an Ethernet frame buffer via ecfr.OverlayETHFrame, creates an
// ecfr.Frame in the payload area, and sets the EtherCAT type to 1. The frame
// is queued for the next Cycle() call.
func (f *UDPFramer) New(maxdatalen int) (*ecfr.Frame, error) {
	ethFrame, err := ecfr.OverlayETHFrame(nil)
	if err != nil {
		return nil, err
	}

	// Set EtherCAT EtherType and broadcast destination.
	ethFrame.Type = EthercatUDPPort
	ethFrame.Destination = ecfr.ETHAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

	payload := ethFrame.GetPayload()
	frame, err := ecfr.PointFrameTo(payload)
	if err != nil {
		return nil, err
	}

	frame.Header.SetType(1) // EtherCAT frame type

	fr := &frame
	f.oframes = append(f.oframes, outgoingFrame{ethFrame, fr})
	return fr, nil
}

// Cycle sends all queued outgoing frames to the multicast group and reads
// incoming responses.
//
// It supports cycle stretching: if fewer responses than sent frames are
// received, it will wait up to 10 additional cycle-time periods before
// giving up. Each read operation uses a deadline-based timeout.
func (f *UDPFramer) Cycle() (iframes []*ecfr.Frame, err error) {
	defer func() {
		f.cycnum++
		f.oframes = nil
	}()

	for _, of := range f.oframes {
		// Commit the EtherCAT frame into the Ethernet payload buffer.
		_, err = of.frame.Commit()
		if err != nil {
			return nil, err
		}

		// Write the Ethernet header.
		if err = of.ethFrame.WriteDown(); err != nil {
			return nil, err
		}

		// Resize the Ethernet frame buffer to match the actual payload.
		if err = of.ethFrame.SetPayloadLen(of.frame.ByteLen()); err != nil {
			return nil, err
		}

		_, err = f.sock.WriteTo(of.ethFrame.GetFrameBuf(), f.dst)
		if err != nil {
			err = errorMask(err)
			return nil, err
		}
	}

	if err = f.sock.SetReadDeadline(time.Now().Add(f.cycletime)); err != nil {
		return nil, err
	}

	stretchcnt := 0
	rbuf := make([]byte, udpReceiveBuflen)
	for {
		var n int
		n, _, err = f.sock.ReadFromUDP(rbuf)
		if isTimeout(err) {
			if stretchcnt < 10 && len(iframes) < len(f.oframes) {
				stretchcnt++
				f.sock.SetReadDeadline(time.Now().Add(f.cycletime))
				continue
			}
			err = nil
			break
		}
		if err != nil {
			return nil, err
		}

		var fr ecfr.Frame
		_, err = fr.Overlay(rbuf[:n])
		if err != nil {
			// Discard malformed frames.
			continue
		}

		iframes = append(iframes, &fr)
		rbuf = make([]byte, udpReceiveBuflen)
	}

	return iframes, nil
}

// Close releases the UDP socket and the ipv4.PacketConn.
//
// Bug fix: the original code called f.Close() recursively inside Close(),
// causing a stack overflow. This version correctly calls f.sock.Close()
// and f.conn.Close() directly.
func (f *UDPFramer) Close() error {
	if f.conn != nil {
		f.conn.Close()
	}
	if f.sock != nil {
		return f.sock.Close()
	}
	return nil
}

// DebugMessage sends a debug message string to the multicast group on port 1024.
// Any error is intentionally ignored.
func (f *UDPFramer) DebugMessage(m string) {
	addr := *f.dst
	addr.Port = 1024

	// The error is intentionally ignored.
	f.sock.WriteTo([]byte(m), &addr)
}

// isTimeout reports whether err is a net.Error with a Timeout() == true.
func isTimeout(err error) bool {
	if ne, ok := err.(net.Error); ok {
		return ne.Timeout()
	}
	return false
}
