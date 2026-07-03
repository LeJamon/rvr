package attach

import "testing"

func TestFindDetach(t *testing.T) {
	const exit = 0x1c // ctrl+\
	cases := []struct {
		name    string
		data    []byte
		wantIdx int
		wantLen int
	}{
		{"nothing", []byte("hello"), -1, 0},
		{"exit key", []byte{'a', 'b', exit}, 2, 1},
		{"left arrow CSI", []byte{'x', 0x1b, '[', 'D'}, 1, 3},
		{"left arrow SS3", []byte{0x1b, 'O', 'D'}, 0, 3},
		{"right arrow is not detach", []byte{0x1b, '[', 'C'}, -1, 0},
		{"bare esc is not detach", []byte{0x1b}, -1, 0},
		{"earliest wins: arrow before exit", []byte{0x1b, '[', 'D', exit}, 0, 3},
		{"earliest wins: exit before arrow", []byte{exit, 0x1b, '[', 'D'}, 0, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			idx, length := findDetach(c.data, exit)
			if idx != c.wantIdx || length != c.wantLen {
				t.Errorf("findDetach(%v) = (%d,%d), want (%d,%d)", c.data, idx, length, c.wantIdx, c.wantLen)
			}
		})
	}
}
