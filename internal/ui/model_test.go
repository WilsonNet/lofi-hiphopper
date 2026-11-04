package ui

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/wilsonpsn/lofi-hiphopper/internal/audio"
	"github.com/wilsonpsn/lofi-hiphopper/internal/config"
	"github.com/wilsonpsn/lofi-hiphopper/internal/detect"
	"github.com/wilsonpsn/lofi-hiphopper/internal/media"
	"github.com/wilsonpsn/lofi-hiphopper/internal/player"
)

type fakePlayer struct {
	mu       sync.Mutex
	snapshot player.Snapshot
	events   chan player.Event
	calls    []string
	playURL  string
}

func newFakePlayer() *fakePlayer {
	return &fakePlayer{
		snapshot: player.Snapshot{Running: true, Idle: true, Volume: 55},
		events:   make(chan player.Event, 16),
	}
}

func (f *fakePlayer) Start(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "start")
	f.snapshot.Running = true
	return nil
}

func (f *fakePlayer) Play(_ context.Context, uri string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "play")
	f.playURL = uri
	f.snapshot.Running = true
	f.snapshot.Idle = false
	f.snapshot.Loading = false
	f.snapshot.Paused = false
	return nil
}

func (f *fakePlayer) Pause(_ context.Context, paused bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "pause")
	f.snapshot.Paused = paused
	return nil
}

func (f *fakePlayer) Stop(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "stop")
	f.snapshot.Idle = true
	f.snapshot.Paused = false
	return nil
}

func (f *fakePlayer) SetVolume(_ context.Context, volume int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "volume")
	f.snapshot.Volume = volume
	return nil
}

func (f *fakePlayer) SetMute(_ context.Context, muted bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "mute")
	f.snapshot.Muted = muted
	return nil
}

func (f *fakePlayer) Snapshot() player.Snapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.snapshot
}

func (f *fakePlayer) Events() <-chan player.Event { return f.events }

func (f *fakePlayer) PID() int { return 4242 }

func (f *fakePlayer) emit(event player.Event) {
	f.events <- event
}

func (f *fakePlayer) hasCall(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, call := range f.calls {
		if call == name {
			return true
		}
	}
	return false
}

type fakeResolver struct {
	mu     sync.Mutex
	calls  int
	urls   []string
	stream media.Stream
	err    error
}

func (f *fakeResolver) Resolve(_ context.Context, pageURL string) (media.Stream, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.urls = append(f.urls, pageURL)
	if f.err != nil {
		return media.Stream{}, f.err
	}
	if f.stream.URL == "" {
		return media.Stream{URL: "https://stream.example/live.m3u8", Title: "Test stream", Live: true}, nil
	}
	return f.stream, nil
}

type fakeAnalyzer struct {
	mu     sync.Mutex
	frames chan audio.Frame
	starts []string
	err    error
}

func newFakeAnalyzer() *fakeAnalyzer {
	return &fakeAnalyzer{frames: make(chan audio.Frame, 2)}
}

func (f *fakeAnalyzer) Start(uri string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts = append(f.starts, uri)
	f.err = nil
}

func (f *fakeAnalyzer) Stop() {}

func (f *fakeAnalyzer) Frames() <-chan audio.Frame { return f.frames }

func (f *fakeAnalyzer) Err() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.err
}

type fakeProbe struct {
	processes []detect.Process
	err       error
}

func (f *fakeProbe) Processes() ([]detect.Process, error) {
	return f.processes, f.err
}

type testRig struct {
	player   *fakePlayer
	resolver *fakeResolver
	analyzer *fakeAnalyzer
	probe    *fakeProbe
	model    *Model
}

func newRig(t *testing.T) *testRig {
	t.Helper()
	cfg := config.Default()
	rig := &testRig{
		player:   newFakePlayer(),
		resolver: &fakeResolver{},
		analyzer: newFakeAnalyzer(),
		probe:    &fakeProbe{},
	}
	rig.model = New(cfg, Deps{
		Player: rig.player, Analyzer: rig.analyzer, Resolver: rig.resolver, Probe: rig.probe,
	}, Options{})
	rig.model.width = 120
	rig.model.height = 40
	rig.model.now = time.Now()
	return rig
}

func (r *testRig) detect(processes ...detect.Process) tea.Cmd {
	return r.model.handleDetection(detectionMsg{processes: processes})
}

func (r *testRig) startPlayback(t *testing.T) {
	t.Helper()
	cmd := r.detect()
	if cmd == nil {
		t.Fatal("initial silence did not produce a playback command")
	}
	message := cmd()
	if _, ok := message.(tuneMsg); !ok {
		t.Fatalf("command returned %T, want tuneMsg", message)
	}
	_, next := r.model.Update(message)
	if next != nil {
		_ = runCommand(next)
	}
	if r.model.state != statePlaying {
		t.Fatalf("state = %v, want playing (playerErr=%v)", r.model.state, r.model.playerErr)
	}
}

func runCommand(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

func TestInitialSilenceStartsDefaultRadio(t *testing.T) {
	rig := newRig(t)
	rig.startPlayback(t)
	if rig.resolver.calls != 1 {
		t.Fatalf("resolver calls = %d", rig.resolver.calls)
	}
	if rig.resolver.urls[0] != config.DefaultRadios()[0].URL {
		t.Fatalf("resolved %q", rig.resolver.urls[0])
	}
	if rig.player.playURL != "https://stream.example/live.m3u8" {
		t.Fatalf("played %q", rig.player.playURL)
	}
	if len(rig.analyzer.starts) != 1 || rig.analyzer.starts[0] == "" {
		t.Fatalf("analyzer starts = %v", rig.analyzer.starts)
	}
	if rig.model.loaded != 0 || rig.model.current != 0 {
		t.Fatalf("loaded/current = %d/%d", rig.model.loaded, rig.model.current)
	}
}

func TestCallSuspendsAndResumesAfterTwoSilentSamples(t *testing.T) {
	rig := newRig(t)
	rig.startPlayback(t)

	cmd := rig.detect(detect.Process{PID: 7, Name: "zoom.us", Input: true, Output: true})
	if cmd != nil {
		if msg := cmd(); msg != nil {
			if _, ok := msg.(playerActionMsg); !ok {
				t.Fatalf("hold command returned %T", msg)
			}
		}
	}
	if rig.model.state != stateSuspended {
		t.Fatalf("state = %v, want suspended", rig.model.state)
	}
	if rig.model.pauseReason != "call" {
		t.Fatalf("reason = %q", rig.model.pauseReason)
	}
	if !rig.player.hasCall("stop") {
		t.Fatalf("live call did not stop mpv: %v", rig.player.calls)
	}

	if cmd := rig.detect(); cmd != nil {
		t.Fatalf("one silent sample resumed too early")
	}
	cmd = rig.detect()
	if cmd == nil {
		t.Fatal("two silent samples did not resume")
	}
	message := cmd()
	if _, ok := message.(tuneMsg); !ok {
		t.Fatalf("resume returned %T", message)
	}
	_, next := rig.model.Update(message)
	if next != nil {
		_ = runCommand(next)
	}
	if rig.model.state != statePlaying {
		t.Fatalf("state after resume = %v", rig.model.state)
	}
	if rig.resolver.calls != 2 {
		t.Fatalf("resolver calls = %d, want re-resolve", rig.resolver.calls)
	}
}

func TestOtherAudioBlocksAutoplayUntilSilence(t *testing.T) {
	rig := newRig(t)
	cmd := rig.detect(detect.Process{PID: 9, Name: "Spotify", Output: true})
	if cmd != nil {
		t.Fatalf("unexpected command while other audio plays: %T", cmd())
	}
	if rig.model.state != stateIdle {
		t.Fatalf("state = %v", rig.model.state)
	}
	if rig.resolver.calls != 0 {
		t.Fatalf("resolver calls = %d", rig.resolver.calls)
	}
	_ = rig.detect()
	cmd = rig.detect()
	if cmd == nil {
		t.Fatal("silence did not eventually start playback")
	}
	_, next := rig.model.Update(cmd())
	if next != nil {
		_ = runCommand(next)
	}
	if rig.model.state != statePlaying {
		t.Fatalf("state = %v", rig.model.state)
	}
}

func TestAutofillOffRequiresEnter(t *testing.T) {
	rig := newRig(t)
	rig.model.autofill = false
	if cmd := rig.detect(); cmd != nil {
		t.Fatalf("autofill off still started: %T", cmd())
	}
	cmd := rig.model.requestPlay(0)
	if cmd == nil {
		t.Fatal("enter did not start playback")
	}
	_, next := rig.model.Update(cmd())
	if next != nil {
		_ = runCommand(next)
	}
	if rig.model.state != statePlaying {
		t.Fatalf("state = %v", rig.model.state)
	}
}

func TestVolumeAndMuteKeys(t *testing.T) {
	rig := newRig(t)
	rig.startPlayback(t)

	if cmd := rig.model.handleKey("="); cmd == nil {
		t.Fatal("volume up produced no command")
	} else {
		_ = runCommand(cmd)
	}
	if rig.player.Snapshot().Volume != 60 {
		t.Fatalf("volume = %d", rig.player.Snapshot().Volume)
	}
	if cmd := rig.model.handleKey("m"); cmd == nil {
		t.Fatal("mute produced no command")
	} else {
		_ = runCommand(cmd)
	}
	if !rig.player.Snapshot().Muted || !rig.model.muted {
		t.Fatal("mute did not apply")
	}
	if cmd := rig.model.handleKey("-"); cmd == nil {
		t.Fatal("volume down produced no command")
	} else {
		_ = runCommand(cmd)
	}
	if rig.player.Snapshot().Muted || rig.model.muted {
		t.Fatal("volume key should unmute")
	}
	if rig.player.Snapshot().Volume != 55 {
		t.Fatalf("volume = %d", rig.player.Snapshot().Volume)
	}
}

func TestVisualizerAndSelectionKeys(t *testing.T) {
	rig := newRig(t)
	rig.model.handleKey("l")
	if rig.model.viz.Mode != ModeMirror {
		t.Fatalf("mode = %d", rig.model.viz.Mode)
	}
	rig.model.handleKey("h")
	if rig.model.viz.Mode != ModeBars {
		t.Fatalf("mode = %d", rig.model.viz.Mode)
	}
	rig.model.handleKey("4")
	if rig.model.viz.Mode != ModeBars {
		t.Fatalf("numeric key should be unbound, mode = %d", rig.model.viz.Mode)
	}
	rig.model.handleKey("j")
	if rig.model.selected != 1 {
		t.Fatalf("selected = %d", rig.model.selected)
	}
	rig.model.handleKey("k")
	rig.model.handleKey("k")
	if rig.model.selected != len(rig.model.radios)-1 {
		t.Fatalf("selection did not wrap: %d", rig.model.selected)
	}
}

func TestEnterDuringCallQueuesPlayback(t *testing.T) {
	rig := newRig(t)
	_ = rig.detect(detect.Process{PID: 1, Name: "FaceTime", Input: true})
	cmd := rig.model.requestPlay(2)
	if cmd != nil {
		if msg := cmd(); msg != nil {
			t.Fatalf("unexpected command during call: %T", msg)
		}
	}
	if rig.model.current != 2 || !rig.model.resumeOnSilence {
		t.Fatalf("request not queued: current=%d resume=%v", rig.model.current, rig.model.resumeOnSilence)
	}
	_ = rig.detect()
	cmd = rig.detect()
	if cmd == nil {
		t.Fatal("queued playback did not start after silence")
	}
	_, next := rig.model.Update(cmd())
	if next != nil {
		_ = runCommand(next)
	}
	if rig.model.state != statePlaying || rig.model.current != 2 {
		t.Fatalf("state/current = %v/%d", rig.model.state, rig.model.current)
	}
}

func TestStreamEndSchedulesRetry(t *testing.T) {
	rig := newRig(t)
	rig.startPlayback(t)
	rig.player.emit(player.Event{Kind: player.EventEnded, Reason: "eof"})
	cmd := rig.model.pumpPlayer()
	if cmd != nil {
		t.Fatalf("unexpected immediate retry command: %T", cmd())
	}
	if rig.model.state != stateRetrying {
		t.Fatalf("state = %v, want retrying", rig.model.state)
	}
	if rig.model.retryAt.Before(time.Now()) {
		t.Fatal("retryAt should be in the future")
	}

	rig.model.retryAt = time.Now().Add(-time.Second)
	rig.model.now = time.Now()
	cmd = rig.model.pumpPlayer()
	if cmd == nil {
		t.Fatal("retry due did not start a tune")
	}
	_, next := rig.model.Update(cmd())
	if next != nil {
		_ = runCommand(next)
	}
	if rig.model.state != statePlaying {
		t.Fatalf("state = %v", rig.model.state)
	}
	if rig.model.retryBackoff != minRetry {
		t.Fatalf("backoff did not reset: %s", rig.model.retryBackoff)
	}
}

func TestTuneFailureRetriesLater(t *testing.T) {
	rig := newRig(t)
	rig.resolver.err = errors.New("yt-dlp: nope")
	cmd := rig.detect()
	message := cmd()
	_, next := rig.model.Update(message)
	if next != nil {
		_ = runCommand(next)
	}
	if rig.model.state != stateRetrying {
		t.Fatalf("state = %v", rig.model.state)
	}
	if rig.model.playerErr == nil || !strings.Contains(rig.model.playerErr.Error(), "nope") {
		t.Fatalf("playerErr = %v", rig.model.playerErr)
	}
	if time.Until(rig.model.retryAt) > 5*time.Second {
		t.Fatalf("retryAt too far away: %s", time.Until(rig.model.retryAt))
	}
}

func TestDemoFramesAndView(t *testing.T) {
	cfg := config.Default()
	model := New(cfg, Deps{}, Options{Demo: true})
	model.width = 120
	model.height = 40
	bands, wave := model.demoFrame()
	if len(bands) != audio.BandCount || len(wave) != audio.WaveCount {
		t.Fatal("demo frame has wrong shape")
	}
	peak := 0.0
	for _, value := range bands {
		if value > peak {
			peak = value
		}
	}
	if peak < 0.2 {
		t.Fatalf("demo spectrum too quiet: %f", peak)
	}
	view := model.View()
	assertFrameSize(t, view, 120, 40)
	if !strings.Contains(plain(view), "DEMO") {
		t.Fatal("demo badge missing")
	}
}

func TestViewAcrossSizes(t *testing.T) {
	cfg := config.Default()
	for _, size := range [][2]int{
		{140, 45}, {120, 40}, {100, 30}, {92, 24}, {80, 30}, {60, 20},
		{46, 18}, {45, 18}, {40, 15}, {30, 10}, {20, 5}, {8, 3}, {1, 1},
	} {
		model := New(cfg, Deps{}, Options{Demo: true})
		model.width, model.height = size[0], size[1]
		view := model.View()
		assertFrameSize(t, view, size[0], size[1])
	}
}

func TestViewNonDemo(t *testing.T) {
	rig := newRig(t)
	view := rig.model.View()
	assertFrameSize(t, view, 120, 40)
	text := plain(view)
	for _, want := range []string{"RADIO", "NOW PLAYING", "SIGNAL", "BARS", "AUTO-FILL"} {
		if !strings.Contains(text, want) {
			t.Errorf("view missing %q", want)
		}
	}
}

func TestPlayerStartFailureIsVisible(t *testing.T) {
	rig := newRig(t)
	rig.model.Update(playerStartedMsg{err: errors.New("mpv missing")})
	view := rig.model.View()
	if !strings.Contains(plain(view), "mpv missing") && !strings.Contains(rig.model.notice, "mpv missing") {
		t.Fatalf("failure not surfaced: notice=%q view=%q", rig.model.notice, plain(view))
	}
}

func TestAutofillToggleOnStartsWhenSilent(t *testing.T) {
	rig := newRig(t)
	rig.model.autofill = false
	if cmd := rig.detect(); cmd != nil {
		t.Fatalf("autofill off started unexpectedly: %T", cmd())
	}
	cmd := rig.model.handleKey("a")
	if cmd == nil {
		t.Fatal("turning auto-fill on did not start playback")
	}
	_, next := rig.model.Update(cmd())
	if next != nil {
		_ = runCommand(next)
	}
	if rig.model.state != statePlaying {
		t.Fatalf("state = %v", rig.model.state)
	}
}

func TestCallSafetyDoesNotDependOnAutofill(t *testing.T) {
	rig := newRig(t)
	rig.startPlayback(t)
	rig.model.autofill = false
	cmd := rig.detect(detect.Process{PID: 88, Name: "zoom.us", Input: true})
	if cmd != nil {
		_ = runCommand(cmd)
	}
	if rig.model.state != stateSuspended {
		t.Fatalf("call did not suspend with autofill off: %v", rig.model.state)
	}
	_ = rig.detect()
	cmd = rig.detect()
	if cmd == nil {
		t.Fatal("did not resume after call with autofill off")
	}
	_, next := rig.model.Update(cmd())
	if next != nil {
		_ = runCommand(next)
	}
	if rig.model.state != statePlaying {
		t.Fatalf("state = %v", rig.model.state)
	}
}

func TestManualPlayWorksWhenDetectionFails(t *testing.T) {
	rig := newRig(t)
	rig.model.probeErr = errors.New("probe unavailable")
	rig.model.ready = false
	cmd := rig.model.requestPlay(0)
	if cmd == nil {
		t.Fatal("manual play did not start with detection offline")
	}
	_, next := rig.model.Update(cmd())
	if next != nil {
		_ = runCommand(next)
	}
	if rig.model.state != statePlaying {
		t.Fatalf("state = %v", rig.model.state)
	}
}

func TestMouseWheelShortcuts(t *testing.T) {
	rig := newRig(t)
	rig.startPlayback(t)

	beforeVolume := rig.model.volume
	_, cmd := rig.model.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	if cmd != nil {
		_ = runCommand(cmd)
	}
	if rig.model.volume != beforeVolume+5 {
		t.Fatalf("wheel up volume = %d, want %d", rig.model.volume, beforeVolume+5)
	}
	if rig.player.Snapshot().Volume != beforeVolume+5 {
		t.Fatalf("mpv volume = %d", rig.player.Snapshot().Volume)
	}

	beforeSelected := rig.model.selected
	rig.model.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Shift: true, Action: tea.MouseActionPress})
	if rig.model.selected != (beforeSelected+1)%len(rig.model.radios) {
		t.Fatalf("shift+wheel selected = %d", rig.model.selected)
	}

	beforeMode := rig.model.viz.Mode
	rig.model.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Ctrl: true, Action: tea.MouseActionPress})
	if rig.model.viz.Mode != (beforeMode+1)%VisualizerCount {
		t.Fatalf("ctrl+wheel mode = %d", rig.model.viz.Mode)
	}
}

func TestPlainWheelDoesNotChangeRadioOrVisualizer(t *testing.T) {
	rig := newRig(t)
	selected, mode := rig.model.selected, rig.model.viz.Mode
	rig.model.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	if rig.model.selected != selected || rig.model.viz.Mode != mode {
		t.Fatal("plain wheel changed an unrelated control")
	}
}

func TestHelpOverlay(t *testing.T) {
	cfg := config.Default()
	model := New(cfg, Deps{}, Options{Demo: true})
	model.width, model.height = 120, 40
	model.handleKey("?")
	if !model.helpOpen {
		t.Fatal("? did not open help")
	}
	view := plain(model.View())
	for _, want := range []string{"SHORTCUTS", "PLAYBACK", "volume up", "wheel ↑", "edit bindings", "config.yaml"} {
		if !strings.Contains(view, want) {
			t.Errorf("help missing %q", want)
		}
	}
	assertFrameSize(t, model.View(), 120, 40)
	model.handleKey("?")
	if model.helpOpen {
		t.Fatal("? did not close help")
	}
	model.handleKey("?")
	model.handleKey("esc")
	if model.helpOpen {
		t.Fatal("esc did not close help")
	}
}

func TestHelpRespectsCustomBindings(t *testing.T) {
	cfg := config.Default()
	for index := range cfg.Shortcuts {
		switch cfg.Shortcuts[index].Action {
		case "mute":
			cfg.Shortcuts[index].Keys = []string{"x"}
		case "help":
			cfg.Shortcuts[index].Keys = []string{"f1"}
		}
	}
	model := New(cfg, Deps{}, Options{Demo: true})
	model.width, model.height = 100, 30
	model.handleKey("?")
	if model.helpOpen {
		t.Fatal("? opened help despite custom help binding")
	}
	model.handleKey("f1")
	if !model.helpOpen {
		t.Fatal("custom help binding did not open help")
	}
	if !strings.Contains(plain(model.View()), "x") {
		t.Fatal("custom mute binding absent from help")
	}
	model.handleKey("esc")
	model.handleKey("m")
	if model.muted {
		t.Fatal("old mute binding still active")
	}
	model.handleKey("x")
	if !model.muted {
		t.Fatal("custom mute binding inactive")
	}
	footer := plain(model.View())
	if !strings.Contains(footer, "x mute") {
		t.Fatalf("footer did not adapt to custom mute binding:\n%s", footer)
	}
	if !strings.Contains(footer, "f1 help") {
		t.Fatalf("footer did not adapt to custom help binding:\n%s", footer)
	}
}

func TestVolumeStepFromConfig(t *testing.T) {
	cfg := config.Default()
	cfg.VolumeStep = 11
	model := New(cfg, Deps{}, Options{Demo: true})
	model.handleKey("=")
	if model.volume != cfg.Volume+11 {
		t.Fatalf("volume = %d, want %d", model.volume, cfg.Volume+11)
	}
}

func TestHelpViewAcrossSizes(t *testing.T) {
	cfg := config.Default()
	for _, size := range [][2]int{{140, 45}, {120, 40}, {100, 30}, {92, 24}, {60, 20}, {46, 18}, {30, 10}} {
		model := New(cfg, Deps{}, Options{Demo: true})
		model.width, model.height = size[0], size[1]
		model.helpOpen = true
		assertFrameSize(t, model.View(), size[0], size[1])
	}
}

func TestSpaceBindingAlias(t *testing.T) {
	cfg := config.Default()
	for index := range cfg.Shortcuts {
		if cfg.Shortcuts[index].Action == "mute" {
			cfg.Shortcuts[index].Keys = []string{"space"}
		}
	}
	model := New(cfg, Deps{}, Options{Demo: true})
	model.handleKey(" ")
	if !model.muted {
		t.Fatal("space alias did not trigger mute")
	}
}
