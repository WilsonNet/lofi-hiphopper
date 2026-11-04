package ui

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/wilsonpsn/lofi-hiphopper/internal/audio"
	"github.com/wilsonpsn/lofi-hiphopper/internal/config"
	"github.com/wilsonpsn/lofi-hiphopper/internal/detect"
	"github.com/wilsonpsn/lofi-hiphopper/internal/media"
	"github.com/wilsonpsn/lofi-hiphopper/internal/player"
	"github.com/wilsonpsn/lofi-hiphopper/internal/update"
)

// Player is the subset of mpv control the TUI needs.
type Player interface {
	Start(ctx context.Context) error
	Play(ctx context.Context, uri string) error
	Pause(ctx context.Context, paused bool) error
	Stop(ctx context.Context) error
	SetVolume(ctx context.Context, volume int) error
	SetMute(ctx context.Context, muted bool) error
	Snapshot() player.Snapshot
	Events() <-chan player.Event
	PID() int
}

// Analyzer is the silent ffmpeg spectrum source.
type Analyzer interface {
	Start(uri string)
	Stop()
	Frames() <-chan audio.Frame
	Err() error
}

// Resolver turns a radio page URL into a playable stream.
type Resolver interface {
	Resolve(ctx context.Context, pageURL string) (media.Stream, error)
}

// Probe enumerates macOS audio processes.
type Probe interface {
	Processes() ([]detect.Process, error)
}

// Updater checks GitHub releases and applies them.
type Updater interface {
	Check(ctx context.Context) (update.Release, bool, error)
	Apply(ctx context.Context, release update.Release) error
}

// Deps wires the TUI to the outside world.
type Deps struct {
	Player   Player
	Analyzer Analyzer
	Resolver Resolver
	Probe    Probe
	Updater  Updater
}

// Options configures the model.
type Options struct {
	Demo       bool
	ConfigPath string
}

type playbackState int

const (
	stateIdle playbackState = iota
	stateTuning
	statePlaying
	stateSuspended
	stateRetrying
)

type noticeKind int

const (
	noticeInfo noticeKind = iota
	noticeGood
	noticeBad
)

const (
	animationInterval = 33 * time.Millisecond
	minRetry          = 3 * time.Second
	maxRetry          = 30 * time.Second
)

// Model is the Bubble Tea model.
type Model struct {
	cfg        config.Config
	deps       Deps
	demo       bool
	configPath string

	volumeStep int
	shortcuts  []config.Shortcut
	bindings   map[string]string
	helpOpen   bool

	width  int
	height int
	radios []config.Radio

	selected int
	current  int
	loaded   int
	state    playbackState
	stream   media.Stream

	pauseReason     string
	suspendedStop   bool
	resumeOnSilence bool
	muteHold        bool

	autofill bool
	volume   int
	muted    bool

	volumeChangedAt time.Time
	muteChangedAt   time.Time

	viz  *Visualizer
	tick int
	now  time.Time

	ready      bool
	detecting  bool
	activity   detect.Activity
	assessment detect.Assessment
	tracker    *detect.Tracker
	probeErr   error
	playerErr  error
	audioErr   error

	snap        player.Snapshot
	lastFrameAt time.Time
	lastPlayAt  time.Time

	tuneGen    int
	tuneEpoch  atomic.Int64
	tuneLock   sync.Mutex
	tuneCancel context.CancelFunc

	retryAt      time.Time
	retryBackoff time.Duration

	notice      string
	noticeKind  noticeKind
	noticeUntil time.Time

	updateAvailable  bool
	updateRelease    update.Release
	updateDialog     bool
	updating         bool
	updateErr        error
	restartRequested bool

	demoPhase float64
}

// New builds the TUI model. Config radios are copied.
func New(cfg config.Config, deps Deps, options Options) *Model {
	radios := append([]config.Radio(nil), cfg.Radios...)
	m := &Model{
		cfg:          cfg,
		deps:         deps,
		demo:         options.Demo,
		radios:       radios,
		selected:     0,
		current:      0,
		loaded:       -1,
		state:        stateIdle,
		autofill:     cfg.Autofill,
		volume:       cfg.Volume,
		muted:        false,
		viz:          NewVisualizer(),
		now:          time.Now(),
		tracker:      detect.NewTracker(),
		retryBackoff: minRetry,
	}
	if options.ConfigPath == "" {
		m.configPath = config.Path()
	} else {
		m.configPath = options.ConfigPath
	}
	m.volumeStep = cfg.VolumeStep
	if m.volumeStep < 1 {
		m.volumeStep = config.DefaultVolumeStep
	}
	shortcuts := cfg.Shortcuts
	if len(shortcuts) == 0 {
		shortcuts = config.DefaultShortcuts()
	}
	m.shortcuts = append([]config.Shortcut(nil), shortcuts...)
	m.bindings = make(map[string]string)
	for _, shortcut := range m.shortcuts {
		for _, key := range shortcut.Keys {
			bindingKey := key
			if bindingKey == "space" {
				bindingKey = " "
			}
			m.bindings[bindingKey] = shortcut.Action
		}
	}
	if m.demo {
		m.state = statePlaying
		m.ready = true
		m.loaded = 0
		title := "demo"
		if len(radios) > 0 {
			title = radios[0].Name
		}
		m.stream = media.Stream{Title: title, Live: true}
	}
	return m
}

// Init starts mpv, the detector, and the animation loop.
func (m *Model) Init() tea.Cmd {
	if m.demo {
		m.setNotice("demo mode · simulated spectrum", noticeInfo, 4*time.Second)
		return tea.Batch(m.animationTick(), m.updateCheckCmd())
	}
	if len(m.radios) == 0 {
		m.setNotice("no radios configured", noticeBad, 10*time.Second)
		return m.animationTick()
	}
	m.detecting = true
	cmds := []tea.Cmd{
		m.animationTick(),
		m.startPlayerCmd(),
		m.detectCmd(),
		m.detectionTick(),
		m.updateCheckCmd(),
	}
	return tea.Batch(cmds...)
}

func (m *Model) animationTick() tea.Cmd {
	return tea.Tick(animationInterval, func(time.Time) tea.Msg {
		return animationTickMsg{}
	})
}

func (m *Model) detectionTick() tea.Cmd {
	return tea.Tick(m.cfg.Poll(), func(time.Time) tea.Msg {
		return detectionTickMsg{}
	})
}

func (m *Model) detectCmd() tea.Cmd {
	if m.deps.Probe == nil {
		return nil
	}
	probe := m.deps.Probe
	return func() tea.Msg {
		processes, err := probe.Processes()
		return detectionMsg{processes: processes, err: err}
	}
}

func (m *Model) startPlayerCmd() tea.Cmd {
	if m.deps.Player == nil {
		return nil
	}
	playerService := m.deps.Player
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		err := playerService.Start(ctx)
		return playerStartedMsg{err: err}
	}
}

// Messages are deliberately tiny so Update remains side-effect free.
type (
	animationTickMsg struct{}
	detectionTickMsg struct{}
	detectionMsg     struct {
		processes []detect.Process
		err       error
	}
	playerStartedMsg struct{ err error }
	playerActionMsg  struct {
		action string
		err    error
	}
	tuneMsg struct {
		gen    int
		index  int
		stream media.Stream
		err    error
	}
	updateCheckMsg struct {
		release   update.Release
		available bool
		err       error
	}
	updateAppliedMsg struct{ err error }
)

// Update implements tea.Model.
func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = message.Width, message.Height
		return m, nil
	case tea.KeyMsg:
		return m, m.handleKey(message.String())
	case tea.MouseMsg:
		return m, m.handleMouse(message)
	case animationTickMsg:
		m.now = time.Now()
		m.tick++
		frameCmd := m.pumpAudio()
		eventCmd := m.pumpPlayer()
		if eventCmd != nil {
			return m, tea.Batch(m.animationTick(), eventCmd)
		}
		if frameCmd != nil {
			return m, tea.Batch(m.animationTick(), frameCmd)
		}
		return m, m.animationTick()
	case detectionTickMsg:
		if m.demo {
			return m, nil
		}
		if m.detecting {
			return m, m.detectionTick()
		}
		m.detecting = true
		return m, tea.Batch(m.detectCmd(), m.detectionTick())
	case detectionMsg:
		m.detecting = false
		return m, m.handleDetection(message)
	case playerStartedMsg:
		if message.err != nil {
			m.playerErr = message.err
			m.setNotice("mpv: "+message.err.Error(), noticeBad, 7*time.Second)
			return m, nil
		}
		m.playerErr = nil
		return m, m.applyAudioPrefsCmd()
	case playerActionMsg:
		if message.err != nil {
			m.setNotice(message.action+": "+message.err.Error(), noticeBad, 5*time.Second)
		}
		return m, nil
	case tuneMsg:
		return m, m.handleTune(message)
	case updateCheckMsg:
		if message.err == nil && message.available {
			m.updateAvailable = true
			m.updateRelease = message.release
		}
		return m, nil
	case updateAppliedMsg:
		m.updating = false
		if message.err != nil {
			m.updateErr = message.err
			return m, nil
		}
		m.updateDialog = false
		m.updateAvailable = false
		m.restartRequested = true
		return m, m.quit()
	}
	return m, nil
}

func (m *Model) handleKey(key string) tea.Cmd {
	if key == "ctrl+c" {
		return m.quit()
	}
	if m.updateDialog {
		return m.handleUpdateKey(key)
	}
	if m.helpOpen {
		if key == "esc" {
			m.helpOpen = false
			return nil
		}
		action, ok := m.bindings[key]
		if !ok {
			return nil
		}
		switch action {
		case "help":
			m.helpOpen = false
		case "quit":
			return m.quit()
		}
		return nil
	}
	action, ok := m.bindings[key]
	if !ok {
		return nil
	}
	return m.performAction(action)
}

func (m *Model) handleMouse(event tea.MouseMsg) tea.Cmd {
	mouse := tea.MouseEvent(event)
	if m.helpOpen || !mouse.IsWheel() {
		return nil
	}
	token := mouseBindingToken(mouse)
	if token == "" {
		return nil
	}
	action, ok := m.bindings[token]
	if !ok {
		return nil
	}
	return m.performAction(action)
}

func (m *Model) performAction(action string) tea.Cmd {
	switch action {
	case "quit":
		return m.quit()
	case "help":
		m.helpOpen = !m.helpOpen
		return nil
	case "mute":
		m.muted = !m.muted
		m.muteChangedAt = m.now
		notice := map[bool]string{true: "muted", false: "sound on"}[m.muted]
		if m.muteHold && !m.muted {
			notice = "held · unmute after silence"
		}
		m.setNotice(notice, noticeInfo, 2*time.Second)
		return m.setMuteCmd(m.muted)
	case "volume_up":
		return m.changeVolume(m.volumeStep)
	case "volume_down":
		return m.changeVolume(-m.volumeStep)
	case "next_radio":
		m.moveSelection(1)
		return nil
	case "previous_radio":
		m.moveSelection(-1)
		return nil
	case "play":
		return m.requestPlay(m.selected)
	case "next_visualizer":
		m.viz.Next()
		m.setNotice("visualizer · "+m.viz.Name(), noticeInfo, 2*time.Second)
		return nil
	case "previous_visualizer":
		m.viz.Prev()
		m.setNotice("visualizer · "+m.viz.Name(), noticeInfo, 2*time.Second)
		return nil
	case "toggle_autofill":
		m.autofill = !m.autofill
		if m.autofill {
			m.setNotice("auto-fill on", noticeGood, 2*time.Second)
		} else {
			m.setNotice("auto-fill off", noticeInfo, 2*time.Second)
		}
		return m.sync()
	case "retry":
		return m.restart()
	case "update":
		if m.updating {
			return nil
		}
		if !m.updateAvailable {
			m.setNotice("no update available", noticeInfo, 2*time.Second)
			return nil
		}
		m.updateDialog = true
		m.updateErr = nil
		return nil
	}
	return nil
}

func (m *Model) handleUpdateKey(key string) tea.Cmd {
	if m.updating {
		return nil
	}
	switch strings.ToLower(key) {
	case "y":
		m.updating = true
		m.updateErr = nil
		return m.applyUpdateCmd()
	case "n", "esc":
		m.updateDialog = false
	}
	return nil
}

func (m *Model) updateCheckCmd() tea.Cmd {
	if m.deps.Updater == nil {
		return nil
	}
	updater := m.deps.Updater
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		release, available, err := updater.Check(ctx)
		return updateCheckMsg{release: release, available: available, err: err}
	}
}

func (m *Model) applyUpdateCmd() tea.Cmd {
	if m.deps.Updater == nil || m.updateRelease.Version == "" {
		return nil
	}
	updater := m.deps.Updater
	release := m.updateRelease
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		return updateAppliedMsg{err: updater.Apply(ctx, release)}
	}
}

// RestartRequested tells main to exec the replaced binary after the TUI exits.
func (m *Model) RestartRequested() bool {
	return m.restartRequested
}

func (m *Model) quit() tea.Cmd {
	m.cancelTune()
	if m.deps.Analyzer != nil {
		m.deps.Analyzer.Stop()
	}
	return tea.Quit
}

func mouseBindingToken(event tea.MouseEvent) string {
	button := ""
	switch event.Button {
	case tea.MouseButtonWheelUp:
		button = "wheel_up"
	case tea.MouseButtonWheelDown:
		button = "wheel_down"
	case tea.MouseButtonWheelLeft:
		button = "wheel_left"
	case tea.MouseButtonWheelRight:
		button = "wheel_right"
	default:
		return ""
	}
	var parts []string
	if event.Ctrl {
		parts = append(parts, "ctrl")
	}
	if event.Alt {
		parts = append(parts, "alt")
	}
	if event.Shift {
		parts = append(parts, "shift")
	}
	parts = append(parts, button)
	return strings.Join(parts, "+")
}

func (m *Model) moveSelection(delta int) {
	if len(m.radios) == 0 {
		return
	}
	m.selected = (m.selected + delta + len(m.radios)) % len(m.radios)
}

func (m *Model) changeVolume(delta int) tea.Cmd {
	m.volume = clampInt(m.volume+delta, 0, 100)
	m.volumeChangedAt = m.now
	unmute := false
	if m.muted {
		m.muted = false
		m.muteChangedAt = m.now
		unmute = true
	}
	notice := fmt.Sprintf("volume · %d%%", m.volume)
	if m.muteHold && unmute {
		notice = "held · volume saved for resume"
	}
	m.setNotice(notice, noticeInfo, 2*time.Second)
	if m.demo || m.deps.Player == nil {
		return nil
	}
	playerService := m.deps.Player
	volume := m.volume
	targetMuted := m.muted || m.muteHold
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := playerService.SetVolume(ctx, volume); err != nil {
			return playerActionMsg{action: "volume", err: err}
		}
		if err := playerService.SetMute(ctx, targetMuted); err != nil {
			return playerActionMsg{action: "mute", err: err}
		}
		return playerActionMsg{action: "volume"}
	}
}

func (m *Model) setMuteCmd(muted bool) tea.Cmd {
	if m.demo || m.deps.Player == nil {
		return nil
	}
	playerService := m.deps.Player
	targetMuted := muted || m.muteHold
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := playerService.SetMute(ctx, targetMuted); err != nil {
			return playerActionMsg{action: "mute", err: err}
		}
		return playerActionMsg{action: "mute"}
	}
}

func (m *Model) applyAudioPrefsCmd() tea.Cmd {
	if m.demo || m.deps.Player == nil {
		return nil
	}
	playerService := m.deps.Player
	volume := m.volume
	muted := m.muted || m.muteHold
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := playerService.SetVolume(ctx, volume); err != nil {
			return playerActionMsg{action: "volume", err: err}
		}
		if err := playerService.SetMute(ctx, muted); err != nil {
			return playerActionMsg{action: "mute", err: err}
		}
		return playerActionMsg{action: "audio"}
	}
}

func (m *Model) handleDetection(message detectionMsg) tea.Cmd {
	if message.err != nil {
		m.probeErr = message.err
		m.setNotice("audio detection: "+message.err.Error(), noticeBad, 8*time.Second)
		return nil
	}
	m.probeErr = nil
	ownPID := 0
	if m.deps.Player != nil {
		ownPID = m.deps.Player.PID()
	}
	assessment := detect.Assess(message.processes, ownPID)
	m.assessment = assessment
	m.activity = m.tracker.Observe(assessment.Activity)
	m.ready = true
	return m.sync()
}

func (m *Model) sync() tea.Cmd {
	if m.demo {
		return nil
	}
	if !m.ready {
		return nil
	}

	switch m.activity {
	case detect.Call:
		m.resumeOnSilence = true
		if m.state == statePlaying || m.state == stateTuning {
			return m.suspend("call")
		}
		if m.state == stateSuspended {
			m.pauseReason = "call"
		}
		return nil
	case detect.OtherAudio:
		if m.state == stateSuspended {
			m.pauseReason = "audio"
		}
		if !m.autofill {
			return nil
		}
		m.resumeOnSilence = true
		if m.state == statePlaying || m.state == stateTuning {
			return m.suspend("audio")
		}
		return nil
	case detect.Silence:
		if m.state == stateSuspended {
			if m.autofill || m.resumeOnSilence {
				return m.resume()
			}
			return nil
		}
		if (m.state == stateIdle || m.state == stateRetrying) && (m.autofill || m.resumeOnSilence) && m.current >= 0 {
			if !m.now.Before(m.retryAt) {
				return m.startTune(m.current, "silence")
			}
		}
	}
	return nil
}

func (m *Model) suspend(reason string) tea.Cmd {
	m.pauseReason = reason
	m.resumeOnSilence = true
	if m.state == stateTuning {
		m.cancelTune()
		m.state = stateSuspended
		m.suspendedStop = true
		m.muteHold = false
		if m.deps.Analyzer != nil {
			m.deps.Analyzer.Stop()
		}
		// A loadfile may have raced past cancellation; a stop restores silence.
		return m.playerAction("hold", func(ctx context.Context) error {
			return m.deps.Player.Stop(ctx)
		})
	}
	if m.state != statePlaying {
		return nil
	}
	m.state = stateSuspended
	playerService := m.deps.Player

	if m.cfg.HoldMode == "pause" {
		m.muteHold = false
		m.suspendedStop = false
		if m.deps.Analyzer != nil {
			m.deps.Analyzer.Stop()
		}
		return m.playerAction("pause", func(ctx context.Context) error {
			return playerService.Pause(ctx, true)
		})
	}

	// Mute hold keeps the stream and the visualizer alive. The user's mute
	// intent is stored in m.muted; this hold is temporary.
	m.muteHold = true
	m.suspendedStop = false
	return m.playerAction("mute hold", func(ctx context.Context) error {
		return playerService.SetMute(ctx, true)
	})
}

func (m *Model) resume() tea.Cmd {
	if m.current < 0 || len(m.radios) == 0 {
		return nil
	}
	if m.state != stateSuspended && m.state != stateIdle && m.state != stateRetrying {
		return nil
	}
	if m.suspendedStop || m.loaded != m.current || m.stream.URL == "" {
		return m.startTune(m.current, "resume")
	}
	if m.muteHold {
		m.state = statePlaying
		m.muteHold = false
		m.resumeOnSilence = false
		m.pauseReason = ""
		userMuted := m.muted
		playerService := m.deps.Player
		return m.playerAction("unmute", func(ctx context.Context) error {
			return playerService.SetMute(ctx, userMuted)
		})
	}
	m.state = statePlaying
	m.resumeOnSilence = false
	m.pauseReason = ""
	if m.deps.Analyzer != nil {
		m.deps.Analyzer.Start(m.stream.URL)
	}
	playerService := m.deps.Player
	return m.playerAction("resume", func(ctx context.Context) error {
		return playerService.Pause(ctx, false)
	})
}

func (m *Model) requestPlay(index int) tea.Cmd {
	if index < 0 || index >= len(m.radios) {
		return nil
	}
	m.selected = index
	m.current = index
	if m.demo {
		m.state = statePlaying
		m.loaded = index
		m.stream = media.Stream{Title: m.radios[index].Name, Live: true}
		m.setNotice("demo · "+m.radios[index].Name, noticeGood, 3*time.Second)
		return nil
	}
	if !m.ready {
		if m.probeErr != nil {
			m.setNotice("detection offline · playing manually", noticeBad, 4*time.Second)
			return m.startTune(index, "manual")
		}
		m.resumeOnSilence = true
		m.setNotice("checking audio · armed", noticeInfo, 3*time.Second)
		return nil
	}
	if m.activity == detect.Call {
		m.resumeOnSilence = true
		m.setNotice("call in progress · queued", noticeInfo, 4*time.Second)
		return m.sync()
	}
	if m.activity == detect.OtherAudio && m.autofill {
		m.resumeOnSilence = true
		m.setNotice("other audio · queued for silence", noticeInfo, 4*time.Second)
		return m.sync()
	}
	if m.state == statePlaying && m.loaded == index {
		m.setNotice("already playing · "+m.radios[index].Name, noticeInfo, 2*time.Second)
		return nil
	}
	return m.startTune(index, "request")
}

func (m *Model) restart() tea.Cmd {
	index := m.selected
	if index < 0 || index >= len(m.radios) {
		return nil
	}
	if m.demo {
		return m.requestPlay(index)
	}
	if m.activity == detect.Call || (m.activity == detect.OtherAudio && m.autofill) {
		m.current = index
		m.resumeOnSilence = true
		m.setNotice("queued · waiting for silence", noticeInfo, 4*time.Second)
		return m.sync()
	}
	return m.startTune(index, "restart")
}

func (m *Model) startTune(index int, reason string) tea.Cmd {
	if index < 0 || index >= len(m.radios) {
		return nil
	}
	if m.demo || m.deps.Player == nil || m.deps.Resolver == nil {
		m.stream = media.Stream{Title: m.radios[index].Name, Live: true}
		m.state = statePlaying
		return nil
	}
	m.cancelTune()
	m.tuneGen++
	gen := m.tuneGen
	epoch := m.tuneEpoch.Add(1)
	ctx, cancel := context.WithCancel(context.Background())
	m.tuneCancel = cancel
	m.current = index
	m.state = stateTuning
	m.suspendedStop = false
	m.resumeOnSilence = false
	m.muteHold = false
	m.pauseReason = ""
	m.retryAt = m.now
	m.lastFrameAt = time.Time{}
	m.setNotice("tuning · "+m.radios[index].Name, noticeInfo, 6*time.Second)

	radio := m.radios[index]
	playerService := m.deps.Player
	resolver := m.deps.Resolver
	analyzer := m.deps.Analyzer
	epochPtr := &m.tuneEpoch
	lock := &m.tuneLock
	volume := m.volume
	muted := m.muted
	_ = reason

	return func() tea.Msg {
		tuneCtx, cancelTune := context.WithTimeout(ctx, 90*time.Second)
		defer cancelTune()

		if err := playerService.Start(tuneCtx); err != nil {
			return tuneMsg{gen: gen, index: index, err: err}
		}
		_ = playerService.SetVolume(tuneCtx, volume)
		_ = playerService.SetMute(tuneCtx, muted)
		stream, err := resolver.Resolve(tuneCtx, radio.URL)
		if err != nil {
			return tuneMsg{gen: gen, index: index, err: err}
		}
		if tuneCtx.Err() != nil || epochPtr.Load() != epoch {
			return tuneMsg{gen: gen, index: index, err: context.Canceled}
		}

		lock.Lock()
		if epochPtr.Load() != epoch || tuneCtx.Err() != nil {
			lock.Unlock()
			return tuneMsg{gen: gen, index: index, err: context.Canceled}
		}
		err = playerService.Play(tuneCtx, stream.URL)
		lock.Unlock()
		if err != nil {
			return tuneMsg{gen: gen, index: index, err: err}
		}
		if epochPtr.Load() == epoch && analyzer != nil {
			analyzer.Start(stream.URL)
		}
		return tuneMsg{gen: gen, index: index, stream: stream}
	}
}

func (m *Model) cancelTune() {
	if m.tuneCancel != nil {
		m.tuneCancel()
		m.tuneCancel = nil
	}
	m.tuneGen++
	m.tuneEpoch.Add(1)
}

func (m *Model) handleTune(message tuneMsg) tea.Cmd {
	if message.gen != m.tuneGen {
		return nil
	}
	if message.err != nil {
		if errors.Is(message.err, context.Canceled) {
			return nil
		}
		m.playerErr = message.err
		m.state = stateRetrying
		m.loaded = -1
		if m.retryBackoff < minRetry {
			m.retryBackoff = minRetry
		}
		delay := m.retryBackoff
		m.retryBackoff = nextBackoff(m.retryBackoff)
		m.retryAt = m.now.Add(delay)
		m.setNotice(message.err.Error()+" · retrying in "+shortDuration(delay), noticeBad, 8*time.Second)
		return nil
	}
	m.stream = message.stream
	m.current = message.index
	m.loaded = message.index
	m.state = statePlaying
	m.lastPlayAt = m.now
	m.playerErr = nil
	m.audioErr = nil
	m.suspendedStop = false
	m.resumeOnSilence = false
	m.muteHold = false
	m.retryBackoff = minRetry
	m.retryAt = m.now
	if message.stream.Title != "" {
		m.setNotice("playing · "+Sanitize(message.stream.Title), noticeGood, 3*time.Second)
	} else {
		m.setNotice("playing · "+m.radios[message.index].Name, noticeGood, 3*time.Second)
	}
	return m.sync()
}

func (m *Model) streamLost(reason string) tea.Cmd {
	if m.state != statePlaying {
		return nil
	}
	if m.deps.Analyzer != nil {
		m.deps.Analyzer.Stop()
	}
	m.state = stateRetrying
	m.loaded = -1
	if m.retryBackoff < minRetry {
		m.retryBackoff = minRetry
	}
	delay := m.retryBackoff
	m.retryBackoff = nextBackoff(m.retryBackoff)
	m.retryAt = m.now.Add(delay)
	err := errors.New(reason)
	m.playerErr = err
	m.setNotice(reason+" · retrying in "+shortDuration(delay), noticeBad, 6*time.Second)
	return nil
}

func nextBackoff(current time.Duration) time.Duration {
	if current < minRetry {
		return minRetry
	}
	next := current * 2
	if next > maxRetry {
		return maxRetry
	}
	return next
}

func (m *Model) pumpAudio() tea.Cmd {
	if m.demo {
		bands, wave := m.demoFrame()
		m.viz.Advance(bands, wave)
		return nil
	}
	if m.deps.Analyzer == nil {
		m.viz.Advance(nil, nil)
		return nil
	}
	frames := m.deps.Analyzer.Frames()
	var latest audio.Frame
	received := false
	for {
		select {
		case frame := <-frames:
			latest = frame
			received = true
		default:
			goto drained
		}
	}
drained:
	if err := m.deps.Analyzer.Err(); err != nil {
		m.audioErr = err
	} else if received {
		m.audioErr = nil
	}
	if received {
		m.lastFrameAt = m.now
	}
	if m.state == statePlaying && !m.snap.Paused && !m.lastFrameAt.IsZero() && m.now.Sub(m.lastFrameAt) > 5*time.Second && m.audioErr == nil {
		m.audioErr = errors.New("spectrum offline · no frames from ffmpeg")
	}
	if (m.state == statePlaying || m.muteHold) && !m.snap.Paused && received {
		m.viz.Advance(latest.Bands, latest.Wave)
	} else {
		m.viz.Advance(nil, nil)
	}
	return nil
}

func (m *Model) pumpPlayer() tea.Cmd {
	if m.demo || m.deps.Player == nil {
		return nil
	}
	snapshot := m.deps.Player.Snapshot()
	m.snap = snapshot
	if m.now.Sub(m.volumeChangedAt) > time.Second && snapshot.Running {
		m.volume = clampInt(snapshot.Volume, 0, 100)
	}
	if !m.muteHold && m.now.Sub(m.muteChangedAt) > time.Second && snapshot.Running {
		m.muted = snapshot.Muted
	}
	if snapshot.Error != "" {
		m.playerErr = errors.New(snapshot.Error)
	}

	var cmd tea.Cmd
	events := m.deps.Player.Events()
	for {
		select {
		case event := <-events:
			switch event.Kind {
			case player.EventFileLoaded:
			case player.EventEnded:
				if event.Reason == "eof" || event.Reason == "error" {
					if m.state == statePlaying {
						cmd = m.streamLost("stream ended")
					}
				}
			case player.EventError, player.EventExited:
				if m.state == statePlaying {
					reason := "mpv error"
					if event.Err != nil && event.Err.Error() != "" {
						reason = event.Err.Error()
					}
					cmd = m.streamLost(reason)
				}
			}
		default:
			goto drained
		}
	}
drained:
	if m.state == statePlaying && snapshot.Running && snapshot.Idle && m.now.Sub(m.lastPlayAt) > 4*time.Second {
		cmd = m.streamLost("stream ended")
	}
	if m.state == stateRetrying && m.ready && m.activity == detect.Silence && (m.autofill || m.resumeOnSilence) && !m.now.Before(m.retryAt) {
		cmd = m.startTune(m.current, "retry")
	}
	if m.state == stateIdle && m.ready && m.activity == detect.Silence && (m.autofill || m.resumeOnSilence) && m.current >= 0 && !m.now.Before(m.retryAt) {
		cmd = m.startTune(m.current, "silence")
	}
	return cmd
}

func (m *Model) playerAction(action string, actionFn func(context.Context) error) tea.Cmd {
	if m.demo || m.deps.Player == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return playerActionMsg{action: action, err: actionFn(ctx)}
	}
}

func (m *Model) demoFrame() ([]float64, []float64) {
	m.demoPhase += 0.033
	t := m.demoPhase
	bands := make([]float64, audio.BandCount)
	bumps := []float64{3, 9, 18, 31, 40}
	for i := range bands {
		value := 0.035 + 0.13*(0.5+0.5*math.Sin(t*0.7+float64(i)*0.23))
		for bumpIndex, center := range bumps {
			distance := float64(i) - center
			value += (0.42 - 0.055*float64(bumpIndex)) * math.Exp(-distance*distance/14) * (0.55 + 0.45*math.Sin(t*(1.1+0.17*float64(bumpIndex))+float64(i)*0.1))
		}
		bands[i] = clamp01(value)
	}
	wave := make([]float64, audio.WaveCount)
	for i := range wave {
		wave[i] = 0.30*math.Sin(float64(i)*0.035+t*4.6) +
			0.08*math.Sin(float64(i)*0.071-t*2.9+1.7) +
			0.04*math.Sin(float64(i)*0.019+t*1.4)
	}
	return bands, wave
}

func (m *Model) setNotice(text string, kind noticeKind, ttl time.Duration) {
	m.notice = Sanitize(text)
	m.noticeKind = kind
	m.noticeUntil = m.now.Add(ttl)
}

func (m *Model) noticeText() (string, noticeKind) {
	if m.notice == "" || m.now.After(m.noticeUntil) {
		return "", noticeInfo
	}
	return m.notice, m.noticeKind
}

func clampInt(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func shortDuration(duration time.Duration) string {
	if duration < time.Second {
		return "now"
	}
	return fmt.Sprintf("%.0fs", math.Ceil(duration.Seconds()))
}
