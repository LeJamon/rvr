package supervisor

import (
	"bytes"
	"testing"
)

func TestObserveModesTracksSticky(t *testing.T) {
	m := map[int]bool{}
	observeModes(m, []byte("\x1b[?1000h\x1b[?1006h\x1b[?2004h"))
	if !m[1000] || !m[1006] || !m[2004] {
		t.Fatalf("mouse/paste modes not tracked: %v", m)
	}
	// Disabling one clears just it.
	observeModes(m, []byte("\x1b[?1000l"))
	if m[1000] || !m[1006] {
		t.Errorf("disable did not clear 1000 only: %v", m)
	}
	// Non-sticky modes (25 = cursor visibility, 1049 = alt screen) are ignored.
	observeModes(m, []byte("\x1b[?25h\x1b[?1049h"))
	if m[25] || m[1049] {
		t.Errorf("non-sticky mode tracked: %v", m)
	}
}

func TestObserveModesMultiParam(t *testing.T) {
	m := map[int]bool{}
	observeModes(m, []byte("\x1b[?1002;1006h"))
	if !m[1002] || !m[1006] {
		t.Errorf("multi-param set not parsed: %v", m)
	}
}

func TestModeSequenceReenables(t *testing.T) {
	m := map[int]bool{2004: true, 1000: true}
	seq := modeSequence(m)
	if !bytes.Contains(seq, []byte("\x1b[?1000h")) || !bytes.Contains(seq, []byte("\x1b[?2004h")) {
		t.Errorf("mode sequence = %q", seq)
	}
	if modeSequence(map[int]bool{}) != nil {
		t.Error("empty mode set should produce no sequence")
	}
}

func TestModeSequenceStableOrder(t *testing.T) {
	// Numeric order is deterministic regardless of map iteration.
	got := modeSequence(map[int]bool{2004: true, 1000: true, 1006: true})
	want := []byte("\x1b[?1000h\x1b[?1006h\x1b[?2004h")
	if !bytes.Equal(got, want) {
		t.Errorf("mode sequence order = %q, want %q", got, want)
	}
}
