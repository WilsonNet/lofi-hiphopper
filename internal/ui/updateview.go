package ui

import (
	"strings"
)

// drawUpdateDialog paints the update confirmation and progress modal.
func (m *Model) drawUpdateDialog(canvas *Canvas) {
	if canvas == nil || !m.updateDialog {
		return
	}
	canvas.Dim(0.32)

	title := "update available"
	right := "y / n"
	if m.updating {
		title = "updating"
		right = "please wait"
	}
	width := min(68, m.width-4)
	if width < 38 {
		width = m.width - 2
	}
	height := 10
	if m.updateErr != nil {
		height = 12
	}
	if height > m.height-2 {
		height = m.height - 2
	}
	if width < 20 || height < 7 {
		return
	}
	x := (m.width - width) / 2
	y := (m.height - height) / 2
	innerX, innerY, innerW, innerH := canvas.Panel(x, y, width, height, title, right, PanelStyle{
		Border:     Success,
		Background: PanelRaised,
		Title:      Cream,
		Right:      Muted,
	})
	if innerW <= 0 || innerH <= 0 {
		return
	}
	row := innerY
	version := m.updateRelease.Version
	if version == "" {
		version = "a newer release"
	} else {
		version = "v" + strings.TrimPrefix(version, "v")
	}

	if m.updating {
		canvas.TextClipped(innerX, row, innerW, version+" is downloading.", Style{FG: Ink, BG: PanelRaised, Bold: true})
		row++
		if row < innerY+innerH {
			canvas.TextClipped(innerX, row, innerW, "Verifying the release checksum, then replacing the binary.", Style{FG: Muted, BG: PanelRaised})
			row++
		}
		if row < innerY+innerH {
			canvas.TextClipped(innerX, row, innerW, "The app restarts automatically when it is done.", Style{FG: Muted, BG: PanelRaised})
		}
		return
	}

	canvas.TextClipped(innerX, row, innerW, version+" is available.", Style{FG: Ink, BG: PanelRaised, Bold: true})
	row++
	if row < innerY+innerH {
		canvas.TextClipped(innerX, row, innerW, "Install it now? The binary is checksum-verified and the app restarts.", Style{FG: Muted, BG: PanelRaised})
		row++
	}
	if m.updateErr != nil {
		row++
		if row < innerY+innerH {
			canvas.TextClipped(innerX, row, innerW, "Update failed: "+Sanitize(m.updateErr.Error()), Style{FG: Danger, BG: PanelRaised})
			row++
		}
	}
	row++
	if row < innerY+innerH {
		canvas.Text(innerX, row, "[y]", Style{FG: Success, BG: PanelRaised, Bold: true})
		canvas.Text(innerX+4, row, "install now", Style{FG: Ink, BG: PanelRaised})
		canvas.Text(innerX+20, row, "[n]", Style{FG: Dim, BG: PanelRaised, Bold: true})
		canvas.Text(innerX+24, row, "not now", Style{FG: Muted, BG: PanelRaised})
	}
	if row+1 < innerY+innerH {
		canvas.TextClipped(innerX, row+1, innerW, "press y or n · esc cancels", Style{FG: Dim, BG: PanelRaised})
	}
}
