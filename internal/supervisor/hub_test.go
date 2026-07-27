package supervisor

import (
	"bytes"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/LeJamon/rvr/internal/ringbuf"
	"github.com/LeJamon/rvr/internal/wire"
)

func TestConfiguredFullScreenRetainsScrollbackCheckpoint(t *testing.T) {
	h := newHub(ringbuf.New(64), newScreen(40, 6), wire.Info{SessionID: "codex"})
	h.broadcastOutput([]byte(
		"\x1b[3J\x1b[?2026h" +
			"EARLIER-CHAT-LINE\r\n",
	))
	h.broadcastOutput([]byte("MORE-HISTORY\r\n\x1b[?2026l"))

	// Simulate enough cursor-addressed animation/diff output to evict the
	// transcript rebuild from the ordinary byte ring.
	for range 20 {
		h.broadcastOutput([]byte("\x1b[1;1H\x1b[2Kworking..."))
	}
	h.broadcastOutput([]byte("\x1b[3;1HCURRENT-FRAME"))
	if bytes.Contains(h.ring.Snapshot(), []byte("EARLIER-CHAT-LINE")) {
		t.Fatal("test setup did not evict the transcript checkpoint from the byte ring")
	}

	cl := newClient(nil)
	h.register(cl, true)
	var primer []byte
	for {
		frame := <-cl.out
		if frame.Type == wire.TypeOutput {
			primer = append(primer, frame.Payload...)
		}
		if frame.Type == wire.TypeState {
			break
		}
	}

	history := bytes.Index(primer, []byte("EARLIER-CHAT-LINE"))
	snapshot := bytes.LastIndex(primer, []byte("\x1b[2J"))
	if history < 0 {
		t.Fatalf("attach primer lost the retained chat history: %.200q", primer)
	}
	if snapshot < 0 || history > snapshot {
		t.Fatalf("chat history was not replayed before the current-screen snapshot: %.200q", primer)
	}
	if !bytes.Contains(primer[snapshot:], []byte("CURRENT-FRAME")) {
		t.Fatalf("current-screen snapshot missing after history replay: %.200q", primer)
	}
}

func TestBroadcastExitFlushesQueuedFramesInOrder(t *testing.T) {
	server, peer := net.Pipe()
	defer peer.Close()
	cl := newClient(server)
	go cl.writeLoop()
	h := &hub{clients: map[*client]struct{}{cl: {}}}
	if !cl.enqueue(wire.Frame{Type: wire.TypeOutput, Payload: []byte("before exit")}) {
		t.Fatal("enqueue output failed")
	}

	flushed := make(chan struct{})
	go func() {
		h.broadcastExitWithin(wire.Exit{Status: "completed", ExitCode: 0}, time.Second)
		close(flushed)
	}()
	select {
	case <-flushed:
		t.Fatal("broadcast returned before the peer read queued frames")
	case <-time.After(20 * time.Millisecond):
	}

	first, err := wire.Read(peer)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if first.Type != wire.TypeOutput || string(first.Payload) != "before exit" {
		t.Fatalf("first frame = (%v, %q), want queued output", first.Type, first.Payload)
	}
	last, err := wire.Read(peer)
	if err != nil {
		t.Fatalf("read exit: %v", err)
	}
	if last.Type != wire.TypeExit {
		t.Fatalf("last frame type = %v, want exit", last.Type)
	}
	var exit wire.Exit
	if err := last.DecodeJSON(&exit); err != nil {
		t.Fatalf("decode exit: %v", err)
	}
	if exit.Status != "completed" || exit.ExitCode != 0 {
		t.Fatalf("exit = %+v", exit)
	}
	select {
	case <-flushed:
	case <-time.After(time.Second):
		t.Fatal("broadcast did not finish after exit was read")
	}
	if _, err := wire.Read(peer); !errors.Is(err, io.EOF) {
		t.Fatalf("post-exit read error = %v, want EOF", err)
	}
}

func TestBroadcastExitBoundsStalledClient(t *testing.T) {
	server, peer := net.Pipe()
	defer peer.Close()
	cl := newClient(server)
	go cl.writeLoop()
	h := &hub{clients: map[*client]struct{}{cl: {}}}

	started := time.Now()
	h.broadcastExitWithin(wire.Exit{Status: "failed", ExitCode: 1}, 25*time.Millisecond)
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("stalled exit flush took %s", elapsed)
	}
	select {
	case <-cl.done:
	default:
		t.Fatal("stalled client was not closed")
	}
}

func TestRegisterAfterExitReceivesTerminalFrame(t *testing.T) {
	h := newHub(ringbuf.New(1024), newScreen(80, 24), wire.Info{SessionID: "late"})
	h.broadcastExitWithin(wire.Exit{Status: "completed"}, 25*time.Millisecond)

	server, peer := net.Pipe()
	defer peer.Close()
	cl := newClient(server)
	go cl.writeLoop()
	h.register(cl, false)
	for _, want := range []wire.Type{wire.TypeHello, wire.TypeState, wire.TypeExit} {
		frame, err := wire.Read(peer)
		if err != nil {
			t.Fatalf("read %v: %v", want, err)
		}
		if frame.Type != want {
			t.Fatalf("frame type = %v, want %v", frame.Type, want)
		}
	}
	if _, err := wire.Read(peer); !errors.Is(err, io.EOF) {
		t.Fatalf("post-exit read error = %v, want EOF", err)
	}
}
