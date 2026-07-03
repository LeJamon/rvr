package supervisor

import (
	"bytes"
	"testing"
)

func TestSplitSafe(t *testing.T) {
	cases := []struct {
		name      string
		in        string
		wantEmit  string
		wantCarry string
	}{
		{"plain text", "hello world", "hello world", ""},
		{"complete CSI", "a\x1b[31mred", "a\x1b[31mred", ""},
		{"truecolor split mid-params", "x\x1b[38;2;255", "x", "\x1b[38;2;255"},
		{"CSI cut right after ESC[", "ok\x1b[", "ok", "\x1b["},
		{"bare trailing ESC", "text\x1b", "text", "\x1b"},
		{"complete OSC with BEL", "\x1b]0;title\x07after", "\x1b]0;title\x07after", ""},
		{"OSC missing terminator", "a\x1b]11;rgb:1e/1e/1e", "a", "\x1b]11;rgb:1e/1e/1e"},
		{"OSC with ST terminator", "\x1b]11;?\x1b\\done", "\x1b]11;?\x1b\\done", ""},
		{"OSC cut inside ST", "x\x1b]0;t\x1b", "x", "\x1b]0;t\x1b"},
		{"charset two-byte complete", "\x1b(Bx", "\x1b(Bx", ""},
		{"charset cut after intermediate", "ab\x1b(", "ab", "\x1b("},
		{"SS3 complete", "\x1bOPq", "\x1bOPq", ""},
		{"SS3 cut", "q\x1bO", "q", "\x1bO"},
		{"cursor pos complete", "\x1b[12;39H", "\x1b[12;39H", ""},
		{"cursor pos split", "z\x1b[12;39", "z", "\x1b[12;39"},
		{"utf8 complete", "héllo", "héllo", ""},
		{"utf8 split 2-byte", "h\xc3", "h", "\xc3"},
		{"utf8 split 3-byte", "a\xe2\x94", "a", "\xe2\x94"},
		{"utf8 split 4-byte", "a\xf0\x9f\x98", "a", "\xf0\x9f\x98"},
		{"empty", "", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			emit, carry := splitSafe([]byte(c.in))
			if string(emit) != c.wantEmit || string(carry) != c.wantCarry {
				t.Errorf("splitSafe(%q) = (%q, %q), want (%q, %q)",
					c.in, emit, carry, c.wantEmit, c.wantCarry)
			}
		})
	}
}

func TestSplitSafeCarryReassembles(t *testing.T) {
	full := []byte("A\x1b[38;2;255;255;255mB\x1b[12;39HC")
	// Feed byte by byte; concatenated emissions must equal the input, and no
	// emission may end inside a sequence.
	var out []byte
	var carry []byte
	for _, b := range full {
		data := append(append([]byte(nil), carry...), b)
		emit, rest := splitSafe(data)
		carry = append([]byte(nil), rest...)
		out = append(out, emit...)
	}
	out = append(out, carry...) // final flush
	if !bytes.Equal(out, full) {
		t.Errorf("reassembled %q != original %q", out, full)
	}
}

func TestSplitSafeOversizedSequenceFlushes(t *testing.T) {
	// A malformed "sequence" longer than maxCarry must not stall forever.
	junk := append([]byte("\x1b]"), bytes.Repeat([]byte("x"), maxCarry+10)...)
	emit, carry := splitSafe(junk)
	if len(emit) == 0 {
		t.Error("oversized unterminated sequence was not flushed")
	}
	if len(carry) != 0 {
		t.Errorf("carry should be empty after flush, got %d bytes", len(carry))
	}
}
