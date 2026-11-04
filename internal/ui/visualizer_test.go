package ui

import (
	"math"
	"strings"
	"testing"

	"github.com/wilsonpsn/lofi-hiphopper/internal/audio"
)

func populatedVisualizer() *Visualizer {
	v := NewVisualizer()
	bands := make([]float64, audio.BandCount)
	for i := range bands {
		bands[i] = 0.15 + 0.8*math.Abs(math.Sin(float64(i)*0.37))
	}
	wave := make([]float64, audio.WaveCount)
	for i := range wave {
		wave[i] = 0.6 * math.Sin(float64(i)*0.19)
	}
	for i := 0; i < 20; i++ {
		v.Advance(bands, wave)
	}
	return v
}

func TestVisualizerCyclesAllModes(t *testing.T) {
	v := NewVisualizer()
	if v.Name() != "BARS" {
		t.Fatalf("name = %q", v.Name())
	}
	v.Prev()
	if v.Mode != ModeMatrix {
		t.Fatalf("prev mode = %d", v.Mode)
	}
	v.Next()
	if v.Mode != ModeBars {
		t.Fatalf("next mode = %d", v.Mode)
	}
	for i := 0; i < VisualizerCount; i++ {
		if v.Name() != visualizerNames[i] {
			t.Fatalf("mode %d name = %q", i, v.Name())
		}
		v.Next()
	}
	if v.Mode != ModeBars {
		t.Fatalf("cycle ended at %d", v.Mode)
	}
	v.SetMode(-1)
	v.SetMode(999)
	if v.Mode != ModeBars {
		t.Fatalf("invalid SetMode changed mode to %d", v.Mode)
	}
}

func TestVisualizerSmoothsAndDecays(t *testing.T) {
	v := NewVisualizer()
	bands := make([]float64, audio.BandCount)
	for i := range bands {
		bands[i] = 0.8
	}
	v.Advance(bands, nil)
	if v.Bands[0] >= 0.8 || v.Bands[0] <= 0 {
		t.Fatalf("first attack = %f", v.Bands[0])
	}
	for i := 0; i < 30; i++ {
		v.Advance(bands, nil)
	}
	if v.Bands[0] < 0.78 {
		t.Fatalf("did not approach target: %f", v.Bands[0])
	}
	if v.Peaks[0] < v.Bands[0] {
		t.Fatalf("peak %f below band %f", v.Peaks[0], v.Bands[0])
	}
	for i := 0; i < 120; i++ {
		v.Advance(nil, nil)
	}
	if v.Bands[0] > 0.01 {
		t.Fatalf("did not decay: %f", v.Bands[0])
	}
	if v.Peaks[0] > 0.02 {
		t.Fatalf("peak did not fall: %f", v.Peaks[0])
	}
}

func TestVisualizerPeak(t *testing.T) {
	v := NewVisualizer()
	bands := make([]float64, audio.BandCount)
	bands[10] = 0.9
	for i := 0; i < 20; i++ {
		v.Advance(bands, nil)
	}
	if v.PeakBand() != 10 {
		t.Fatalf("peak band = %d", v.PeakBand())
	}
	frequency := v.PeakHz()
	want := audio.BandFrequency(10, audio.BandCount)
	if math.Abs(frequency-want) > 0.1 {
		t.Fatalf("peak hz = %f, want %f", frequency, want)
	}
}

func TestVisualizerDrawsEveryMode(t *testing.T) {
	v := populatedVisualizer()
	seen := map[string]string{}
	for mode := 0; mode < VisualizerCount; mode++ {
		v.SetMode(mode)
		canvas := NewCanvas(110, 30, Background)
		canvas.Panel(0, 0, 110, 30, "spectrum", "h/l", PanelStyle{
			Border: BorderColor, Background: PanelBG, Title: Accent, Right: Muted,
		})
		v.Draw(canvas, 2, 1, 106, 28, DrawOptions{Playing: true})
		rendered := canvas.Render()
		assertFrameSize(t, rendered, 110, 30)
		text := plain(rendered)
		if strings.TrimSpace(text) == "" {
			t.Fatalf("mode %d rendered blank", mode)
		}
		seen[v.Name()] = text
	}
	if len(seen) != VisualizerCount {
		t.Fatalf("saw %d modes", len(seen))
	}
	if seen["BARS"] == seen["MIRROR"] || seen["SCOPE"] == seen["BARS"] {
		t.Fatal("modes rendered identically")
	}
}

func TestVisualizerDrawsRuler(t *testing.T) {
	v := populatedVisualizer()
	v.SetMode(ModeBars)
	canvas := NewCanvas(100, 24, Background)
	v.Draw(canvas, 0, 0, 100, 24, DrawOptions{Playing: true})
	text := plain(canvas.Render())
	for _, label := range []string{"63", "1k", "16k"} {
		if !strings.Contains(text, label) {
			t.Errorf("ruler missing %q", label)
		}
	}
}

func TestVisualizerTinyAndIdle(t *testing.T) {
	v := NewVisualizer()
	for mode := 0; mode < VisualizerCount; mode++ {
		v.SetMode(mode)
		for _, size := range [][2]int{{6, 4}, {12, 6}, {30, 10}} {
			canvas := NewCanvas(size[0], size[1], Background)
			v.Draw(canvas, 0, 0, size[0], size[1], DrawOptions{})
			assertFrameSize(t, canvas.Render(), size[0], size[1])
		}
	}
}

func TestVisualizerPausedStillDraws(t *testing.T) {
	v := populatedVisualizer()
	for mode := 0; mode < VisualizerCount; mode++ {
		v.SetMode(mode)
		canvas := NewCanvas(80, 20, Background)
		v.Draw(canvas, 0, 0, 80, 20, DrawOptions{Playing: true, Paused: true})
		assertFrameSize(t, canvas.Render(), 80, 20)
	}
}

func TestColumnCount(t *testing.T) {
	if got := columnCount(200); got != audio.BandCount {
		t.Fatalf("wide columns = %d", got)
	}
	if got := columnCount(102); got != 34 {
		t.Fatalf("normal columns = %d", got)
	}
	if got := columnCount(6); got != 8 {
		t.Fatalf("narrow columns = %d", got)
	}
}

func BenchmarkVisualizerBars(b *testing.B) {
	v := populatedVisualizer()
	canvas := NewCanvas(120, 35, Background)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.Advance(make([]float64, audio.BandCount), make([]float64, audio.WaveCount))
		v.Draw(canvas, 0, 0, 120, 35, DrawOptions{Playing: true})
	}
}
