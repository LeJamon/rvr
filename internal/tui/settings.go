package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"xanax/internal/config"
)

// openSettings opens the keybindings editor over the current view. It is reached
// from the session context, so closing it simply returns there (the composer
// stays blurred) rather than refocusing the prompt box like the harness picker.
func (m model) openSettings() (tea.Model, tea.Cmd) {
	m.settingsOn = true
	m.settingsCapture = false
	m.settingsSearch = ""
	m.settingsInput.SetValue("")
	m.settingsIdx = 0
	m.settingsScroll = 0
	return m, m.settingsInput.Focus()
}

// closeSettings dismisses the editor without refocusing the composer.
func (m model) closeSettings() (tea.Model, tea.Cmd) {
	m.settingsOn = false
	m.settingsCapture = false
	m.settingsInput.Blur()
	return m, nil
}

// filteredActions returns the bindable actions whose name or description matches
// the (case-insensitive) search text. An empty search keeps them all.
func (m model) filteredActions() []config.KeyAction {
	all := m.deps.Cfg.Keys.Actions()
	if m.settingsSearch == "" {
		return all
	}
	q := strings.ToLower(m.settingsSearch)
	out := make([]config.KeyAction, 0, len(all))
	for _, a := range all {
		if strings.Contains(a.Name, q) || strings.Contains(strings.ToLower(a.Desc), q) {
			out = append(out, a)
		}
	}
	return out
}

// updateSettingsKey handles keys while the keybindings editor is open. In list
// mode the search box is focused (type to filter); ↑/↓ move the highlight,
// Confirm starts capturing a new key for the highlighted action, Cancel closes.
// In capture mode the next keypress becomes the binding (Esc aborts).
func (m model) updateSettingsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.settingsCapture {
		return m.captureKeyBinding(msg)
	}
	k := m.keys()
	switch {
	case keyMatches(k.Up, msg):
		m.moveSettings(-1)
		return m, nil
	case keyMatches(k.Down, msg):
		m.moveSettings(1)
		return m, nil
	case keyMatches(k.Confirm, msg):
		if len(m.filteredActions()) == 0 {
			return m, nil
		}
		m.settingsCapture = true // next key rebinds the highlighted action
		return m, nil
	case keyMatches(k.Cancel, msg):
		return m.closeSettings()
	}
	// Anything else filters the list; the highlight resets to the top match.
	var cmd tea.Cmd
	m.settingsInput, cmd = m.settingsInput.Update(msg)
	m.settingsSearch = m.settingsInput.Value()
	m.settingsIdx = 0
	m.settingsScroll = 0
	return m, cmd
}

// captureKeyBinding binds the highlighted action to the pressed key and persists
// it. Esc aborts the capture with no change. Rebinding replaces the action's
// keys with the single captured key; multi-key aliases stay editable in the file.
// (The quit key still quits rather than being captured — it is intercepted a
// level up in updateKey — so binding it must be done in the config file.)
func (m model) captureKeyBinding(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc { // reliable abort regardless of the cancel binding
		m.settingsCapture = false
		return m, nil
	}
	filtered := m.filteredActions()
	if m.settingsIdx >= len(filtered) {
		m.settingsCapture = false
		return m, nil
	}
	name := filtered[m.settingsIdx].Name
	newKey := canonKey(msg.String())
	m.settingsCapture = false

	if m.deps.ConfigPath == "" {
		m.status = "rebind failed: no config path"
		return m, nil
	}
	orig, origErr := os.ReadFile(m.deps.ConfigPath)
	if err := setKeyBindingInConfig(m.deps.ConfigPath, name, []string{newKey}); err != nil {
		m.status = "rebind failed: " + err.Error()
		return m, nil
	}
	cfg, err := config.Load(m.deps.ConfigPath)
	if err != nil {
		restoreConfig(m.deps.ConfigPath, orig, origErr)
		m.status = "rebind reload failed: " + err.Error()
		return m, nil
	}
	m.deps.Cfg = cfg // new bindings take effect immediately (keyMatches reads Cfg.Keys)
	m.status = fmt.Sprintf("bound %s to %s", name, keyLabel(newKey))
	return m, nil
}

// moveSettings moves the highlight by delta within the filtered list, scrolling
// the window so the selected row stays visible.
func (m *model) moveSettings(delta int) {
	n := len(m.filteredActions())
	if n == 0 {
		return
	}
	m.settingsIdx = min(max(m.settingsIdx+delta, 0), n-1)
	vis := m.visibleRows()
	if m.settingsIdx < m.settingsScroll {
		m.settingsScroll = m.settingsIdx
	} else if m.settingsIdx >= m.settingsScroll+vis {
		m.settingsScroll = m.settingsIdx - vis + 1
	}
	if m.settingsScroll < 0 {
		m.settingsScroll = 0
	}
}

// setKeyBindingInConfig writes `<action> = [<keys>]` under the [keys] table in
// the config file, replacing an existing line for that action or inserting one,
// and creating the [keys] table when absent. Everything else in the file — other
// bindings, tables, comments — is preserved. Reuses tomlStringArray for quoting.
func setKeyBindingInConfig(path, action string, keys []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	line := fmt.Sprintf("%s = %s", action, tomlStringArray(keys))
	lines := strings.Split(string(data), "\n")

	// Find the [keys] table header.
	start := -1
	for i, ln := range lines {
		if strings.TrimSpace(ln) == "[keys]" {
			start = i
			break
		}
	}
	if start < 0 {
		// No [keys] table yet — append one after the existing content.
		body := strings.TrimRight(string(data), "\n")
		var b strings.Builder
		if body != "" {
			b.WriteString(body)
			b.WriteString("\n\n")
		}
		b.WriteString("[keys]\n")
		b.WriteString(line)
		b.WriteString("\n")
		return os.WriteFile(path, []byte(b.String()), 0o600)
	}

	// The table runs to the next header (or EOF). Replace the action's line if it
	// is already there; the name+boundary check keeps a prefix like "quit" from
	// matching "quit_list".
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "[") {
			end = i
			break
		}
	}
	for i := start + 1; i < end; i++ {
		t := strings.TrimSpace(lines[i])
		if t == action || strings.HasPrefix(t, action+" ") || strings.HasPrefix(t, action+"=") {
			lines[i] = line
			return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o600)
		}
	}
	// Not present — insert right after the [keys] header.
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:start+1]...)
	out = append(out, line)
	out = append(out, lines[start+1:]...)
	return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o600)
}
