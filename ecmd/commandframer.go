package ecmd

import (
	"errors"

	"github.com/anviod/EtherCAT/ecfr"
)

const (
	// CommandFramerMaxDatagramsLen is the maximum length for datagrams in a frame.
	CommandFramerMaxDatagramsLen = 1470
)

// Framer defines the interface for low-level frame operations.
type Framer interface {
	// New creates a new frame with the given maximum data length.
	New(maxdatalen int) (*ecfr.Frame, error)
	// Cycle performs one round of frame I/O, returning the received frames.
	Cycle() ([]*ecfr.Frame, error)
}

type outgoingFrame struct {
	frame *ecfr.Frame
	cmds  []*ExecutingCommand
}

// CommandFramer implements Commander by scheduling commands into frames
// and matching responses.
type CommandFramer struct {
	currentIndex uint8

	frameOpen          bool
	currentFrame       *ecfr.Frame
	currentFrameLen    uint16
	currentFrameOffset uint16
	currentDgram       *ecfr.Datagram
	currentCmds        []*ExecutingCommand

	frameQueue   []outgoingFrame
	inFrameQueue []*ecfr.Frame

	framer Framer
}

// NewCommandFramer creates a new CommandFramer using the given Framer.
func NewCommandFramer(framer Framer) *CommandFramer {
	return &CommandFramer{framer: framer}
}

// New creates a new command with the given data length, adding it to the current frame.
// 创建指定数据长度的新命令并加入当前帧。
// Creates a new frame automatically if the current one is full.
func (cf *CommandFramer) New(datalen int) (*ExecutingCommand, error) {
	var err error

	dbgl := datalen + ecfr.DatagramOverheadLength
	if dbgl > CommandFramerMaxDatagramsLen {
		return nil, errors.New("datalen exceeds maximum datagram length")
	}

	if cf.frameOpen {
		if dbgl > int(cf.currentFrameLen-cf.currentFrameOffset) {
			cf.finishFrame()
			err = cf.newFrame()
			if err != nil {
				return nil, err
			}
		}
	} else {
		err = cf.newFrame()
		if err != nil {
			return nil, err
		}
	}

	dg, err := cf.currentFrame.NewDatagram(datalen)
	if err != nil {
		return nil, err
	}
	cf.currentDgram = dg

	cf.currentFrameOffset += uint16(dbgl)

	cmd := &ExecutingCommand{
		DatagramOut: dg,
	}
	cf.currentCmds = append(cf.currentCmds, cmd)
	return cmd, nil
}

// finishFrame finalizes the current frame: sets Last flags, assigns index, queues it.
// 完成当前帧：设置 Last 标志、分配索引、加入发送队列。
func (cf *CommandFramer) finishFrame() {
	if len(cf.currentFrame.Datagrams) > 0 {
		for i := 0; i < len(cf.currentFrame.Datagrams)-1; i++ {
			cf.currentFrame.Datagrams[i].Header.SetLast(false)
		}
		cf.currentFrame.Datagrams[0].Header.Index = cf.currentIndex
		cf.currentFrame.Datagrams[len(cf.currentFrame.Datagrams)-1].Header.SetLast(true)
		cf.frameQueue = append(cf.frameQueue, outgoingFrame{cf.currentFrame, cf.currentCmds})
	}

	cf.frameOpen = false
	cf.currentFrame = nil
	cf.currentFrameLen = 0
	cf.currentFrameOffset = 0xffff
	cf.currentCmds = nil
	cf.currentIndex++
}

// newFrame creates a new frame from the framer.
func (cf *CommandFramer) newFrame() error {
	frame, err := cf.framer.New(CommandFramerMaxDatagramsLen)
	if err != nil {
		return err
	}

	cf.currentFrame = frame
	cf.currentDgram = nil
	cf.currentCmds = nil
	cf.frameOpen = true
	cf.currentFrameLen = CommandFramerMaxDatagramsLen
	cf.currentFrameOffset = 0
	return nil
}

// Cycle sends all queued frames and matches incoming responses to outgoing commands.
// 发送所有排队帧并将响应匹配到对应的发送命令。
// Matching: by frame length, datagram count, index, then per-datagram command+length.
func (cf *CommandFramer) Cycle() error {
	if cf.currentFrame != nil && len(cf.currentFrame.Datagrams) > 0 {
		cf.finishFrame()
	}

	var err error
	cf.inFrameQueue, err = cf.framer.Cycle()
	if err != nil {
		return err
	}

	// Match incoming frames to outgoing frames by frame length, datagram count,
	// and index.
	oi := 0
	for _, infr := range cf.inFrameQueue {
		if oi >= len(cf.frameQueue) {
			break
		}

		matched := false
		for i := oi; i < len(cf.frameQueue); i++ {
			ofr := cf.frameQueue[i].frame

			// Basic frame-level matching criteria.
			if infr.Header.FrameLength() != ofr.Header.FrameLength() {
				continue
			}

			if len(infr.Datagrams) == 0 || len(ofr.Datagrams) == 0 {
				continue
			}

			if len(infr.Datagrams) != len(ofr.Datagrams) {
				continue
			}

			if infr.Datagrams[0].Header.Index != ofr.Datagrams[0].Header.Index {
				continue
			}

			// Match each datagram in the frame.
			for j, ocmd := range cf.frameQueue[i].cmds {
				odgram := ocmd.DatagramOut
				indgram := infr.Datagrams[j]

				if odgram.Header.Command != indgram.Header.Command {
					continue
				}

				if odgram.Header.DataLength() != indgram.Header.DataLength() {
					continue
				}

				ocmd.DatagramIn = indgram
				ocmd.Arrived = true
				ocmd.Overlayed = true
				ocmd.Error = nil
			}

			// Move past the matched frame and break to the next incoming frame.
			oi = i + 1
			matched = true
			break
		}

		if !matched {
			// Incoming frame did not match any outgoing frame.
			// The corresponding commands will have Arrived=false.
			break
		}
	}

	// Clear queues for the next cycle.
	cf.frameQueue = nil
	cf.inFrameQueue = nil

	return nil
}

// Close closes the CommandFramer, releasing resources.
func (cf *CommandFramer) Close() error {
	return nil
}

// DebugMessage sends a debug message to the underlying framer if it supports it.
func (cf *CommandFramer) DebugMessage(m string) {
	if dm, ok := cf.framer.(debugMessager); ok {
		dm.DebugMessage(m)
	}
}

// debugMessager is an optional interface for framers that support debug messages.
type debugMessager interface {
	DebugMessage(string)
}

// printDebugMessage sends a debug message to the given object if it supports it.
func printDebugMessage(p interface{}, m string) {
	if dm, ok := p.(debugMessager); ok {
		dm.DebugMessage(m)
	}
}
