package supervisor

import (
	"sort"
	"strconv"
)

// stickyModes are DEC private modes the harness enables on the client terminal
// and expects to persist for the session — mouse tracking, bracketed paste,
// focus reporting. vt10x does not track these, so the supervisor records them
// from the output stream and replays them to a newly attached client (whose
// terminal was reset on the previous detach).
var stickyModes = map[int]bool{
	1000: true, 1002: true, 1003: true, // mouse: click, drag, any-motion
	1006: true, 1015: true, // mouse encodings: SGR, urxvt
	2004: true, // bracketed paste
	1004: true, // focus reporting
}

// observeModes updates enabled from the private-mode set/reset sequences
// (ESC [ ? n[;n...] h|l) present in p.
func observeModes(enabled map[int]bool, p []byte) {
	for i := 0; i+3 < len(p); i++ {
		if p[i] != 0x1b || p[i+1] != '[' || p[i+2] != '?' {
			continue
		}
		j := i + 3
		for j < len(p) && (p[j] >= '0' && p[j] <= '9' || p[j] == ';') {
			j++
		}
		if j >= len(p) || (p[j] != 'h' && p[j] != 'l') {
			continue
		}
		set := p[j] == 'h'
		for _, n := range parseParams(p[i+3 : j]) {
			if stickyModes[n] {
				if set {
					enabled[n] = true
				} else {
					delete(enabled, n)
				}
			}
		}
		i = j
	}
}

// modeSequence returns the escape bytes that re-enable the currently sticky
// modes, in stable numeric order.
func modeSequence(enabled map[int]bool) []byte {
	if len(enabled) == 0 {
		return nil
	}
	nums := make([]int, 0, len(enabled))
	for n := range enabled {
		nums = append(nums, n)
	}
	sort.Ints(nums)
	var out []byte
	for _, n := range nums {
		out = append(out, "\x1b[?"...)
		out = append(out, strconv.Itoa(n)...)
		out = append(out, 'h')
	}
	return out
}

func parseParams(b []byte) []int {
	var out []int
	cur, has := 0, false
	for _, c := range b {
		switch {
		case c >= '0' && c <= '9':
			cur = cur*10 + int(c-'0')
			has = true
		case c == ';':
			if has {
				out = append(out, cur)
			}
			cur, has = 0, false
		}
	}
	if has {
		out = append(out, cur)
	}
	return out
}
