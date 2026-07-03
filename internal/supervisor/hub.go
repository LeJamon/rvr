package supervisor

import (
	"net"
	"sync"

	"xanax/internal/ringbuf"
	"xanax/internal/wire"
)

const (
	clientQueueDepth = 256
	outputChunkSize  = 32 * 1024
)

// client is one attached connection. A dedicated writer goroutine drains out,
// so broadcasters never block on a slow socket.
type client struct {
	conn net.Conn
	out  chan wire.Frame
	done chan struct{}
	once sync.Once
}

func newClient(conn net.Conn) *client {
	return &client{
		conn: conn,
		out:  make(chan wire.Frame, clientQueueDepth),
		done: make(chan struct{}),
	}
}

// enqueue queues a frame without blocking. It returns false if the client's
// queue is full (a slow consumer), signaling the caller to drop it.
func (cl *client) enqueue(f wire.Frame) bool {
	select {
	case cl.out <- f:
		return true
	case <-cl.done:
		return false
	default:
		return false
	}
}

func (cl *client) close() {
	cl.once.Do(func() {
		close(cl.done)
		cl.conn.Close()
	})
}

func (cl *client) writeLoop() {
	for {
		select {
		case f := <-cl.out:
			if err := wire.Write(cl.conn, f.Type, f.Payload); err != nil {
				cl.close()
				return
			}
		case <-cl.done:
			return
		}
	}
}

// hub fans PTY output and state frames out to attached clients. The ring write
// and every broadcast happen under mu so a newly registered client's snapshot
// is consistent with the live stream (no gaps, no duplication).
type hub struct {
	mu      sync.Mutex
	clients map[*client]struct{}
	ring    *ringbuf.Ring
	info    wire.Info
	state   wire.State
}

func newHub(ring *ringbuf.Ring, info wire.Info) *hub {
	return &hub{
		clients: make(map[*client]struct{}),
		ring:    ring,
		info:    info,
		state:   wire.State{Status: info.Status, Detail: info.Detail},
	}
}

// broadcastOutput records PTY bytes in the ring and forwards them live.
func (h *hub) broadcastOutput(p []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ring.Write(p)
	for _, chunk := range chunkBytes(p, outputChunkSize) {
		h.fanout(wire.Frame{Type: wire.TypeOutput, Payload: chunk})
	}
}

// broadcastState updates the latest state and forwards it.
func (h *hub) broadcastState(s wire.State) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state = s
	h.fanout(jsonFrame(wire.TypeState, s))
}

// broadcastExit forwards the terminal outcome and closes all clients.
func (h *hub) broadcastExit(e wire.Exit) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.fanout(jsonFrame(wire.TypeExit, e))
	for cl := range h.clients {
		delete(h.clients, cl)
		go func(c *client) { <-c.out; c.close() }(cl) // let the exit frame flush
	}
}

// clearScreen homes the cursor and clears the display. Sent on attach to a
// full-screen harness so it repaints cleanly (via the SIGWINCH that follows the
// client's resize) rather than showing a replay of stale frames.
var clearScreen = []byte("\x1b[2J\x1b[H")

// register admits a client: greet, prime the screen, send current state, then
// start receiving live frames. Held under mu so no output is missed.
//
// For a line-based harness the scrollback ring is replayed. For a full-screen
// TUI (altScreen) the ring holds interleaved cursor-addressed frames that
// garble when replayed, so the client's screen is cleared instead and the
// harness redraws on the next SIGWINCH.
func (h *hub) register(cl *client, altScreen bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	cl.enqueue(jsonFrame(wire.TypeHello, h.info))
	if altScreen {
		cl.enqueue(wire.Frame{Type: wire.TypeOutput, Payload: clearScreen})
	} else {
		for _, chunk := range chunkBytes(h.ring.Snapshot(), outputChunkSize) {
			cl.enqueue(wire.Frame{Type: wire.TypeOutput, Payload: chunk})
		}
	}
	cl.enqueue(jsonFrame(wire.TypeState, h.state))
	h.clients[cl] = struct{}{}
}

func (h *hub) remove(cl *client) {
	h.mu.Lock()
	delete(h.clients, cl)
	h.mu.Unlock()
	cl.close()
}

func (h *hub) clientCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

// fanout must be called with mu held.
func (h *hub) fanout(f wire.Frame) {
	for cl := range h.clients {
		if !cl.enqueue(f) {
			delete(h.clients, cl)
			cl.close()
		}
	}
}

func jsonFrame(t wire.Type, v any) wire.Frame {
	// Our control payloads always marshal; ignore the impossible error.
	f, _ := wire.MarshalFrame(t, v)
	return f
}

func chunkBytes(p []byte, size int) [][]byte {
	if len(p) == 0 {
		return nil
	}
	var out [][]byte
	for len(p) > size {
		out = append(out, p[:size])
		p = p[size:]
	}
	return append(out, p)
}
