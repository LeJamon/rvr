package supervisor

// PTY reads can split an escape sequence (or a UTF-8 rune) across two chunks.
// Existing clients reassemble fine — their terminals parse a continuous byte
// stream — but a client that attaches *between* two such chunks starts
// mid-sequence and prints the tail as literal text (";2;255;255;255m" garbage).
// splitSafe normalizes chunk boundaries so a broadcast never ends inside a
// sequence: the incomplete tail is carried into the next chunk.

// maxCarry bounds how long we wait for a sequence terminator; anything longer
// is treated as malformed output and flushed as-is rather than stalling.
const maxCarry = 8 * 1024

// splitSafe returns the longest prefix of p that does not end in the middle of
// an escape sequence or UTF-8 rune, and the remainder to prepend to the next
// chunk. p must itself start at a safe boundary (guaranteed by carrying).
func splitSafe(p []byte) (emit, carry []byte) {
	const (
		stGround   = iota
		stEsc      // just saw ESC
		stEscInter // ESC + intermediate bytes (0x20-0x2F), e.g. charset "ESC ( B"
		stCSI      // ESC [ ... until final byte 0x40-0x7E
		stSS3      // ESC O + exactly one byte
		stString   // OSC/DCS/SOS/PM/APC ... until BEL or ESC \
		stStringEsc
	)
	state := stGround
	seqStart := -1

	for i := range len(p) {
		b := p[i]
		switch state {
		case stGround:
			if b == 0x1b {
				seqStart = i
				state = stEsc
			}
		case stEsc:
			switch {
			case b == '[':
				state = stCSI
			case b == ']' || b == 'P' || b == 'X' || b == '^' || b == '_':
				state = stString
			case b == 'O':
				state = stSS3
			case b >= 0x20 && b <= 0x2f:
				state = stEscInter
			default: // two-byte sequence complete
				state, seqStart = stGround, -1
			}
		case stEscInter:
			if b < 0x20 || b > 0x2f { // final byte
				state, seqStart = stGround, -1
			}
		case stCSI:
			if b >= 0x40 && b <= 0x7e {
				state, seqStart = stGround, -1
			}
		case stSS3:
			state, seqStart = stGround, -1
		case stString:
			if b == 0x07 {
				state, seqStart = stGround, -1
			} else if b == 0x1b {
				state = stStringEsc
			}
		case stStringEsc:
			switch b {
			case '\\': // ST terminator
				state, seqStart = stGround, -1
			case 0x1b:
				// still a candidate ST start
			default:
				state = stString
			}
		}
	}

	cut := len(p)
	if seqStart >= 0 && len(p)-seqStart <= maxCarry {
		cut = seqStart
	}
	cut = utf8SafeCut(p, cut)
	return p[:cut], p[cut:]
}

// utf8SafeCut moves cut left past any incomplete trailing multibyte rune.
func utf8SafeCut(p []byte, cut int) int {
	// A UTF-8 rune is at most 4 bytes; look back at most 3.
	for back := 1; back <= 3 && back <= cut; back++ {
		b := p[cut-back]
		if b&0xc0 == 0x80 {
			continue // continuation byte, keep looking for the start
		}
		if b < 0x80 {
			return cut // ASCII, boundary is fine
		}
		// Multibyte start: complete only if its full length fits before cut.
		var need int
		switch {
		case b&0xe0 == 0xc0:
			need = 2
		case b&0xf0 == 0xe0:
			need = 3
		case b&0xf8 == 0xf0:
			need = 4
		default:
			return cut // invalid byte; let the terminal deal with it
		}
		if back < need {
			return cut - back // incomplete rune: cut before its start
		}
		return cut
	}
	return cut
}
