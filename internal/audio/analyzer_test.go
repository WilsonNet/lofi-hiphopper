package audio

import (
	"context"
	"math"
	"math/cmplx"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFFTMatchesDFT(t *testing.T) {
	random := rand.New(rand.NewSource(7))
	n := 64
	re := make([]float64, n)
	im := make([]float64, n)
	for i := range re {
		re[i] = random.Float64()*2 - 1
	}
	wantRe := append([]float64(nil), re...)
	wantIm := append([]float64(nil), im...)
	fftInPlace(re, im)
	dftInPlace(wantRe, wantIm)
	for i := 0; i < n; i++ {
		got := complex(re[i], im[i])
		want := complex(wantRe[i], wantIm[i])
		if cmplx.Abs(got-want) > 1e-9 {
			t.Fatalf("bin %d: got %v, want %v", i, got, want)
		}
	}
}

func dftInPlace(re, im []float64) {
	n := len(re)
	outRe := make([]float64, n)
	outIm := make([]float64, n)
	for k := 0; k < n; k++ {
		for t := 0; t < n; t++ {
			angle := -2 * math.Pi * float64(k*t) / float64(n)
			outRe[k] += re[t]*math.Cos(angle) - im[t]*math.Sin(angle)
			outIm[k] += re[t]*math.Sin(angle) + im[t]*math.Cos(angle)
		}
	}
	copy(re, outRe)
	copy(im, outIm)
}

func TestFFTHandlesEdgeCases(t *testing.T) {
	fftInPlace(nil, nil)
	fftInPlace([]float64{1}, []float64{0})
	fftInPlace([]float64{1, 2, 3}, []float64{0, 0, 0})
	if got := nextPowerOfTwo(0); got != 1 {
		t.Fatalf("nextPowerOfTwo(0) = %d", got)
	}
	if got := nextPowerOfTwo(17); got != 32 {
		t.Fatalf("nextPowerOfTwo(17) = %d", got)
	}
}

func TestBandsFindSinePeak(t *testing.T) {
	samples := sineSamples(440, 0.4, FFTSize)
	bands := BandsFromSamples(samples, SampleRate, BandCount)
	if len(bands) != BandCount {
		t.Fatalf("len = %d", len(bands))
	}
	maxIndex := 0
	for i, value := range bands {
		if value < 0 || value > 1 {
			t.Fatalf("band %d out of range: %f", i, value)
		}
		if value > bands[maxIndex] {
			maxIndex = i
		}
	}
	peak := BandFrequency(maxIndex, BandCount)
	if peak < 400 || peak > 490 {
		t.Fatalf("peak = %.1f Hz (band %d), want ~440 Hz; bands=%v", peak, maxIndex, bands)
	}
	if bands[maxIndex] < 0.6 {
		t.Fatalf("peak amplitude = %.3f, want >= 0.6", bands[maxIndex])
	}
}

func TestBandsSilence(t *testing.T) {
	bands := BandsFromSamples(make([]float64, FFTSize), SampleRate, BandCount)
	for i, value := range bands {
		if value != 0 {
			t.Fatalf("silent band %d = %f", i, value)
		}
	}
}

func TestBandFrequenciesAreLogSpaced(t *testing.T) {
	first := BandFrequency(0, BandCount)
	last := BandFrequency(BandCount-1, BandCount)
	if first < 40 || first > 50 {
		t.Fatalf("first band = %.2f Hz", first)
	}
	if last < 14000 || last > 16000 {
		t.Fatalf("last band = %.2f Hz", last)
	}
	previous := 0.0
	for i := 0; i < BandCount; i++ {
		current := BandFrequency(i, BandCount)
		if current <= previous {
			t.Fatalf("band %d = %.2f, previous = %.2f", i, current, previous)
		}
		previous = current
	}
	// Ratios should be constant for a log scale.
	ratioA := BandFrequency(10, BandCount) / BandFrequency(9, BandCount)
	ratioB := BandFrequency(30, BandCount) / BandFrequency(29, BandCount)
	if math.Abs(ratioA-ratioB) > 0.01 {
		t.Fatalf("ratios differ: %.4f vs %.4f", ratioA, ratioB)
	}
}

func TestDownsampleWave(t *testing.T) {
	samples := sineSamples(100, 0.5, 1024)
	out := make([]float64, WaveCount)
	downsampleWave(samples, out)
	if len(out) != WaveCount {
		t.Fatalf("len = %d", len(out))
	}
	nonzero := 0
	for _, value := range out {
		if math.Abs(value) > 0.05 {
			nonzero++
		}
	}
	if nonzero < WaveCount/4 {
		t.Fatalf("wave looks empty: %d nonzero", nonzero)
	}
}

func TestSendLatestKeepsNewest(t *testing.T) {
	ch := make(chan Frame, 1)
	sendLatest(ch, Frame{RMS: 1})
	sendLatest(ch, Frame{RMS: 2})
	select {
	case frame := <-ch:
		if frame.RMS != 2 {
			t.Fatalf("rms = %v, want 2", frame.RMS)
		}
	default:
		t.Fatal("channel empty")
	}
}

func TestAnalyzerReadsLocalAudio(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	path := filepath.Join(t.TempDir(), "tone.wav")
	if err := writeTestWAV(path, 440, 1.2); err != nil {
		t.Fatal(err)
	}
	analyzer := NewAnalyzer()
	defer analyzer.Close()
	analyzer.Start(path)

	deadline := time.Now().Add(8 * time.Second)
	var frame Frame
	for time.Now().Before(deadline) {
		select {
		case frame = <-analyzer.Frames():
		case <-time.After(50 * time.Millisecond):
			continue
		}
		if len(frame.Bands) == BandCount {
			break
		}
	}
	if len(frame.Bands) != BandCount {
		t.Fatalf("no analysis frames; err = %v", analyzer.Err())
	}
	if frame.RMS <= 0 {
		t.Fatalf("rms = %f; frame = %+v", frame.RMS, frame)
	}
	maxIndex := 0
	for i, value := range frame.Bands {
		if value > frame.Bands[maxIndex] {
			maxIndex = i
		}
	}
	if frequency := BandFrequency(maxIndex, BandCount); frequency < 400 || frequency > 490 {
		t.Fatalf("analyzer peak = %.1f Hz", frequency)
	}
	analyzer.Stop()
}

func TestAnalyzerMissingBinary(t *testing.T) {
	analyzer := NewAnalyzer()
	analyzer.binary = "/definitely/missing/ffmpeg"
	analyzer.Start("https://example.com/audio")
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if analyzer.Err() != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := analyzer.Err(); err == nil || !strings.Contains(err.Error(), "ffmpeg not found") {
		t.Fatalf("err = %v", err)
	}
	analyzer.Close()
}

func TestAnalyzerCloseIsIdempotent(t *testing.T) {
	analyzer := NewAnalyzer()
	analyzer.Close()
	analyzer.Close()
	analyzer.Start("https://example.com/audio")
	analyzer.Close()
	_ = context.Background()
}

func sineSamples(frequency, amplitude float64, count int) []float64 {
	out := make([]float64, count)
	for i := range out {
		out[i] = amplitude * math.Sin(2*math.Pi*frequency*float64(i)/SampleRate)
	}
	return out
}

func writeTestWAV(path string, frequency, duration float64) error {
	const sampleRate = SampleRate
	samples := int(duration * sampleRate)
	dataSize := samples * 2
	data := make([]byte, 44+dataSize)
	copy(data[0:4], "RIFF")
	put32(data[4:8], uint32(36+dataSize))
	copy(data[8:12], "WAVE")
	copy(data[12:16], "fmt ")
	put32(data[16:20], 16)
	put16(data[20:22], 1)
	put16(data[22:24], 1)
	put32(data[24:28], sampleRate)
	put32(data[28:32], sampleRate*2)
	put16(data[32:34], 2)
	put16(data[34:36], 16)
	copy(data[36:40], "data")
	put32(data[40:44], uint32(dataSize))
	for i := 0; i < samples; i++ {
		value := int16(math.Sin(2*math.Pi*frequency*float64(i)/sampleRate) * 0.2 * 32767)
		put16(data[44+i*2:], uint16(value))
	}
	return os.WriteFile(path, data, 0o644)
}

func put16(buffer []byte, value uint16) {
	buffer[0] = byte(value)
	buffer[1] = byte(value >> 8)
}

func put32(buffer []byte, value uint32) {
	buffer[0] = byte(value)
	buffer[1] = byte(value >> 8)
	buffer[2] = byte(value >> 16)
	buffer[3] = byte(value >> 24)
}
