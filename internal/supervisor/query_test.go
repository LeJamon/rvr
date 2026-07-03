package supervisor

import (
	"bytes"
	"slices"
	"testing"
)

func TestTerminalQueryResponses(t *testing.T) {
	t.Run("cursor position reports size", func(t *testing.T) {
		got := terminalQueryResponses([]byte("\x1b[6n"), 40, 120)
		if !bytes.Equal(got, []byte("\x1b[40;120R")) {
			t.Errorf("got %q", got)
		}
	})
	t.Run("primary device attributes", func(t *testing.T) {
		got := terminalQueryResponses([]byte("prefix\x1b[csuffix"), 24, 80)
		if !bytes.Contains(got, []byte("\x1b[?1;2c")) {
			t.Errorf("got %q", got)
		}
	})
	t.Run("da1 explicit zero", func(t *testing.T) {
		got := terminalQueryResponses([]byte("\x1b[0c"), 24, 80)
		if !bytes.Contains(got, []byte("\x1b[?1;2c")) {
			t.Errorf("got %q", got)
		}
	})
	t.Run("background color", func(t *testing.T) {
		got := terminalQueryResponses([]byte("\x1b]11;?\x07"), 24, 80)
		if !bytes.Contains(got, []byte("\x1b]11;rgb:")) {
			t.Errorf("got %q", got)
		}
	})
	t.Run("no query no response", func(t *testing.T) {
		if got := terminalQueryResponses([]byte("plain output"), 24, 80); len(got) != 0 {
			t.Errorf("unexpected response %q", got)
		}
	})
	t.Run("combined queries", func(t *testing.T) {
		got := terminalQueryResponses([]byte("\x1b[6n\x1b[c"), 10, 20)
		if !bytes.Contains(got, []byte("\x1b[10;20R")) || !bytes.Contains(got, []byte("\x1b[?1;2c")) {
			t.Errorf("got %q", got)
		}
	})
	t.Run("da1 does not falsely trigger da2", func(t *testing.T) {
		got := terminalQueryResponses([]byte("\x1b[c"), 24, 80)
		if bytes.Contains(got, []byte("\x1b[>0;10;1c")) {
			t.Errorf("DA1 query triggered a DA2 response: %q", got)
		}
	})
	t.Run("synchronized output and kitty keyboard", func(t *testing.T) {
		got := terminalQueryResponses([]byte("\x1b[?2026$p\x1b[?u"), 24, 80)
		if !bytes.Contains(got, []byte("\x1b[?2026;2$y")) || !bytes.Contains(got, []byte("\x1b[?0u")) {
			t.Errorf("got %q", got)
		}
	})
}

func TestWithStableTerm(t *testing.T) {
	in := []string{"PATH=/bin", "TERM=xterm-ghostty", "COLORTERM=truecolor", "HOME=/home/x"}
	out := withStableTerm(in)

	var terms, colorterms int
	for _, e := range out {
		if e == "TERM=xterm-256color" {
			terms++
		}
		if e == "COLORTERM=truecolor" {
			colorterms++
		}
		if e == "TERM=xterm-ghostty" {
			t.Error("ghostty TERM was not overridden")
		}
	}
	if terms != 1 {
		t.Errorf("want exactly one stable TERM, got %d", terms)
	}
	if colorterms != 1 {
		t.Errorf("want exactly one COLORTERM, got %d", colorterms)
	}
	// Unrelated vars are preserved.
	if !slices.Contains(out, "PATH=/bin") || !slices.Contains(out, "HOME=/home/x") {
		t.Errorf("unrelated env vars were dropped: %v", out)
	}
}
