// Package audio decodes a stream with ffmpeg and turns it into spectrum frames
// for the TUI. Playback stays in mpv; this is a silent, parallel analyser.
package audio

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	// SampleRate is deliberately modest: plenty for 16 kHz visuals.
	SampleRate = 44100
	// FFTSize gives 21.5 Hz bins while staying cheap.
	FFTSize = 2048
	// Hop is half a window, so frames overlap and motion feels continuous.
	Hop = FFTSize / 2
	// BandCount is the number of log-spaced frequency bands.
	BandCount = 48
	// WaveCount is the number of time-domain points exposed to the UI.
	WaveCount = 256

	// FMin and FMax bound the displayed spectrum.
	FMin = 40.0
	FMax = 16000.0
)

// Frame is one analysed video frame's worth of audio.
type Frame struct {
	Bands []float64 // 0..1, log-spaced from FMin to FMax
	Wave  []float64 // -1..1 time-domain samples
	RMS   float64
	At    time.Time
}

// Analyzer owns a single ffmpeg analysis process at a time.
type Analyzer struct {
	binary    string
	userAgent string

	mu     sync.Mutex
	frames chan Frame
	cancel context.CancelFunc
	gen    int
	err    error
	wg     sync.WaitGroup
	closed bool
}

// NewAnalyzer creates a stopped analyser using ffmpeg from PATH.
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		binary:    "ffmpeg",
		userAgent: defaultUserAgent,
		frames:    make(chan Frame, 1),
	}
}

const defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// Frames returns the channel for the newest analysis frame. The channel is
// replaced whenever Start is called, so always fetch it freshly.
func (a *Analyzer) Frames() <-chan Frame {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.frames
}

// Err returns the most recent analyzer failure, if any.
func (a *Analyzer) Err() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.err
}

// Start begins analysing uri, stopping any previous analysis.
func (a *Analyzer) Start(uri string) {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return
	}
	if a.cancel != nil {
		a.cancel()
	}
	a.gen++
	gen := a.gen
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.err = nil
	frames := make(chan Frame, 1)
	a.frames = frames
	a.wg.Add(1)
	a.mu.Unlock()

	go a.run(ctx, gen, frames, uri)
}

// Stop halts analysis without blocking the caller.
func (a *Analyzer) Stop() {
	a.mu.Lock()
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
	a.gen++
	a.mu.Unlock()
}

// Close stops analysis and waits briefly for ffmpeg to exit.
func (a *Analyzer) Close() {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		a.wg.Wait()
		return
	}
	a.closed = true
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
	a.gen++
	a.mu.Unlock()
	a.wg.Wait()
}

func (a *Analyzer) run(ctx context.Context, gen int, frames chan Frame, uri string) {
	defer a.wg.Done()

	ffmpeg := strings.TrimSpace(a.binary)
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}
	if _, err := exec.LookPath(ffmpeg); err != nil {
		a.setError(gen, errors.New("ffmpeg not found (install it with: brew install ffmpeg)"))
		return
	}

	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-nostdin",
		"-re",
	}
	if strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://") {
		args = append(args,
			"-user_agent", a.userAgent,
			"-reconnect", "1",
			"-reconnect_streamed", "1",
			"-reconnect_delay_max", "5",
		)
	}
	args = append(args,
		"-i", uri,
		"-vn",
		"-ac", "1",
		"-ar", fmt.Sprintf("%d", SampleRate),
		"-f", "s16le",
		"pipe:1",
	)

	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	stderr := &tailBuffer{limit: 4 * 1024}
	cmd.Stderr = stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		a.setError(gen, fmt.Errorf("ffmpeg stdout: %w", err))
		return
	}
	if err := cmd.Start(); err != nil {
		a.setError(gen, fmt.Errorf("start ffmpeg: %w", err))
		return
	}

	ring := make([]float64, FFTSize)
	hop := make([]float64, Hop)
	wave := make([]float64, WaveCount)
	reader := bufio.NewReaderSize(stdout, Hop*2*4)
	bytesPerFrame := Hop * 2
	buffer := make([]byte, bytesPerFrame)
	first := true

	for {
		if _, err := io.ReadFull(reader, buffer); err != nil {
			if ctx.Err() != nil || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			a.setError(gen, fmt.Errorf("read ffmpeg audio: %w", err))
			break
		}

		for i := 0; i < Hop; i++ {
			hop[i] = float64(int16(binary.LittleEndian.Uint16(buffer[i*2:]))) / 32768.0
		}
		copy(ring, ring[Hop:])
		copy(ring[FFTSize-Hop:], hop)

		if first {
			// Let the ring fill before exposing a frame; otherwise the first
			// bars flash a half-empty window.
			first = false
			continue
		}

		rms := rootMeanSquare(hop)
		bands := BandsFromSamples(ring, SampleRate, BandCount)
		downsampleWave(ring[FFTSize-512:], wave)
		frame := Frame{
			Bands: bands,
			Wave:  append([]float64(nil), wave...),
			RMS:   rms,
			At:    time.Now(),
		}
		if !a.valid(gen) {
			break
		}
		sendLatest(frames, frame)
	}

	err = cmd.Wait()
	if ctx.Err() != nil {
		return
	}
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		a.setError(gen, fmt.Errorf("ffmpeg: %s", firstLine(detail)))
	}
}

func (a *Analyzer) valid(gen int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return !a.closed && a.gen == gen
}

func (a *Analyzer) setError(gen int, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.gen == gen {
		a.err = err
	}
}

func sendLatest(ch chan Frame, frame Frame) {
	select {
	case ch <- frame:
		return
	default:
	}
	select {
	case <-ch:
	default:
	}
	select {
	case ch <- frame:
	default:
	}
}

// BandsFromSamples returns nBands normalized magnitudes from a PCM window.
func BandsFromSamples(samples []float64, sampleRate, nBands int) []float64 {
	if nBands <= 0 {
		return nil
	}
	out := make([]float64, nBands)
	if len(samples) < 16 || sampleRate <= 0 {
		return out
	}

	size := nextPowerOfTwo(len(samples))
	re := make([]float64, size)
	im := make([]float64, size)
	windowSum := 0.0
	for i := 0; i < len(samples); i++ {
		// Periodic Hann window.
		window := 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(len(samples)))
		re[i] = samples[i] * window
		windowSum += window
	}
	if windowSum <= 0 {
		return out
	}
	fftInPlace(re, im)

	binHz := float64(sampleRate) / float64(size)
	maxBin := size/2 - 1
	scale := 2.0 / windowSum
	dbFloor, dbCeil := -58.0, -4.0

	for band := 0; band < nBands; band++ {
		low := bandEdge(band, nBands)
		high := bandEdge(band+1, nBands)
		start := int(math.Floor(low / binHz))
		end := int(math.Ceil(high / binHz))
		if start < 1 {
			start = 1
		}
		if end <= start {
			end = start + 1
		}
		if end > maxBin {
			end = maxBin
		}
		if start > maxBin {
			start = maxBin
		}
		power := 0.0
		count := 0
		for bin := start; bin < end; bin++ {
			magnitude := math.Hypot(re[bin], im[bin]) * scale
			power += magnitude * magnitude
			count++
		}
		if count == 0 {
			continue
		}
		amplitude := math.Sqrt(power / float64(count))
		db := 20 * math.Log10(amplitude+1e-9)
		tilt := 9.0 * math.Pow(float64(band)/float64(nBands-1), 0.7)
		db += tilt
		value := (db - dbFloor) / (dbCeil - dbFloor)
		value = math.Max(0, math.Min(1, value))
		out[band] = math.Pow(value, 0.92)
	}
	if rootMeanSquare(samples) < 0.00005 {
		return make([]float64, nBands)
	}
	return out
}

// BandFrequency returns the geometric centre of a band in Hz.
func BandFrequency(band, nBands int) float64 {
	if nBands <= 0 {
		return FMin
	}
	if band < 0 {
		band = 0
	}
	if band >= nBands {
		band = nBands - 1
	}
	low := bandEdge(band, nBands)
	high := bandEdge(band+1, nBands)
	return math.Sqrt(low * high)
}

func bandEdge(band, nBands int) float64 {
	return FMin * math.Pow(FMax/FMin, float64(band)/float64(nBands))
}

func downsampleWave(samples []float64, out []float64) {
	if len(samples) == 0 || len(out) == 0 {
		return
	}
	mean := 0.0
	for _, sample := range samples {
		mean += sample
	}
	mean /= float64(len(samples))
	for i := range out {
		index := i * len(samples) / len(out)
		out[i] = samples[index] - mean
	}
}

func rootMeanSquare(samples []float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	sum := 0.0
	for _, sample := range samples {
		sum += sample * sample
	}
	return math.Sqrt(sum / float64(len(samples)))
}

func firstLine(text string) string {
	if index := strings.IndexByte(text, '\n'); index >= 0 {
		text = text[:index]
	}
	text = strings.TrimSpace(text)
	if len(text) > 300 {
		text = text[:300] + "…"
	}
	return text
}

type tailBuffer struct {
	mu    sync.Mutex
	data  []byte
	limit int
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, p...)
	if len(b.data) > b.limit {
		b.data = b.data[len(b.data)-b.limit:]
	}
	return len(p), nil
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimSpace(string(b.data))
}
