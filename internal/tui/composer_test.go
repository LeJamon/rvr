package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// TestNewComposerPlaceholderUsesMutedShade guards the prompt-box placeholder: it
// must be shown (not blank) and rendered in the theme's muted shade rather than
// bubbles' hard-to-read dark-grey default (240), in both focus states.
func TestNewComposerPlaceholderUsesMutedShade(t *testing.T) {
	ta := newComposer()

	if ta.Placeholder == "" {
		t.Fatal("composer placeholder text should be set")
	}

	// colMuted is seeded from the default theme by init.
	for _, tc := range []struct {
		name string
		got  lipgloss.TerminalColor
	}{
		{"focused", ta.FocusedStyle.Placeholder.GetForeground()},
		{"blurred", ta.BlurredStyle.Placeholder.GetForeground()},
	} {
		if tc.got != colMuted {
			t.Errorf("%s placeholder foreground = %v, want muted %v", tc.name, tc.got, colMuted)
		}
		if tc.got == lipgloss.Color("240") {
			t.Errorf("%s placeholder still using bubbles' dark default (240)", tc.name)
		}
	}
}
