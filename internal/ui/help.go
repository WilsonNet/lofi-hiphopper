package ui

import (
	"os"
	"strings"

	"github.com/wilsonpsn/lofi-hiphopper/internal/config"
)

type helpSection struct {
	title   string
	actions []string
}

var helpSections = []helpSection{
	{title: "PLAYBACK", actions: []string{"play", "volume_up", "volume_down", "mute", "toggle_autofill", "retry"}},
	{title: "RADIO", actions: []string{"next_radio", "previous_radio"}},
	{title: "VISUALIZER", actions: []string{"next_visualizer", "previous_visualizer"}},
	{title: "SYSTEM", actions: []string{"help", "quit"}},
}

type helpLine struct {
	section bool
	note    bool
	label   string
	keys    string
}

// drawHelp paints a modal shortcut sheet over the dimmed interface.
func (m *Model) drawHelp(canvas *Canvas) {
	if canvas == nil || m.width < 24 || m.height < 8 {
		return
	}
	canvas.Dim(0.32)

	compact := m.height < 26
	var lines []helpLine
	for _, section := range helpSections {
		if !compact {
			lines = append(lines, helpLine{section: true, label: section.title})
		}
		for _, action := range section.actions {
			keys := m.keysFor(action)
			if len(keys) == 0 {
				continue
			}
			pretty := make([]string, len(keys))
			for index, key := range keys {
				pretty[index] = prettyKey(key)
			}
			lines = append(lines, helpLine{
				label: config.ShortcutLabel(action),
				keys:  strings.Join(pretty, "  /  "),
			})
		}
		if !compact {
			lines = append(lines, helpLine{})
		}
	}

	path := displayConfigPath(m.configPath)
	notes := []helpLine{
		{note: true, label: "edit bindings", keys: path},
		{note: true, label: "mouse", keys: "gestures are editable bindings too"},
	}
	lines = append(lines, notes...)

	width := min(78, m.width-4)
	if width < 34 {
		width = m.width - 2
	}
	height := min(len(lines)+2, m.height-2)
	if height < 7 {
		return
	}
	maxInner := height - 2
	truncated := false
	if len(lines) > maxInner {
		// Drop notes first; the shortcut table is the important part.
		lines = lines[:maxInner]
		truncated = true
	}
	x := (m.width - width) / 2
	y := (m.height - height) / 2

	innerX, innerY, innerW, innerH := canvas.Panel(x, y, width, height, "shortcuts", m.helpCloseHint(), PanelStyle{
		Border:     Accent,
		Background: PanelRaised,
		Title:      Cream,
		Right:      Muted,
	})
	if innerW <= 0 || innerH <= 0 {
		return
	}

	labelWidth := min(19, max(10, innerW/2))
	keyX := innerX + labelWidth + 2
	keyWidth := innerW - labelWidth - 2
	row := innerY
	for _, line := range lines {
		if row >= innerY+innerH {
			break
		}
		switch {
		case line.section:
			canvas.TextClipped(innerX, row, innerW, line.label, Style{FG: Mint, BG: PanelRaised, Bold: true})
			for column := innerX + len([]rune(line.label)) + 2; column < innerX+innerW; column += 2 {
				canvas.Set(column, row, '·', Dim)
			}
		case line.label == "" && line.keys == "":
			// spacer
		case line.note:
			canvas.TextClipped(innerX, row, max(1, innerW/3), line.label, Style{FG: Dim, BG: PanelRaised})
			canvas.TextClipped(innerX+max(10, innerW/3), row, innerW-max(10, innerW/3), line.keys, Style{FG: Muted, BG: PanelRaised})
		default:
			canvas.TextClipped(innerX, row, labelWidth, line.label, Style{FG: Ink, BG: PanelRaised})
			if keyWidth > 0 {
				canvas.TextClipped(keyX, row, keyWidth, line.keys, Style{FG: Accent, BG: PanelRaised})
			}
		}
		row++
	}
	if truncated {
		canvas.Set(innerX+innerW-1, innerY+innerH-1, '…', Dim)
	}
}

func (m *Model) keysFor(action string) []string {
	for _, shortcut := range m.shortcuts {
		if shortcut.Action == action {
			return shortcut.Keys
		}
	}
	return nil
}

func (m *Model) helpCloseHint() string {
	key := "?"
	if keys := m.keysFor("help"); len(keys) > 0 {
		key = prettyKey(keys[0])
	}
	return key + " / esc  close"
}

func prettyKey(key string) string {
	if key == "+" {
		return "+"
	}
	if key == " " {
		return "space"
	}
	parts := strings.Split(key, "+")
	for index, part := range parts {
		switch part {
		case "wheel_up":
			parts[index] = "wheel ↑"
		case "wheel_down":
			parts[index] = "wheel ↓"
		case "wheel_left":
			parts[index] = "wheel ←"
		case "wheel_right":
			parts[index] = "wheel →"
		case "up":
			parts[index] = "↑"
		case "down":
			parts[index] = "↓"
		case "left":
			parts[index] = "←"
		case "right":
			parts[index] = "→"
		}
	}
	return strings.Join(parts, "+")
}

func displayConfigPath(path string) string {
	if path == "" {
		return config.Path()
	}
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
