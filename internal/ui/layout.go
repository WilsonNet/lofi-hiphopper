package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/wilsonpsn/lofi-hiphopper/internal/detect"
)

// View renders the complete frame. The layout switches between a roomy
// two-column desk and a compact single-column stack.
func (m *Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	canvas := NewCanvas(m.width, m.height, Background)
	if m.width < 46 || m.height < 18 {
		m.drawTiny(canvas)
		return canvas.Render()
	}
	if m.width >= 92 && m.height >= 24 {
		m.drawWide(canvas)
	} else {
		m.drawCompact(canvas)
	}
	return canvas.Render()
}

func (m *Model) drawWide(canvas *Canvas) {
	margin := 2
	headerY := 1
	contentY := 4
	footerRuleY := m.height - 4
	statusY := m.height - 3
	keysY := m.height - 2
	contentH := footerRuleY - contentY

	m.drawHeader(canvas, margin, headerY, m.width-2*margin)
	m.drawRule(canvas, margin, 2, m.width-2*margin)

	leftW := clampInt(m.width*32/100, 34, 44)
	rightW := m.width - margin*2 - leftW - 2
	if rightW < 44 {
		leftW = m.width - margin*2 - 46
		rightW = 44
	}
	leftW = max(30, leftW)
	rightX := margin + leftW + 2

	nowH := clampInt(contentH/2, 8, 11)
	signalH := 0
	if contentH >= 27 {
		signalH = 6
	}
	radioH := contentH - nowH - 1
	if signalH > 0 {
		radioH -= signalH + 1
	}
	if radioH < 6 {
		signalH = 0
		nowH = 8
		radioH = contentH - nowH - 1
	}
	if radioH < 4 {
		m.drawCompact(canvas)
		return
	}

	m.drawRadioCard(canvas, margin, contentY, leftW, radioH)
	if signalH > 0 {
		m.drawSignalCard(canvas, margin, contentY+radioH+1, leftW, signalH)
	}
	m.drawNowCard(canvas, margin, contentY+contentH-nowH, leftW, nowH)
	m.drawVisualizerCard(canvas, rightX, contentY, rightW, contentH)
	m.drawFooter(canvas, margin, m.width-2*margin, statusY, keysY)
	m.drawRule(canvas, margin, footerRuleY, m.width-2*margin)
}

func (m *Model) drawCompact(canvas *Canvas) {
	margin := 1
	width := m.width - margin*2
	m.drawHeader(canvas, margin, 1, width)
	m.drawRule(canvas, margin, 2, width)

	nowH := 4
	short := m.height < 22
	if short {
		nowH = 3
	}
	m.drawCompactNow(canvas, margin, 4, width, nowH)

	vizY := 4 + nowH
	if !short {
		vizY++
	}
	footerRuleY := m.height - 4
	vizH := footerRuleY - vizY
	if vizH < 4 {
		vizH = 4
	}
	m.drawVisualizerCard(canvas, margin, vizY, width, vizH)
	m.drawFooter(canvas, margin, width, m.height-3, m.height-2)
	m.drawRule(canvas, margin, footerRuleY, width)
}

func (m *Model) drawTiny(canvas *Canvas) {
	if m.width < 28 || m.height < 8 {
		canvas.TextClipped(0, 0, m.width, "lofi hiphopper", Style{FG: Accent, BG: Background, Bold: true})
		if m.height > 1 {
			text, color := m.detectionSummary()
			canvas.TextClipped(0, m.height-1, m.width, text, Style{FG: color, BG: Background})
		}
		return
	}
	margin := 1
	m.drawHeader(canvas, margin, 0, m.width-2)
	m.drawRule(canvas, margin, 1, m.width-2)
	radio := ""
	if m.selected >= 0 && m.selected < len(m.radios) {
		radio = m.radios[m.selected].Name
	}
	canvas.TextClipped(margin, 3, m.width-2, "▸ "+radio, Style{FG: Ink, BG: Background, Bold: true})

	vizY := 4
	vizH := m.height - 7
	if vizH >= 3 {
		m.drawVisualizerCard(canvas, margin, vizY, m.width-2, vizH)
	}
	text, color := m.detectionSummary()
	canvas.TextClipped(margin, m.height-2, m.width-2, text, Style{FG: color, BG: Background})
	canvas.TextClipped(margin, m.height-1, m.width-2, "m mute · -/= vol · j/k · enter · h/l · q quit", Style{FG: Dim, BG: Background})
}

func (m *Model) drawHeader(canvas *Canvas, x, y, width int) {
	if width <= 0 {
		return
	}
	dotColor := m.statusColor()
	canvas.Set(x, y, '◉', pulseColor(dotColor, float64(m.tick)*0.13, 0.3))
	logoEnd := drawGradientText(canvas, x+2, y, "lofi hiphopper", Accent, Pink, Background, true)

	clock := m.now.Format("15:04")
	right := x + width
	clockX := right - len([]rune(clock))
	if clockX > logoEnd+3 {
		canvas.Text(clockX, y, clock, Style{FG: Dim, BG: Background})
	} else {
		clockX = right
	}
	status, statusColor := m.statusLabel()
	statusText := "● " + status
	statusX := clockX - 2 - len([]rune(statusText))
	if statusX > logoEnd+2 {
		canvas.Set(statusX, y, '●', statusColor)
		canvas.Text(statusX+2, y, status, Style{FG: Ink, BG: Background, Bold: true})
	} else {
		statusX = clockX
	}
	autoLabel := "AUTO-FILL ON"
	autoColor := Muted
	if !m.autofill {
		autoLabel = "AUTO-FILL OFF"
		autoColor = Dim
	}
	if width < 60 {
		autoLabel = "AUTO ON"
		if !m.autofill {
			autoLabel = "AUTO OFF"
		}
	}
	autoX := statusX - 2 - len([]rune(autoLabel))
	if autoX > logoEnd+4 {
		canvas.Text(autoX, y, autoLabel, Style{FG: autoColor, BG: Background, Bold: m.autofill})
	} else {
		autoX = statusX
	}

	notice, kind := m.noticeText()
	noticeColor := Muted
	if kind == noticeBad {
		noticeColor = Danger
	} else if kind == noticeGood {
		noticeColor = Success
	} else if kind == noticeInfo {
		noticeColor = Accent
	}
	if notice == "" {
		notice = "silence → sound"
		noticeColor = Dim
	}
	noticeStart := logoEnd + 3
	noticeEnd := min(autoX, statusX) - 3
	if noticeEnd-noticeStart >= 10 {
		canvas.TextClipped(noticeStart, y, noticeEnd-noticeStart, notice, Style{FG: noticeColor, BG: Background})
	}
}

func (m *Model) drawRule(canvas *Canvas, x, y, width int) {
	if width <= 0 {
		return
	}
	canvas.HLine(x, y, width, BorderColor)
	canvas.Set(x, y, '━', Accent)
	if width > 1 {
		canvas.Set(x+1, y, '━', lerp(Accent, BorderColor, 0.35))
	}
}

func (m *Model) drawRadioCard(canvas *Canvas, x, y, width, height int) {
	style := PanelStyle{Border: BorderColor, Background: PanelBG, Title: Accent, Right: Dim}
	innerX, innerY, innerW, innerH := canvas.Panel(x, y, width, height, "radio", "j/k · enter", style)
	if innerH <= 0 || innerW <= 0 {
		return
	}
	if len(m.radios) == 0 {
		canvas.TextClipped(innerX, innerY, innerW, "no radios configured", Style{FG: Danger, BG: PanelBG})
		return
	}

	visible := innerH
	start := 0
	if len(m.radios) > visible {
		start = m.selected - visible/2
		if start < 0 {
			start = 0
		}
		if start+visible > len(m.radios) {
			start = len(m.radios) - visible
		}
	}
	for row := 0; row < visible; row++ {
		index := start + row
		if index >= len(m.radios) {
			break
		}
		rowY := innerY + row
		selected := index == m.selected
		background := PanelBG
		if selected {
			background = PanelRaised
			canvas.Fill(innerX, rowY, innerW, 1, ' ', Style{FG: Ink, BG: background})
		}
		marker := ' '
		markerColor := Dim
		if selected {
			marker = '▸'
			markerColor = Accent
		}
		canvas.Set(innerX, rowY, marker, markerColor)

		dot := '○'
		dotColor := Dim
		if index == m.loaded {
			dot = '●'
			dotColor = Pink
			if m.state == stateSuspended {
				dot = '◐'
				dotColor = Peach
			}
			if m.state == stateTuning && index == m.current {
				dot = '◌'
				dotColor = Peach
			}
		} else if index == m.current && m.state != stateIdle {
			dot = '◐'
			dotColor = Muted
		}
		canvas.Set(innerX+2, rowY, dot, dotColor)
		nameColor := Ink
		if !selected {
			nameColor = Muted
		}
		canvas.TextClipped(innerX+4, rowY, max(4, innerW-5), m.radios[index].Name, Style{FG: nameColor, BG: background, Bold: selected})
	}

	if start > 0 {
		canvas.Set(innerX+innerW-1, innerY, '↑', Dim)
	}
	if start+visible < len(m.radios) {
		canvas.Set(innerX+innerW-1, innerY+visible-1, '↓', Dim)
	}
}

func (m *Model) drawSignalCard(canvas *Canvas, x, y, width, height int) {
	innerX, innerY, innerW, innerH := canvas.Panel(x, y, width, height, "signal", "", PanelStyle{
		Border: BorderColor, Background: PanelBG, Title: Mint, Right: Dim,
	})
	if innerH <= 0 {
		return
	}
	audioText, audioColor := m.activityShort()
	spectrumText, spectrumColor := "waiting", Muted
	if m.audioErr != nil {
		spectrumText, spectrumColor = "offline", Danger
	} else if m.state == statePlaying {
		spectrumText, spectrumColor = "live", Success
	} else if m.state == stateSuspended {
		spectrumText, spectrumColor = "held", Peach
	}
	playerText, playerColor := "idle", Muted
	if m.demo {
		playerText, playerColor = "demo synth", Violet
	} else if m.snap.Running {
		playerText, playerColor = fmt.Sprintf("mpv · %d%%", m.volume), Success
	}
	if m.muted {
		playerText, playerColor = "mpv · muted", Peach
		if m.demo {
			playerText = "demo · muted"
		}
	}
	autoText := "off"
	if m.autofill {
		autoText = "on"
	}

	rows := [][3]any{
		{"audio", audioText, audioColor},
		{"spectrum", spectrumText, spectrumColor},
		{"player", playerText, playerColor},
		{"auto-fill", autoText, Muted},
	}
	for row, item := range rows {
		if row >= innerH {
			break
		}
		canvas.Text(innerX, innerY+row, item[0].(string), Style{FG: Dim, BG: PanelBG})
		canvas.TextClipped(innerX+10, innerY+row, max(1, innerW-10), item[1].(string), Style{FG: item[2].(RGB), BG: PanelBG})
	}
}

func (m *Model) drawNowCard(canvas *Canvas, x, y, width, height int) {
	right := m.stateBadge()
	innerX, innerY, innerW, innerH := canvas.Panel(x, y, width, height, "now playing", right, PanelStyle{
		Border: BorderColor, Background: PanelBG, Title: Pink, Right: m.stateColor(),
	})
	if innerH <= 0 || innerW <= 0 {
		return
	}
	row := innerY
	title := m.nowTitle()
	canvas.TextClipped(innerX, row, innerW, title, Style{FG: Ink, BG: PanelBG, Bold: true})
	row++
	if row < innerY+innerH {
		subtitle := m.nowSubtitle()
		canvas.TextClipped(innerX, row, innerW, subtitle, Style{FG: Muted, BG: PanelBG})
		row++
	}
	row++
	if row < innerY+innerH {
		m.drawPlaybackStatus(canvas, innerX, row, innerW)
		row++
	}
	if row < innerY+innerH {
		m.drawProgress(canvas, innerX, row, innerW)
		row++
	}
	row++
	if row < innerY+innerH {
		m.drawVolume(canvas, innerX, row, innerW)
		row++
	}
	if row < innerY+innerH {
		hint := ""
		hintColor := Dim
		switch {
		case m.playerErr != nil:
			hint = m.playerErr.Error()
			hintColor = Danger
		case m.audioErr != nil:
			hint = "spectrum offline · playback continues"
			hintColor = Dim
		case m.state == stateRetrying:
			hint = "retrying in " + shortDuration(time.Until(m.retryAt)) + " · press r"
			hintColor = Peach
		case m.state == stateSuspended:
			hint = "held for " + m.pauseReason + " · auto-resumes"
			hintColor = Peach
		case m.state == stateIdle && m.ready:
			hint = "waiting for silence · auto-fill"
			hintColor = Dim
		}
		if hint != "" {
			canvas.TextClipped(innerX, row, innerW, hint, Style{FG: hintColor, BG: PanelBG})
		}
	}
}

func (m *Model) drawCompactNow(canvas *Canvas, x, y, width, height int) {
	innerX, innerY, innerW, innerH := canvas.Panel(x, y, width, height, "now", m.stateBadge(), PanelStyle{
		Border: BorderColor, Background: PanelBG, Title: Pink, Right: m.stateColor(),
	})
	if innerH <= 0 {
		return
	}
	indexText := fmt.Sprintf("%d/%d", m.selected+1, max(1, len(m.radios)))
	name := "no radio"
	if m.selected >= 0 && m.selected < len(m.radios) {
		name = m.radios[m.selected].Name
	}
	prefix := "▸ " + indexText + "  "
	vol := fmt.Sprintf("%d%%", m.volume)
	volColor := Muted
	if m.muted {
		vol = "MUTED"
		volColor = Danger
	}
	nameWidth := innerW - len([]rune(prefix)) - len(vol) - 2
	if nameWidth < 8 {
		nameWidth = max(1, innerW-len([]rune(prefix)))
		vol = ""
	}
	canvas.Text(innerX, innerY, prefix, Style{FG: Accent, BG: PanelBG, Bold: true})
	canvas.TextClipped(innerX+len([]rune(prefix)), innerY, nameWidth, name, Style{FG: Ink, BG: PanelBG, Bold: true})
	if vol != "" {
		drawRightText(canvas, innerX+innerW, innerY, vol, Style{FG: volColor, BG: PanelBG, Bold: m.muted})
	}
	if innerH > 1 {
		line := m.nowSubtitle()
		if line == "" {
			line, _ = m.detectionSummary()
		}
		if m.playerErr != nil {
			line = m.playerErr.Error()
		}
		canvas.TextClipped(innerX, innerY+1, innerW, line, Style{FG: Muted, BG: PanelBG})
	}
}

func (m *Model) drawVisualizerCard(canvas *Canvas, x, y, width, height int) {
	if width < 6 || height < 3 {
		return
	}
	right := fmt.Sprintf("%d/%d  h/l", m.viz.Mode+1, VisualizerCount)
	if width >= 64 {
		right = fmt.Sprintf("peak %s   %d/%d  h/l", formatHz(m.viz.PeakHz()), m.viz.Mode+1, VisualizerCount)
	}
	innerX, innerY, innerW, innerH := canvas.Panel(x, y, width, height, m.viz.Name(), right, PanelStyle{
		Border: BorderColor, Background: PanelBG, Title: Accent, Right: Muted,
	})
	if innerW <= 0 || innerH <= 0 {
		return
	}
	if innerW < 8 || innerH < 3 {
		canvas.TextClipped(innerX, innerY, innerW, "too small", Style{FG: Dim, BG: PanelBG})
		return
	}
	m.viz.Draw(canvas, innerX, innerY, innerW, innerH, DrawOptions{
		Playing: m.state == statePlaying || m.demo,
		Paused:  m.state == stateSuspended,
	})
	m.drawVisualizerOverlay(canvas, innerX, innerY, innerW, innerH)
}

func (m *Model) drawVisualizerOverlay(canvas *Canvas, x, y, width, height int) {
	if height < 6 {
		return
	}
	label := ""
	switch {
	case m.demo:
		return
	case m.state == stateTuning:
		label = "TUNING STREAM"
	case m.state == stateSuspended:
		label = "PAUSED · " + strings.ToUpper(m.pauseReason)
	case m.state == stateRetrying:
		label = "RECONNECTING"
	case m.state == statePlaying && m.audioErr != nil:
		label = "SPECTRUM OFFLINE"
	case m.state == statePlaying:
		return
	case !m.ready && m.probeErr != nil:
		label = "DETECTION OFFLINE"
	case !m.ready:
		label = "CHECKING AUDIO"
	default:
		label = "WAITING FOR SILENCE"
	}
	label = Sanitize(label)
	if label == "" || len([]rune(label))+6 > width {
		return
	}
	labelX := x + (width-len([]rune(label)))/2
	labelY := y + height/2
	color := blend(PanelBG, Muted, 0.75)
	canvas.Text(labelX, labelY, label, Style{FG: color, BG: PanelBG, Bold: true})
	dotColor := pulseColor(blend(PanelBG, Accent, 0.8), float64(m.tick)*0.16, 0.25)
	canvas.Set(labelX-2, labelY, '·', dotColor)
	canvas.Set(labelX+len([]rune(label))+1, labelY, '·', dotColor)
}

func (m *Model) drawPlaybackStatus(canvas *Canvas, x, y, width int) {
	label := m.playbackLabel()
	color := m.stateColor()
	canvas.Text(x, y, label, Style{FG: color, BG: PanelBG, Bold: true})
	elapsed := formatDuration(m.snap.Position)
	if m.demo {
		elapsed = formatDuration(time.Duration(m.tick) * animationInterval)
	}
	right := elapsed
	if !m.stream.Live && m.duration() > 0 {
		right = elapsed + " / " + formatDuration(m.duration())
	}
	if m.state != statePlaying && m.state != stateSuspended {
		right = ""
	}
	if m.stream.Live && elapsed == "0:00" && !m.demo {
		right = ""
	}
	if right != "" && len([]rune(label))+len([]rune(right))+2 <= width {
		drawRightText(canvas, x+width, y, right, Style{FG: Muted, BG: PanelBG})
	}
}

func (m *Model) drawProgress(canvas *Canvas, x, y, width int) {
	if width <= 0 {
		return
	}
	barWidth := width
	fraction := -1.0
	if !m.stream.Live && m.duration() > 0 {
		fraction = clamp01(float64(m.snap.Position) / float64(m.duration()))
	}
	if fraction < 0 {
		for i := 0; i < barWidth; i++ {
			canvas.Set(x+i, y, '─', shade(BorderColor, 1.5))
		}
		if m.state == statePlaying && m.stream.Live {
			// Live streams get a slow, luminous ticker rather than fake progress.
			head := int(float64(m.tick)*0.7) % max(1, barWidth)
			for offset := 0; offset < min(5, barWidth); offset++ {
				position := (head + offset) % barWidth
				canvas.Set(x+position, y, '━', lerp(Accent, Pink, float64(offset)/5))
			}
		}
		return
	}
	filled := int(math.Round(fraction * float64(barWidth)))
	for i := 0; i < barWidth; i++ {
		color := shade(BorderColor, 1.5)
		glyph := '─'
		if i < filled {
			glyph = '━'
			color = warmGradient(float64(i) / float64(max(1, barWidth-1)))
		}
		canvas.Set(x+i, y, glyph, color)
	}
	if filled > 0 && filled < barWidth {
		canvas.Set(x+filled, y, '●', Cream)
	}
}

func (m *Model) drawVolume(canvas *Canvas, x, y, width int) {
	label := "VOL"
	labelColor := Dim
	if m.muted {
		label = "MUTED"
		labelColor = Danger
	}
	canvas.Text(x, y, label, Style{FG: labelColor, BG: PanelBG, Bold: m.muted})
	barX := x + 6
	barWidth := min(14, max(4, width-12))
	filled := int(math.Round(float64(m.volume) / 100 * float64(barWidth)))
	for i := 0; i < barWidth; i++ {
		color := shade(BorderColor, 1.5)
		glyph := '░'
		if m.muted {
			color = shade(BorderColor, 1.2)
		} else if i < filled {
			color = warmGradient(0.35 + 0.55*float64(i)/float64(max(1, barWidth-1)))
			glyph = '█'
		}
		canvas.Set(barX+i, y, glyph, color)
	}
	percent := fmt.Sprintf("%d%%", m.volume)
	if m.muted {
		percent = ""
	}
	canvas.TextClipped(barX+barWidth+1, y, max(1, width-(barX+barWidth+1-x)), percent, Style{FG: Muted, BG: PanelBG})
}

func (m *Model) drawFooter(canvas *Canvas, x, width, statusY, keysY int) {
	text, color := m.detectionSummary()
	canvas.Set(x, statusY, '●', color)
	auto := "AUTO-FILL ON"
	if !m.autofill {
		auto = "AUTO-FILL OFF"
	}
	shortAuto := "AUTO ON"
	if !m.autofill {
		shortAuto = "AUTO OFF"
	}
	textWidth := max(1, width-2)
	if width >= 56 {
		textWidth = max(10, width-2-len([]rune(auto))-2)
	} else if width >= 40 {
		textWidth = max(8, width-2-len([]rune(shortAuto))-2)
	}
	canvas.TextClipped(x+2, statusY, textWidth, text, Style{FG: Ink, BG: Background})
	if width >= 56 {
		drawRightText(canvas, x+width, statusY, auto, Style{FG: Muted, BG: Background, Bold: m.autofill})
	} else if width >= 40 {
		drawRightText(canvas, x+width, statusY, shortAuto, Style{FG: Muted, BG: Background, Bold: m.autofill})
	}

	if width >= 108 {
		m.drawHints(canvas, x, keysY, width, []keyHint{
			{"m", "mute"}, {"- / =", "volume"}, {"j / k", "select"}, {"enter", "play"},
			{"h / l", "visualizer"}, {"a", "auto-fill"}, {"r", "retry"}, {"q", "quit"},
		})
	} else if width >= 78 {
		m.drawHints(canvas, x, keysY, width, []keyHint{
			{"m", "mute"}, {"-/+", "vol"}, {"j/k", "select"}, {"enter", "play"},
			{"h/l", "style"}, {"a", "auto"}, {"q", "quit"},
		})
	} else if width >= 56 {
		m.drawHints(canvas, x, keysY, width, []keyHint{
			{"m", "mute"}, {"-/+", "vol"}, {"j/k", ""}, {"enter", ""}, {"h/l", ""}, {"q", "quit"},
		})
	} else {
		m.drawHints(canvas, x, keysY, width, []keyHint{
			{"m", "mute"}, {"-/+", "vol"}, {"j/k", ""}, {"enter", ""}, {"h/l", ""}, {"q", ""},
		})
	}
}

type keyHint struct {
	key   string
	label string
}

func (m *Model) drawHints(canvas *Canvas, x, y, width int, hints []keyHint) {
	cursor := x
	for index, hint := range hints {
		keyWidth := len([]rune(hint.key))
		labelWidth := len([]rune(hint.label))
		needed := keyWidth + 1 + labelWidth
		if index > 0 {
			needed += 3
		}
		if cursor+needed > x+width {
			break
		}
		if index > 0 {
			canvas.Text(cursor, y, " · ", Style{FG: Dim, BG: Background})
			cursor += 3
		}
		canvas.Text(cursor, y, hint.key, Style{FG: Accent, BG: Background, Bold: true})
		cursor += keyWidth + 1
		canvas.Text(cursor, y, hint.label, Style{FG: Dim, BG: Background})
		cursor += labelWidth
	}
}

func (m *Model) nowTitle() string {
	if len(m.radios) == 0 {
		return "no radios configured"
	}
	if m.current >= 0 && m.current < len(m.radios) {
		return Sanitize(m.radios[m.current].Name)
	}
	return "nothing playing"
}

func (m *Model) nowSubtitle() string {
	if m.demo {
		return "simulated audio · all six visualizers live"
	}
	switch m.state {
	case stateTuning:
		return "resolving with yt-dlp…"
	case stateSuspended:
		return "held for " + m.pauseReason + " · resumes when silent"
	case stateRetrying:
		return "stream lost · retrying in " + shortDuration(time.Until(m.retryAt))
	case stateIdle:
		if m.playerErr != nil {
			return Sanitize(m.playerErr.Error())
		}
		return "listening for silence"
	}
	if title := Sanitize(m.stream.Title); title != "" {
		return title
	}
	return "YouTube · live"
}

func (m *Model) playbackLabel() string {
	switch m.state {
	case stateTuning:
		return "● TUNING"
	case statePlaying:
		if m.stream.Live {
			return "● LIVE"
		}
		return "● PLAYING"
	case stateSuspended:
		return "❚❚ HELD"
	case stateRetrying:
		return "↻ RETRYING"
	case stateIdle:
		return "· IDLE"
	default:
		return "· READY"
	}
}

func (m *Model) stateBadge() string {
	if m.demo {
		return "DEMO"
	}
	switch m.state {
	case stateTuning:
		return "TUNING"
	case statePlaying:
		if m.stream.Live {
			return "LIVE"
		}
		return "PLAYING"
	case stateSuspended:
		return "HELD"
	case stateRetrying:
		return "RETRY"
	case stateIdle:
		return "IDLE"
	}
	return "READY"
}

func (m *Model) stateColor() RGB {
	switch m.state {
	case statePlaying:
		return Success
	case stateTuning:
		return Peach
	case stateSuspended:
		return Peach
	case stateRetrying:
		return Danger
	case stateIdle:
		return Muted
	}
	return Ink
}

func (m *Model) statusLabel() (string, RGB) {
	if m.demo {
		return "DEMO", Violet
	}
	switch m.state {
	case stateTuning:
		return "TUNING", Peach
	case statePlaying:
		return "FILLING", Success
	case stateSuspended:
		return "HELD", Peach
	case stateRetrying:
		return "RETRYING", Danger
	case stateIdle:
		return "LISTENING", Muted
	}
	return "READY", Muted
}

func (m *Model) statusColor() RGB {
	_, color := m.statusLabel()
	return color
}

func (m *Model) activityShort() (string, RGB) {
	if m.demo {
		return "simulated", Violet
	}
	if m.probeErr != nil {
		return "offline", Danger
	}
	switch m.activity {
	case detect.Call:
		return joinApps(m.assessment.CallApps) + " call", Danger
	case detect.OtherAudio:
		return joinApps(m.assessment.AudioApps), Peach
	default:
		return "silence", Muted
	}
}

func (m *Model) detectionSummary() (string, RGB) {
	if m.demo {
		return "DEMO MODE · simulated spectrum", Violet
	}
	if m.probeErr != nil {
		return "AUDIO DETECTION OFFLINE · " + Sanitize(m.probeErr.Error()), Danger
	}
	switch m.activity {
	case detect.Call:
		return "CALL · " + joinApps(m.assessment.CallApps) + " · playback held", Danger
	case detect.OtherAudio:
		state := "yielding"
		if !m.autofill {
			state = "auto-fill off"
		}
		return "OTHER AUDIO · " + joinApps(m.assessment.AudioApps) + " · " + state, Peach
	default:
		if m.state == statePlaying {
			return "SILENCE · filling with lofi", Success
		}
		if m.autofill {
			return "SILENCE · auto-fill armed", Muted
		}
		return "SILENCE · auto-fill off", Dim
	}
}

func (m *Model) duration() time.Duration {
	if m.snap.Duration > 0 {
		return m.snap.Duration
	}
	return m.stream.Duration
}

func drawGradientText(canvas *Canvas, x, y int, text string, from, to, background RGB, bold bool) int {
	runes := []rune(text)
	for index, ch := range runes {
		t := float64(index) / float64(max(1, len(runes)-1))
		canvas.SetStyle(x+index, y, ch, Style{FG: lerp(from, to, t), BG: background, Bold: bold})
	}
	return x + len(runes)
}

func drawRightText(canvas *Canvas, right, y int, text string, style Style) int {
	text = Sanitize(text)
	x := right - len([]rune(text))
	if x < 0 {
		x = 0
	}
	canvas.Text(x, y, text, style)
	return x
}

func joinApps(apps []string) string {
	if len(apps) == 0 {
		return "app"
	}
	cleaned := make([]string, 0, len(apps))
	for _, app := range apps {
		app = Sanitize(app)
		if app != "" {
			cleaned = append(cleaned, app)
		}
	}
	if len(cleaned) == 0 {
		return "app"
	}
	if len(cleaned) > 2 {
		return strings.Join(cleaned[:2], ", ") + fmt.Sprintf(" +%d", len(cleaned)-2)
	}
	return strings.Join(cleaned, ", ")
}

func formatDuration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	seconds := int(duration.Seconds())
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	remainder := seconds % 60
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, remainder)
	}
	return fmt.Sprintf("%d:%02d", minutes, remainder)
}

func formatHz(hz float64) string {
	if hz <= 0 {
		return "—"
	}
	if hz >= 1000 {
		return fmt.Sprintf("%.1f kHz", hz/1000)
	}
	return fmt.Sprintf("%.0f Hz", hz)
}
