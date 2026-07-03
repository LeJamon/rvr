package supervisor

import "testing"

func TestUpdateAltScreen(t *testing.T) {
	cases := []struct {
		name  string
		cur   bool
		chunk string
		want  bool
	}{
		{"enter 1049", false, "\x1b[?1049hhello", true},
		{"enter 47", false, "\x1b[?47h", true},
		{"leave 1049", true, "bye\x1b[?1049l", false},
		{"no marker keeps state true", true, "just text", true},
		{"no marker keeps state false", false, "just text", false},
		{"enter then leave in one chunk", false, "\x1b[?1049hframe\x1b[?1049l", false},
		{"leave then enter in one chunk", true, "\x1b[?1049lgap\x1b[?1049h", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := updateAltScreen(c.cur, []byte(c.chunk)); got != c.want {
				t.Errorf("updateAltScreen(%v, %q) = %v, want %v", c.cur, c.chunk, got, c.want)
			}
		})
	}
}
