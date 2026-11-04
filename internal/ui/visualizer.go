package ui

import (
	"math"

	"github.com/wilsonpsn/lofi-hiphopper/internal/audio"
)

// Visualizer modes. h/l cycles them without an enter key.
const (
	ModeBars = iota
	ModeMirror
	ModeScope
	ModeHalo
	ModeAurora
	ModeMatrix
	VisualizerCount
)

var visualizerNames = []string{"BARS", "MIRROR", "SCOPE", "HALO", "AURORA", "MATRIX"}

var blockRunes = []rune("▁▂▃▄▅▆▇█")

// DrawOptions tells the visualizer whether live audio is expected.
type DrawOptions struct {
	Playing bool
	Paused  bool
}

// Visualizer owns the smoothed spectrum and all six renderers.
type Visualizer struct {
	Mode       int
	Tick       int
	Bands      []float64
	Peaks      []float64
	Reflection []float64
	Wave       []float64
	PrevWave   []float64
}

// NewVisualizer returns a calm, zeroed visualizer.
func NewVisualizer() *Visualizer {
	return &Visualizer{
		Bands:      make([]float64, audio.BandCount),
		Peaks:      make([]float64, audio.BandCount),
		Reflection: make([]float64, audio.BandCount),
		Wave:       make([]float64, audio.WaveCount),
		PrevWave:   make([]float64, audio.WaveCount),
	}
}

// Next cycles forward. No confirmation needed.
func (v *Visualizer) Next() {
	v.Mode = (v.Mode + 1) % VisualizerCount
}

// Prev cycles backward.
func (v *Visualizer) Prev() {
	v.Mode = (v.Mode - 1 + VisualizerCount) % VisualizerCount
}

// SetMode selects one of the six modes by index.
func (v *Visualizer) SetMode(mode int) {
	if mode >= 0 && mode < VisualizerCount {
		v.Mode = mode
	}
}

// Name is the display name of the current mode.
func (v *Visualizer) Name() string {
	if v.Mode < 0 || v.Mode >= len(visualizerNames) {
		return visualizerNames[0]
	}
	return visualizerNames[v.Mode]
}

// Advance smooths toward the latest frame. Nil or empty targets decay.
func (v *Visualizer) Advance(bands, wave []float64) {
	v.Tick++
	target := resample(bands, audio.BandCount)
	targetWave := resampleSigned(wave, audio.WaveCount)

	for i := range v.Bands {
		goal := target[i]
		rate := 0.10
		if goal > v.Bands[i] {
			rate = 0.55
		}
		v.Bands[i] += (goal - v.Bands[i]) * rate
		if v.Bands[i] < 0.0005 {
			v.Bands[i] = 0
		}
		if v.Bands[i] > v.Peaks[i] {
			v.Peaks[i] = v.Bands[i]
		} else {
			v.Peaks[i] -= 0.010
			if v.Peaks[i] < v.Bands[i] {
				v.Peaks[i] = v.Bands[i]
			}
			if v.Peaks[i] < 0 {
				v.Peaks[i] = 0
			}
		}
		v.Reflection[i] += (v.Bands[i] - v.Reflection[i]) * 0.075
	}

	for i := range v.Wave {
		goal := targetWave[i]
		rate := 0.22
		if math.Abs(goal) > math.Abs(v.Wave[i]) {
			rate = 0.65
		}
		v.Wave[i] += (goal - v.Wave[i]) * rate
	}
	copy(v.PrevWave, v.Wave)
}

// PeakBand returns the loudest current band.
func (v *Visualizer) PeakBand() int {
	best := 0
	for i, value := range v.Bands {
		if value > v.Bands[best] {
			best = i
		}
	}
	return best
}

// PeakHz returns the centre frequency of the loudest band.
func (v *Visualizer) PeakHz() float64 {
	if v.MaxBand() < 0.02 {
		return 0
	}
	return audio.BandFrequency(v.PeakBand(), len(v.Bands))
}

// MaxBand returns the strongest smoothed value.
func (v *Visualizer) MaxBand() float64 {
	maximum := 0.0
	for _, value := range v.Bands {
		if value > maximum {
			maximum = value
		}
	}
	return maximum
}

// Draw paints the current mode into a rectangle.
func (v *Visualizer) Draw(canvas *Canvas, x, y, width, height int, options DrawOptions) {
	if canvas == nil || width < 6 || height < 4 {
		return
	}
	now := float64(v.Tick) / 30.0
	artH := height
	if v.Mode != ModeHalo && height >= 9 && width >= 34 {
		artH = height - 1
		drawRuler(canvas, x, y+height-1, width)
	}

	switch v.Mode {
	case ModeBars:
		v.drawBars(canvas, x, y, width, artH, now, options)
	case ModeMirror:
		v.drawMirror(canvas, x, y, width, artH, now, options)
	case ModeScope:
		v.drawScope(canvas, x, y, width, artH, now, options)
	case ModeHalo:
		v.drawHalo(canvas, x, y, width, artH, now, options)
	case ModeAurora:
		v.drawAurora(canvas, x, y, width, artH, now, options)
	case ModeMatrix:
		v.drawMatrix(canvas, x, y, width, artH, now, options)
	}
}

func (v *Visualizer) drawBars(canvas *Canvas, x, y, width, height int, now float64, options DrawOptions) {
	if height < 1 {
		return
	}
	rows := height
	columns := columnCount(width)
	values := resample(v.Bands, columns)
	peaks := resample(v.Peaks, columns)
	base := y + height - 1
	pulse := int(now*10) % max(1, width)

	for i := 0; i < columns; i++ {
		slotStart := x + i*width/columns
		slotEnd := x + (i+1)*width/columns
		barWidth := max(1, slotEnd-slotStart-1)
		if slotEnd-slotStart <= 2 {
			barWidth = 1
		}
		frequencyT := float64(i) / float64(max(1, columns-1))
		value := displayLevel(values[i])
		if !options.Playing && !options.Paused {
			value = 0
		}
		units := value * float64(rows) * 8

		for barX := slotStart; barX < slotStart+barWidth && barX < x+width; barX++ {
			for row := 0; row < rows; row++ {
				remaining := units - float64(row*8)
				if remaining <= 0 {
					break
				}
				heightT := float64(row+1) / float64(max(1, rows))
				color := warmGradient(0.65*frequencyT + 0.35*heightT)
				glyph := '█'
				if remaining < 8 {
					glyph = blockRunes[max(0, int(remaining)-1)]
					color = lerp(color, Cream, 0.25)
				}
				canvas.Set(barX, base-row, glyph, color)
			}
		}

		peak := displayLevel(peaks[i])
		if peak > 0.02 && rows > 0 {
			peakRow := min(rows-1, int(math.Round(peak*float64(rows)))-1)
			if peakRow >= 0 {
				canvas.Set(slotStart, base-peakRow, '▔', pulseColor(Cream, now*2, 0.3))
			}
		}
		// A faint floor dot keeps the frequency axis readable during silence.
		if units <= 0 || options.Paused || !options.Playing {
			color := Dim
			if abs(slotStart-pulse) < 3 {
				color = Accent
			}
			canvas.Set(slotStart, base, '·', color)
		}
	}
}

func (v *Visualizer) drawMirror(canvas *Canvas, x, y, width, height int, now float64, options DrawOptions) {
	if height < 4 {
		v.drawBars(canvas, x, y, width, height, now, options)
		return
	}
	axis := y + height/2
	half := min(axis-y, y+height-1-axis)
	columns := columnCount(width)
	values := resample(v.Bands, columns)
	reflection := resample(v.Reflection, columns)
	peaks := resample(v.Peaks, columns)

	for i := 0; i < columns; i++ {
		slotStart := x + i*width/columns
		slotEnd := x + (i+1)*width/columns
		barWidth := max(1, slotEnd-slotStart-1)
		if slotEnd-slotStart <= 2 {
			barWidth = 1
		}
		frequencyT := float64(i) / float64(max(1, columns-1))
		value := displayLevel(values[i])
		reflected := displayLevel(reflection[i]) * 0.85
		if !options.Playing && !options.Paused {
			value, reflected = 0, 0
		}
		upUnits := value * float64(half) * 8
		downUnits := reflected * float64(half) * 8

		for barX := slotStart; barX < slotStart+barWidth && barX < x+width; barX++ {
			for row := 0; row < half; row++ {
				remaining := upUnits - float64(row*8)
				if remaining > 0 {
					heightT := float64(row+1) / float64(max(1, half))
					color := warmGradient(0.55*frequencyT + 0.45*heightT)
					glyph := '█'
					if remaining < 8 {
						glyph = blockRunes[max(0, int(remaining)-1)]
						color = lerp(color, Cream, 0.2)
					}
					canvas.Set(barX, axis-row-1, glyph, color)
				}
				remaining = downUnits - float64(row*8)
				if remaining > 0 {
					heightT := float64(row+1) / float64(max(1, half))
					color := lerp(auroraGradient(frequencyT), PanelBG, 0.25+0.25*heightT)
					glyph := '█'
					if remaining < 8 {
						glyph = blockRunes[max(0, int(remaining)-1)]
					}
					canvas.Set(barX, axis+row+1, glyph, color)
				}
			}
		}

		peak := displayLevel(peaks[i])
		if peak > 0.02 && half > 0 {
			peakRow := min(half-1, int(math.Round(peak*float64(half)))-1)
			if peakRow >= 0 {
				canvas.Set(slotStart, axis-peakRow-1, '▔', pulseColor(Pink, now*2, 0.25))
			}
		}
	}
	for column := x; column < x+width; column += 2 {
		canvas.Set(column, axis, '·', Dim)
	}
	if !options.Playing {
		position := int(now*18) % max(1, width)
		canvas.Set(x+position, axis, '●', Accent)
	}
}

func (v *Visualizer) drawScope(canvas *Canvas, x, y, width, height int, now float64, options DrawOptions) {
	if height < 3 {
		return
	}
	b := NewBraille(canvas, x, y, width, height)
	center := b.H / 2
	pixelW := b.W
	if pixelW <= 1 {
		return
	}

	// Ghost spectrum silhouette anchors the waveform to frequencies.
	columns := max(4, width)
	bands := resample(v.Bands, columns)
	for i, value := range bands {
		barHeight := int(value * float64(height) * 0.32)
		for row := 0; row < barHeight; row++ {
			column := x + i*width/columns
			canvas.Set(column, y+height-1-row, '░', blend(PanelBG, Violet, 0.22))
		}
	}

	// Previous frame, dim and ghostly.
	drawWaveLine(b, smoothCurve(v.PrevWave, 3), pixelW, center, height, now, func(t float64) RGB {
		return blend(PanelBG, Blue, 0.28)
	})
	// Current frame, glowing.
	drawWaveLine(b, smoothCurve(v.Wave, 3), pixelW, center, height, now, func(t float64) RGB {
		return warmGradient(t)
	})

	if !options.Playing {
		for px := 0; px < pixelW; px++ {
			value := math.Sin(float64(px)*0.055+now*2.2) * 0.035
			py := center - int(value*float64(height*4)*0.7)
			b.Set(px, py, blend(PanelBG, Accent, 0.55))
		}
	}

	for column := x; column < x+width; column += 4 {
		canvas.Set(column, y+height/2, '·', blend(PanelBG, Dim, 0.7))
	}
	b.Flush()
}

func drawWaveLine(b *Braille, wave []float64, pixelW, center, height int, now float64, color func(float64) RGB) {
	if len(wave) == 0 {
		return
	}
	previousY := 0
	for px := 0; px < pixelW; px++ {
		index := px * len(wave) / pixelW
		value := wave[index]
		amplitude := value * 2.15
		amplitude = math.Max(-1, math.Min(1, amplitude))
		py := center - int(amplitude*float64(height*4)*0.42)
		py = max(2, min(b.H-3, py))
		if px == 0 {
			previousY = py
			continue
		}
		t := float64(px) / float64(pixelW)
		b.ThickLine(px-1, previousY, px, py, color(t))
		previousY = py
	}
}

func (v *Visualizer) drawHalo(canvas *Canvas, x, y, width, height int, now float64, options DrawOptions) {
	if height < 5 {
		v.drawBars(canvas, x, y, width, height, now, options)
		return
	}
	b := NewBraille(canvas, x, y, width, height)
	cx, cy := b.W/2, b.H/2
	radius := float64(min(b.W, b.H)) * 0.44
	inner := radius * 0.34
	if inner < 3 {
		inner = 3
	}

	drawCircle(b, cx, cy, int(radius), blend(PanelBG, BorderColor, 0.8))
	drawCircle(b, cx, cy, int(inner), blend(PanelBG, Violet, 0.45))

	rays := 96
	values := resample(v.Bands, rays)
	bass := 0.0
	for i := 0; i < min(8, len(v.Bands)); i++ {
		bass += v.Bands[i]
	}
	bass /= float64(max(1, min(8, len(v.Bands))))
	for i := 0; i < rays; i++ {
		angle := -math.Pi/2 + 2*math.Pi*float64(i)/float64(rays)
		value := values[i]
		if !options.Playing && !options.Paused {
			value = 0
		}
		length := inner + value*(radius-inner)
		x0 := cx + int(math.Cos(angle)*inner)
		y0 := cy + int(math.Sin(angle)*inner)
		x1 := cx + int(math.Cos(angle)*length)
		y1 := cy + int(math.Sin(angle)*length)
		color := lerp(warmGradient(float64(i)/float64(rays-1)), Cream, 0.12)
		b.Line(x0, y0, x1, y1, color)
		if length-inner > 3 {
			b.Line(x0, y0+1, x1, y1+1, lerp(color, Cream, 0.18))
		}
		if value > 0.03 {
			for tip := 0; tip < 2; tip++ {
				b.Set(x1+tip, y1, pulseColor(Cream, now*2+float64(i), 0.4))
				b.Set(x1, y1+tip, pulseColor(color, now*2+float64(i), 0.25))
			}
		}
	}

	coreRadius := int(inner*0.55 + float64(bass)*inner*0.35)
	coreColor := blend(PanelBG, Magenta, 0.5+bass*0.3)
	for py := cy - coreRadius; py <= cy+coreRadius; py += 2 {
		for px := cx - coreRadius; px <= cx+coreRadius; px += 2 {
			dx := float64(px - cx)
			dy := float64(py - cy)
			distance := math.Hypot(dx, dy)
			if distance <= float64(coreRadius) {
				alpha := 0.38 + 0.5*(1-distance/float64(max(1, coreRadius)))
				b.Set(px, py, blend(PanelBG, blendToward(coreColor, Cream, 0.35), alpha))
			}
		}
	}
	drawCircle(b, cx, cy, coreRadius+2, blend(PanelBG, Accent, 0.7))

	orbit := now * 0.55
	if !options.Playing {
		orbit = now * 0.25
	}
	opx := cx + int(math.Cos(orbit)*inner*0.72)
	opy := cy + int(math.Sin(orbit)*inner*0.72)
	b.Set(opx, opy, Cream)
	b.Set(opx+1, opy, Accent)
	b.Flush()
}

func (v *Visualizer) drawAurora(canvas *Canvas, x, y, width, height int, now float64, options DrawOptions) {
	if height < 3 {
		return
	}
	// Starfield.
	starColors := []RGB{blend(PanelBG, Muted, 0.8), blend(PanelBG, Accent, 0.5), blend(PanelBG, Cream, 0.25)}
	for py := 0; py < height; py++ {
		for px := 0; px < width; px++ {
			hash := uint32(px*73856093) ^ uint32(py*19349663) ^ 0x9e3779b9
			if hash%47 != 0 {
				continue
			}
			star := starColors[hash%uint32(len(starColors))]
			if math.Sin(now*1.4+float64(hash%17)) > 0.2 {
				canvas.Set(x+px, y+py, '·', star)
			}
		}
	}

	back := smoothCurve(resample(v.Bands, width), 11)
	front := smoothCurve(resample(v.Bands, width), 5)
	if !options.Playing && !options.Paused {
		for i := range back {
			back[i], front[i] = 0, 0
		}
	}

	for i := 0; i < width; i++ {
		waveOffset := 0.035 * math.Sin(float64(i)*0.17+now*0.8)
		value := clamp01(0.05 + 0.90*math.Sqrt(back[i]) + waveOffset)
		if value <= 0.01 {
			continue
		}
		top := y + height - 1 - int(value*float64(height-1)*0.90)
		for py := top; py < y+height; py++ {
			heightT := float64(py-y) / float64(max(1, height-1))
			color := auroraGradient(0.15 + 0.65*float64(i)/float64(max(1, width-1)) + 0.2*heightT)
			alpha := 0.78 - 0.38*heightT
			canvas.Blend(x+i, py, '█', color, alpha)
		}
		topColor := auroraGradient(0.2 + 0.7*float64(i)/float64(max(1, width-1)))
		canvas.Set(x+i, top, '▔', lerp(topColor, Cream, 0.35))
	}

	for i := 0; i < width; i++ {
		waveOffset := 0.055 * math.Sin(float64(i)*0.11-now*0.55)
		value := clamp01(0.03 + 0.82*math.Sqrt(front[i]) + waveOffset)
		if value <= 0.01 {
			continue
		}
		top := y + height - 1 - int(value*float64(height-1)*0.78)
		for py := top; py < y+height; py++ {
			heightT := float64(py-y) / float64(max(1, height-1))
			color := auroraGradient(0.55 + 0.35*float64(i)/float64(max(1, width-1)))
			alpha := 0.68 - 0.32*heightT
			canvas.Blend(x+i, py, '▓', color, alpha)
		}
		canvas.Set(x+i, top, '▔', lerp(Peach, Cream, 0.4))
	}

	// Fireflies above energetic high bands.
	high := resample(v.Bands, width)
	placed := 0
	for px := width - 1; px >= 0 && placed < 9; px-- {
		if high[px] < 0.24 {
			continue
		}
		hash := uint32(px*2654435761) ^ uint32(v.Tick/3)
		top := y + height/2 - int(high[px]*float64(height)*0.55)
		rise := int(hash % 3)
		py := max(y, top-rise)
		if math.Sin(now*1.8+float64(hash%11)) > -0.2 {
			canvas.Set(x+px, py, '·', pulseColor(Peach, now*2+float64(hash%9), 0.4))
		}
		placed++
	}

	if !options.Playing {
		baseline := y + height - 2
		for px := 0; px < width; px++ {
			value := 0.012 + 0.010*math.Sin(float64(px)*0.12+now*0.9)
			py := baseline - int(value*float64(height))
			canvas.Set(x+px, py, '▁', blend(PanelBG, Accent, 0.45))
		}
	}
}

func (v *Visualizer) drawMatrix(canvas *Canvas, x, y, width, height int, now float64, options DrawOptions) {
	if height < 4 {
		return
	}
	columns := min(48, max(10, width/2))
	values := resample(v.Bands, columns)
	peaks := resample(v.Peaks, columns)
	rows := height
	slot := max(1, width/columns)
	base := y + height - 1
	scanRow := (v.Tick / 4) % max(1, rows)
	for i := 0; i < columns; i++ {
		columnX := x + i*slot + slot/2
		if columnX >= x+width {
			columnX = x + width - 1
		}
		value := displayLevel(values[i])
		peak := displayLevel(peaks[i])
		if !options.Playing && !options.Paused {
			value, peak = 0, 0
		}
		frequencyT := float64(i) / float64(max(1, columns-1))
		lit := int(value * float64(rows))
		for row := 0; row < rows; row++ {
			if row >= lit && (row+columnX)%2 != 0 && row != scanRow {
				continue
			}
			color := blend(PanelBG, Dim, 0.18)
			glyph := '·'
			if row == scanRow {
				color = blend(PanelBG, Muted, 0.34)
			}
			if row < lit {
				heightT := float64(row+1) / float64(max(1, rows))
				color = warmGradient(0.2 + 0.55*frequencyT + 0.25*heightT)
				glyph = '●'
			}
			canvas.Set(columnX, base-row, glyph, color)
		}
		if lit > 0 && lit < rows {
			canvas.Set(columnX, base-lit, '·', blend(PanelBG, warmGradient(0.2+0.55*frequencyT), 0.55))
		}
		peakRow := int(peak * float64(rows))
		if peakRow > lit && peakRow <= rows {
			canvas.Set(columnX, base-peakRow+1, '●', pulseColor(Cream, now*2+float64(i), 0.3))
		}
		if lit == 0 {
			canvas.Set(columnX, base, '·', Dim)
		}
	}
	if !options.Playing {
		position := int(now*8) % max(1, columns)
		canvas.Set(x+position*slot+slot/2, base, '◆', Accent)
	}
}

func drawRuler(canvas *Canvas, x, y, width int) {
	labels := []struct {
		frequency float64
		label     string
	}{
		{63, "63"}, {125, "125"}, {250, "250"}, {500, "500"},
		{1000, "1k"}, {2000, "2k"}, {4000, "4k"}, {8000, "8k"}, {16000, "16k"},
	}
	lastEnd := -10
	for _, item := range labels {
		position := int(math.Round(math.Log(item.frequency/audio.FMin) / math.Log(audio.FMax/audio.FMin) * float64(width-1)))
		start := position - len(item.label)/2
		if start < 0 {
			start = 0
		}
		if start+len(item.label) > width {
			start = width - len(item.label)
		}
		if start <= lastEnd+1 {
			continue
		}
		canvas.Text(x+start, y, item.label, Style{FG: Dim, BG: PanelBG})
		lastEnd = start + len(item.label)
	}
}

func drawCircle(b *Braille, cx, cy, radius int, color RGB) {
	if radius < 1 {
		return
	}
	steps := max(48, radius*8)
	for i := 0; i < steps; i++ {
		angle := 2 * math.Pi * float64(i) / float64(steps)
		b.Set(cx+int(math.Cos(angle)*float64(radius)), cy+int(math.Sin(angle)*float64(radius)), color)
	}
}

func displayLevel(value float64) float64 {
	if value <= 0 {
		return 0
	}
	return clamp01(math.Pow(value, 0.82) * 1.06)
}

func columnCount(width int) int {
	if width <= 0 {
		return 1
	}
	return max(8, min(audio.BandCount, width/3))
}

func resample(values []float64, count int) []float64 {
	out := make([]float64, count)
	if count <= 0 || len(values) == 0 {
		return out
	}
	if len(values) == 1 {
		for i := range out {
			out[i] = values[0]
		}
		return out
	}
	for i := 0; i < count; i++ {
		position := float64(i) * float64(len(values)-1) / float64(max(1, count-1))
		left := int(math.Floor(position))
		right := min(left+1, len(values)-1)
		fraction := position - float64(left)
		out[i] = values[left]*(1-fraction) + values[right]*fraction
	}
	return out
}

func resampleSigned(values []float64, count int) []float64 {
	out := make([]float64, count)
	if count <= 0 || len(values) == 0 {
		return out
	}
	for i := 0; i < count; i++ {
		index := i * len(values) / count
		out[i] = values[min(index, len(values)-1)]
	}
	return out
}

func smoothCurve(values []float64, radius int) []float64 {
	if len(values) == 0 || radius <= 0 {
		return values
	}
	out := make([]float64, len(values))
	for i := range values {
		sum := 0.0
		count := 0
		for offset := -radius; offset <= radius; offset++ {
			index := i + offset
			if index < 0 || index >= len(values) {
				continue
			}
			weight := 1.0 - math.Abs(float64(offset))/float64(radius+1)
			sum += values[index] * weight
			count++
		}
		if count > 0 {
			out[i] = sum / float64(count)
		}
	}
	return out
}

func blendToward(from, to RGB, amount float64) RGB {
	return lerp(from, to, amount)
}

// Canvas.Blend composites a translucent glyph over whatever is already there.
func (c *Canvas) Blend(x, y int, ch rune, color RGB, alpha float64) {
	if c == nil || x < 0 || y < 0 || x >= c.W || y >= c.H {
		return
	}
	cell := &c.cells[c.index(x, y)]
	base := cell.BG
	if cell.Ch != ' ' && !cell.Cont {
		base = cell.FG
	}
	cell.Ch = ch
	cell.FG = lerp(base, color, clamp01(alpha))
	cell.Bold = false
	cell.Cont = false
}
