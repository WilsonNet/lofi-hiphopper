package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/mattn/go-runewidth"
)

// RGB is a truecolor value.
type RGB struct {
	R, G, B uint8
}

// Hex parses "#rrggbb".
func Hex(value string) RGB {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(value) != 6 {
		return RGB{}
	}
	var r, g, b uint8
	if _, err := fmt.Sscanf(value, "%02x%02x%02x", &r, &g, &b); err != nil {
		return RGB{}
	}
	return RGB{r, g, b}
}

func lerp(a, b RGB, t float64) RGB {
	t = math.Max(0, math.Min(1, t))
	return RGB{
		R: uint8(float64(a.R) + (float64(b.R)-float64(a.R))*t),
		G: uint8(float64(a.G) + (float64(b.G)-float64(a.G))*t),
		B: uint8(float64(a.B) + (float64(b.B)-float64(a.B))*t),
	}
}

func blend(base, overlay RGB, alpha float64) RGB {
	return lerp(base, overlay, alpha)
}

func shade(color RGB, factor float64) RGB {
	return RGB{
		R: uint8(math.Max(0, math.Min(255, float64(color.R)*factor))),
		G: uint8(math.Max(0, math.Min(255, float64(color.G)*factor))),
		B: uint8(math.Max(0, math.Min(255, float64(color.B)*factor))),
	}
}

// Style is a single cell's colors and weight.
type Style struct {
	FG   RGB
	BG   RGB
	Bold bool
}

// Cell is one terminal cell. Cont marks the second half of a wide rune.
type Cell struct {
	Ch   rune
	FG   RGB
	BG   RGB
	Bold bool
	Cont bool
}

// Canvas is a fixed-size cell buffer with ANSI rendering. Building the whole
// frame first keeps gradients and panels pixel-aligned.
type Canvas struct {
	W, H  int
	cells []Cell
	base  Style
}

// NewCanvas allocates a blank canvas filled with the supplied background.
func NewCanvas(width, height int, background RGB) *Canvas {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	c := &Canvas{
		W:    width,
		H:    height,
		base: Style{FG: Ink, BG: background},
	}
	c.cells = make([]Cell, width*height)
	for i := range c.cells {
		c.cells[i] = Cell{Ch: ' ', FG: Ink, BG: background}
	}
	return c
}

func (c *Canvas) index(x, y int) int { return y*c.W + x }

// Set writes a rune, preserving the cell's existing background.
func (c *Canvas) Set(x, y int, ch rune, fg RGB) {
	if c == nil || x < 0 || y < 0 || x >= c.W || y >= c.H {
		return
	}
	background := c.cells[c.index(x, y)].BG
	c.SetStyle(x, y, ch, Style{FG: fg, BG: background, Bold: false})
}

// SetStyle writes a fully styled rune.
func (c *Canvas) SetStyle(x, y int, ch rune, style Style) {
	if c == nil || x < 0 || y < 0 || x >= c.W || y >= c.H {
		return
	}
	index := c.index(x, y)
	if style.BG == (RGB{}) {
		style.BG = c.base.BG
	}
	c.cells[index] = Cell{Ch: ch, FG: style.FG, BG: style.BG, Bold: style.Bold}
}

// Fill paints a rectangle with a character and style.
func (c *Canvas) Fill(x, y, width, height int, ch rune, style Style) {
	if c == nil {
		return
	}
	for row := y; row < y+height; row++ {
		for column := x; column < x+width; column++ {
			c.SetStyle(column, row, ch, style)
		}
	}
}

// Text writes a string, clipping at the right edge. Wide runes occupy two
// cells and are tracked with a continuation marker.
func (c *Canvas) Text(x, y int, text string, style Style) int {
	if c == nil || y < 0 || y >= c.H {
		return x
	}
	for _, ch := range text {
		if ch == '\n' || ch == '\r' {
			continue
		}
		if ch == '\t' {
			ch = ' '
		}
		width := runewidth.RuneWidth(ch)
		if width == 0 {
			continue
		}
		if x+width > c.W {
			break
		}
		if x >= 0 {
			c.SetStyle(x, y, ch, style)
			if width == 2 {
				c.SetStyle(x+1, y, 0, style)
				c.cells[c.index(x+1, y)].Cont = true
			}
		}
		x += width
	}
	return x
}

// TextClipped writes text and appends an ellipsis if it does not fit.
func (c *Canvas) TextClipped(x, y, width int, text string, style Style) {
	if width <= 0 {
		return
	}
	text = Sanitize(text)
	clipped := Truncate(text, width)
	c.Text(x, y, clipped, style)
}

// Panel draws a rounded card and returns its interior geometry.
func (c *Canvas) Panel(x, y, width, height int, title, right string, style PanelStyle) (innerX, innerY, innerW, innerH int) {
	if c == nil || width < 4 || height < 3 {
		return x, y, max(0, width), max(0, height)
	}
	border := style.Border
	if border == (RGB{}) {
		border = BorderColor
	}
	background := style.Background
	if background == (RGB{}) {
		background = PanelBG
	}
	c.Fill(x, y, width, height, ' ', Style{FG: Ink, BG: background})

	c.Set(x, y, '╭', border)
	c.Set(x+width-1, y, '╮', border)
	c.Set(x, y+height-1, '╰', border)
	c.Set(x+width-1, y+height-1, '╯', border)
	for column := x + 1; column < x+width-1; column++ {
		c.Set(column, y, '─', border)
		c.Set(column, y+height-1, '─', border)
	}
	for row := y + 1; row < y+height-1; row++ {
		c.Set(x, row, '│', border)
		c.Set(x+width-1, row, '│', border)
	}

	if title != "" {
		title = Sanitize(strings.ToUpper(title))
		label := " " + Truncate(title, width-8) + " "
		c.Text(x+2, y, label, Style{FG: style.Title, BG: background, Bold: true})
	}
	if right != "" {
		right = Sanitize(right)
		label := " " + Truncate(right, width-8) + " "
		c.Text(x+width-1-runewidth.StringWidth(label), y, label, Style{FG: style.Right, BG: background})
	}
	return x + 2, y + 1, width - 4, height - 2
}

// Dim darkens every cell in place, used behind modal panels.
func (c *Canvas) Dim(factor float64) {
	if c == nil {
		return
	}
	factor = math.Max(0, math.Min(1, factor))
	for index := range c.cells {
		cell := &c.cells[index]
		cell.FG = shade(cell.FG, factor)
		cell.BG = shade(cell.BG, factor)
	}
}

// HLine draws a horizontal rule.
func (c *Canvas) HLine(x, y, width int, color RGB) {
	for column := x; column < x+width; column++ {
		c.Set(column, y, '─', color)
	}
}

// Render converts the cell buffer to ANSI. Consecutive cells with the same
// style share one escape sequence.
func (c *Canvas) Render() string {
	if c == nil || c.W <= 0 || c.H <= 0 {
		return ""
	}
	var builder strings.Builder
	builder.Grow(c.W * c.H * 3)
	var current Style
	styleSet := false
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			cell := c.cells[c.index(x, y)]
			if cell.Cont {
				continue
			}
			style := Style{FG: cell.FG, BG: cell.BG, Bold: cell.Bold}
			if !styleSet || style != current {
				builder.WriteString(ansiStyle(style))
				current = style
				styleSet = true
			}
			builder.WriteRune(cell.Ch)
		}
		builder.WriteString("\x1b[0m")
		styleSet = false
		if y < c.H-1 {
			builder.WriteByte('\n')
		}
	}
	return builder.String()
}

func ansiStyle(style Style) string {
	if style.Bold {
		return fmt.Sprintf("\x1b[0;1;38;2;%d;%d;%d;48;2;%d;%d;%dm",
			style.FG.R, style.FG.G, style.FG.B, style.BG.R, style.BG.G, style.BG.B)
	}
	return fmt.Sprintf("\x1b[0;38;2;%d;%d;%d;48;2;%d;%d;%dm",
		style.FG.R, style.FG.G, style.FG.B, style.BG.R, style.BG.G, style.BG.B)
}

// PanelStyle colors a card.
type PanelStyle struct {
	Border     RGB
	Background RGB
	Title      RGB
	Right      RGB
}

// Sanitize strips control characters and wide glyphs so text alignment stays
// predictable on every terminal. Wide glyphs are dropped rather than split.
func Sanitize(text string) string {
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.ReplaceAll(text, "\t", " ")
	var builder strings.Builder
	builder.Grow(len(text))
	previousSpace := false
	for _, ch := range text {
		if ch < 32 || ch == 127 {
			continue
		}
		if ch == ' ' {
			if !previousSpace {
				builder.WriteRune(ch)
				previousSpace = true
			}
			continue
		}
		if runewidth.RuneWidth(ch) != 1 {
			continue
		}
		builder.WriteRune(ch)
		previousSpace = false
	}
	return strings.TrimSpace(builder.String())
}

// Truncate cuts text to width cells, appending a single-cell ellipsis.
func Truncate(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if runewidth.StringWidth(text) <= width {
		return text
	}
	if width == 1 {
		return "…"
	}
	var builder strings.Builder
	used := 0
	for _, ch := range text {
		chWidth := runewidth.RuneWidth(ch)
		if chWidth == 0 {
			continue
		}
		if used+chWidth > width-1 {
			break
		}
		builder.WriteRune(ch)
		used += chWidth
	}
	return builder.String() + "…"
}

// PadRight pads text to an exact cell width.
func PadRight(text string, width int) string {
	text = Truncate(Sanitize(text), width)
	padding := width - runewidth.StringWidth(text)
	if padding < 0 {
		padding = 0
	}
	return text + strings.Repeat(" ", padding)
}

// Braille is a 2x4-dot-per-cell drawing surface for organic line work.
type Braille struct {
	canvas *Canvas
	X, Y   int
	W, H   int // pixel dimensions
	cells  []brailleCell
}

type brailleCell struct {
	bits uint8
	set  bool
	fg   RGB
}

func NewBraille(canvas *Canvas, x, y, width, height int) *Braille {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	return &Braille{
		canvas: canvas,
		X:      x,
		Y:      y,
		W:      width * 2,
		H:      height * 4,
		cells:  make([]brailleCell, width*height),
	}
}

var brailleBits = [4][2]uint8{
	{0x01, 0x08},
	{0x02, 0x10},
	{0x04, 0x20},
	{0x40, 0x80},
}

// Set lights one braille pixel in canvas cell coordinates.
func (b *Braille) Set(px, py int, fg RGB) {
	if b == nil || px < 0 || py < 0 || px >= b.W || py >= b.H {
		return
	}
	cx, cy := px/2, py/4
	index := cy*(b.W/2) + cx
	cell := &b.cells[index]
	cell.bits |= brailleBits[py%4][px%2]
	cell.set = true
	cell.fg = fg
}

// Line draws a Bresenham line in braille pixel space.
func (b *Braille) Line(x0, y0, x1, y1 int, fg RGB) {
	if b == nil {
		return
	}
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy
	for {
		b.Set(x0, y0, fg)
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

// ThickLine draws a three-pixel-wide line for a soft glow.
func (b *Braille) ThickLine(x0, y0, x1, y1 int, fg RGB) {
	b.Line(x0, y0, x1, y1, fg)
	b.Line(x0+1, y0, x1+1, y1, fg)
	b.Line(x0, y0+1, x1, y1+1, fg)
}

// Flush writes braille glyphs into the canvas.
func (b *Braille) Flush() {
	if b == nil {
		return
	}
	width := b.W / 2
	for cy := 0; cy < b.H/4; cy++ {
		for cx := 0; cx < width; cx++ {
			cell := b.cells[cy*width+cx]
			if !cell.set {
				continue
			}
			b.canvas.Set(b.X+cx, b.Y+cy, rune(0x2800)+rune(cell.bits), cell.fg)
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
