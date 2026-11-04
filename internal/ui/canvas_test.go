package ui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func plain(rendered string) string {
	return ansiPattern.ReplaceAllString(rendered, "")
}

func assertFrameSize(t *testing.T, rendered string, width, height int) {
	t.Helper()
	lines := strings.Split(plain(rendered), "\n")
	if len(lines) != height {
		t.Fatalf("rendered %d lines, want %d", len(lines), height)
	}
	for index, line := range lines {
		if got := runewidth.StringWidth(line); got != width {
			t.Fatalf("line %d width = %d, want %d: %q", index, got, width, line)
		}
	}
}

func TestHex(t *testing.T) {
	color := Hex("#12ab34")
	if color.R != 0x12 || color.G != 0xab || color.B != 0x34 {
		t.Fatalf("color = %+v", color)
	}
	if Hex("nope") != (RGB{}) {
		t.Fatal("invalid color should be zero")
	}
}

func TestCanvasRenderSize(t *testing.T) {
	canvas := NewCanvas(20, 5, Background)
	canvas.Text(1, 1, "hello world", Style{FG: Ink, BG: Background})
	canvas.Text(5, 2, "wide 世界", Style{FG: Accent, BG: Background})
	canvas.Set(0, 4, '█', Pink)
	rendered := canvas.Render()
	assertFrameSize(t, rendered, 20, 5)
	for index, line := range strings.Split(rendered, "\n") {
		if !strings.HasPrefix(line, "\x1b[") {
			t.Fatalf("line %d lost its style reset: %q", index, line)
		}
	}
	if !strings.Contains(plain(rendered), "hello world") {
		t.Fatal("text missing")
	}
}

func TestCanvasClips(t *testing.T) {
	canvas := NewCanvas(10, 3, Background)
	canvas.Text(-3, 0, "abcdef", Style{FG: Ink, BG: Background})
	canvas.Text(8, 1, "abcdef", Style{FG: Ink, BG: Background})
	canvas.Set(-1, -1, 'x', Pink)
	canvas.Set(100, 100, 'x', Pink)
	assertFrameSize(t, canvas.Render(), 10, 3)
}

func TestCanvasPanel(t *testing.T) {
	canvas := NewCanvas(40, 10, Background)
	innerX, innerY, innerW, innerH := canvas.Panel(1, 1, 38, 8, "radio", "3 / 6", PanelStyle{
		Border: BorderColor, Background: PanelBG, Title: Accent, Right: Muted,
	})
	if innerX != 3 || innerY != 2 || innerW != 34 || innerH != 6 {
		t.Fatalf("inner = %d,%d,%d,%d", innerX, innerY, innerW, innerH)
	}
	text := plain(canvas.Render())
	if !strings.Contains(text, "RADIO") || !strings.Contains(text, "3 / 6") {
		t.Fatalf("panel labels missing:\n%s", text)
	}
	if !strings.Contains(text, "╭") || !strings.Contains(text, "╯") {
		t.Fatal("panel corners missing")
	}
	assertFrameSize(t, canvas.Render(), 40, 10)
}

func TestTruncateAndPad(t *testing.T) {
	if got := Truncate("hello", 10); got != "hello" {
		t.Fatalf("short = %q", got)
	}
	if got := Truncate("hello world", 6); runewidth.StringWidth(got) != 6 || !strings.HasSuffix(got, "…") {
		t.Fatalf("truncated = %q", got)
	}
	if got := PadRight("hi", 5); got != "hi   " {
		t.Fatalf("padded = %q", got)
	}
	if got := Sanitize("a  b\nc🎵d"); got != "a b cd" {
		t.Fatalf("sanitize = %q", got)
	}
}

func TestBraille(t *testing.T) {
	canvas := NewCanvas(4, 2, Background)
	b := NewBraille(canvas, 0, 0, 4, 2)
	b.Set(0, 0, Pink)
	b.Set(1, 0, Mint)
	b.Set(0, 1, Accent)
	b.Set(2, 0, Blue)
	b.Flush()
	text := plain(canvas.Render())
	lines := strings.Split(text, "\n")
	if !strings.ContainsRune(lines[0], rune(0x2800+0x01+0x02+0x08)) {
		t.Fatalf("expected dots 1, 2 and 4, got %q", lines[0])
	}
	if !strings.ContainsRune(lines[0], rune(0x2800+0x01)) {
		t.Fatalf("expected a second braille cell, got %q", lines[0])
	}
}

func TestBrailleLineClipped(t *testing.T) {
	canvas := NewCanvas(8, 4, Background)
	b := NewBraille(canvas, 0, 0, 8, 4)
	b.ThickLine(-20, -20, 100, 100, Accent)
	b.Flush()
	assertFrameSize(t, canvas.Render(), 8, 4)
}

func TestBlend(t *testing.T) {
	canvas := NewCanvas(2, 1, Background)
	canvas.Set(0, 0, '█', Background)
	canvas.Blend(0, 0, '█', Cream, 0.5)
	canvas.Blend(1, 0, '█', Cream, 0.5)
	rendered := plain(canvas.Render())
	if !strings.Contains(rendered, "██") {
		t.Fatalf("render = %q", rendered)
	}
}

func TestResample(t *testing.T) {
	values := []float64{0, 0.5, 1}
	out := resample(values, 5)
	if len(out) != 5 || out[0] != 0 || out[4] != 1 || out[2] < 0.49 || out[2] > 0.51 {
		t.Fatalf("resample = %v", out)
	}
	if got := resample(nil, 3); len(got) != 3 {
		t.Fatalf("nil resample = %v", got)
	}
}

func BenchmarkCanvasRender(b *testing.B) {
	canvas := NewCanvas(140, 45, Background)
	for y := 0; y < 45; y++ {
		for x := 0; x < 140; x++ {
			canvas.Set(x, y, '█', warmGradient(float64(x+y)/184))
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = canvas.Render()
	}
}
