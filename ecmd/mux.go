package ecmd

import (
	"context"
	"errors"
	"sync"
)

// Multiplexer provides goroutine-safe multiplexing of commands over a single
// Commander. Multiple muxChannel instances can be opened, each implementing
// the Commander interface. All channels' commands are collected and executed
// in a single Cycle() call to the underlying Commander.
type Multiplexer struct {
	c      Commander
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	reqChan chan muxRequest

	chans []*muxChanControlBlock
	mu    sync.Mutex

	cyclePending  bool
	cycleRespChan chan error
}

// NewMultiplexer creates a new Multiplexer and starts its background goroutine.
func NewMultiplexer(c Commander) (*Multiplexer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Multiplexer{
		c:       c,
		ctx:     ctx,
		cancel:  cancel,
		reqChan: make(chan muxRequest),
	}

	m.wg.Add(1)
	go m.loop()

	return m, nil
}

// muxRequest is the interface for all messages sent to the multiplexer loop.
type muxRequest interface {
	handle(m *Multiplexer)
}

// loop is the main event loop for the multiplexer.
func (m *Multiplexer) loop() {
	defer m.wg.Done()

	for {
		// Check if all channels are cycling and we have a pending cycle.
		m.mu.Lock()
		if m.cyclePending {
			allCycling := false
			for _, cb := range m.chans {
				if cb.commandsOpen && !cb.cycling {
					allCycling = false
					break
				}
				if cb.commandsOpen {
					allCycling = true
				}
			}

			if allCycling {
				err := m.c.Cycle()

				for _, cb := range m.chans {
					if cb.cycling {
						cb.muxChannel.cycleRespChan <- err
					}
					cb.cycling = false
					cb.commandsOpen = false
				}

				m.cyclePending = false
				m.cycleRespChan <- err
				m.cycleRespChan = nil
			}
		}

		pending := m.cyclePending
		m.mu.Unlock()

		if pending {
			select {
			case req := <-m.reqChan:
				req.handle(m)
			case <-m.ctx.Done():
				return
			}
		} else {
			select {
			case req := <-m.reqChan:
				req.handle(m)
			case <-m.ctx.Done():
				return
			}
		}
	}
}

// OpenCommander opens a new multiplexed channel that implements the Commander
// interface. Each channel can be used independently and concurrently.
func (m *Multiplexer) OpenCommander() (Commander, error) {
	req := &openCommanderReq{responseChan: make(chan openCommanderResp)}
	select {
	case m.reqChan <- req:
	case <-m.ctx.Done():
		return nil, errors.New("multiplexer closed")
	}
	resp := <-req.responseChan
	return resp.Commander, resp.err
}

// New creates a new ExecutingCommand on the multiplexer itself.
// This is a convenience method that delegates to the underlying Commander.
func (m *Multiplexer) New(datalen int) (*ExecutingCommand, error) {
	return m.c.New(datalen)
}

// Cycle triggers one multiplexed cycle. It waits for all channels to complete
// their New() calls, then executes the underlying Commander's Cycle().
func (m *Multiplexer) Cycle() error {
	req := &muxCycleReq{responseChan: make(chan error)}
	select {
	case m.reqChan <- req:
	case <-m.ctx.Done():
		return errors.New("multiplexer closed")
	}
	return <-req.responseChan
}

// Close closes the multiplexer and all its channels. It cancels the context,
// waits for the goroutine to finish, and closes the underlying Commander.
func (m *Multiplexer) Close() error {
	m.cancel()
	m.wg.Wait()
	return m.c.Close()
}

// ─── muxChannel (implements Commander) ────────────────────────────────────────

// muxChannel implements the Commander interface for a single multiplexed channel.
type muxChannel struct {
	mux             *Multiplexer
	newResponseChan chan muxChanNewResp
	cycleRespChan   chan error
}

// New creates a new ExecutingCommand on this channel.
func (mc *muxChannel) New(datalen int) (*ExecutingCommand, error) {
	req := &muxChanNewReq{
		muxChannel:   mc,
		datalen:      datalen,
		responseChan: make(chan muxChanNewResp, 1),
	}
	select {
	case mc.mux.reqChan <- req:
	case <-mc.mux.ctx.Done():
		return nil, errors.New("multiplexer closed")
	}
	resp := <-req.responseChan
	return resp.ExecutingCommand, resp.error
}

// Cycle signals that this channel is ready for the multiplexed cycle.
func (mc *muxChannel) Cycle() error {
	req := &muxChanCycleReq{
		muxChannel:   mc,
		responseChan: make(chan error, 1),
	}
	select {
	case mc.mux.reqChan <- req:
	case <-mc.mux.ctx.Done():
		return errors.New("multiplexer closed")
	}
	return <-req.responseChan
}

// Close closes this channel.
func (mc *muxChannel) Close() error {
	return nil
}

// DebugMessage passes a debug message through to the underlying commander.
func (mc *muxChannel) DebugMessage(msg string) {
	printDebugMessage(mc.mux.c, msg)
}

// ─── Request/Response types for the multiplexer loop ──────────────────────────

type muxChanNewReq struct {
	*muxChannel
	datalen      int
	responseChan chan muxChanNewResp
}

func (r *muxChanNewReq) handle(m *Multiplexer) {
	ec, err := m.c.New(r.datalen)
	r.responseChan <- muxChanNewResp{ec, err}

	m.mu.Lock()
	cb := m.getCB(r.muxChannel)
	cb.commandsOpen = true
	m.mu.Unlock()
}

type muxChanNewResp struct {
	*ExecutingCommand
	error
}

type muxChanCycleReq struct {
	*muxChannel
	responseChan chan error
}

func (r *muxChanCycleReq) handle(m *Multiplexer) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cb := m.getCB(r.muxChannel)
	if cb.cycling {
		r.responseChan <- errors.New("there already is a concurrent Cycle() pending on this mux channel")
		return
	}

	cb.cycling = true
	r.cycleRespChan = r.responseChan
}

type muxCycleReq struct {
	responseChan chan error
}

func (r *muxCycleReq) handle(m *Multiplexer) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cycleRespChan != nil {
		r.responseChan <- errors.New("there already is a concurrent Cycle() on this multiplexer")
		return
	}

	m.cyclePending = true
	m.cycleRespChan = r.responseChan
}

type openCommanderReq struct {
	responseChan chan openCommanderResp
}

func (r *openCommanderReq) handle(m *Multiplexer) {
	c := &muxChannel{
		mux:             m,
		newResponseChan: make(chan muxChanNewResp),
		cycleRespChan:   make(chan error),
	}

	cb := &muxChanControlBlock{
		muxChannel:   c,
		commandsOpen: false,
		cycling:      false,
	}

	m.mu.Lock()
	m.chans = append(m.chans, cb)
	m.mu.Unlock()

	r.responseChan <- openCommanderResp{c, nil}
}

type openCommanderResp struct {
	Commander Commander
	err       error
}

// ─── Helper ───────────────────────────────────────────────────────────────────

// muxChanControlBlock tracks the state of a single muxChannel.
type muxChanControlBlock struct {
	muxChannel   *muxChannel
	commandsOpen bool
	cycling      bool
}

// getCB returns the control block for the given muxChannel.
// Must be called with m.mu held.
func (m *Multiplexer) getCB(mc *muxChannel) *muxChanControlBlock {
	for _, cb := range m.chans {
		if cb.muxChannel == mc {
			return cb
		}
	}
	panic("missing mux chan control block")
}
