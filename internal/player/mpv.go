// Package player controls a long-lived mpv process over its JSON IPC socket.
package player

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wilsonpsn/lofi-hiphopper/internal/media"
	"github.com/wilsonpsn/lofi-hiphopper/internal/platform"
)

// Snapshot is a cheap, race-free view of mpv for the UI.
type Snapshot struct {
	Running    bool
	Idle       bool
	Paused     bool
	Muted      bool
	Volume     int
	Position   time.Duration
	Duration   time.Duration
	MediaTitle string
	Path       string
	Loading    bool
	Error      string
}

// Options configures the mpv child process.
type Options struct {
	Binary    string
	Volume    int
	UserAgent string
	Socket    string
	// ExtraArgs is primarily useful for integration tests (for example
	// --ao=null).
	ExtraArgs []string
}

// MPV is the concrete player. Its methods are safe for concurrent use.
type MPV struct {
	opts    Options
	events  chan Event
	snap    atomic.Pointer[Snapshot]
	stateMu sync.Mutex

	startMu    sync.Mutex
	mu         sync.Mutex
	gen        int
	cmd        *exec.Cmd
	ipc        *ipc
	done       chan struct{}
	socket     string
	pid        int
	pollCancel context.CancelFunc
	closed     bool
}

// New creates an idle mpv controller. Call Start before Play.
func New(opts Options) *MPV {
	if strings.TrimSpace(opts.Binary) == "" {
		opts.Binary = "mpv"
	}
	opts.Volume = clampVolume(opts.Volume)
	if strings.TrimSpace(opts.UserAgent) == "" {
		opts.UserAgent = media.DefaultUserAgent
	}
	m := &MPV{
		opts:   opts,
		events: make(chan Event, 64),
	}
	initial := &Snapshot{Idle: true, Volume: opts.Volume}
	m.snap.Store(initial)
	return m
}

// Start launches mpv and connects to its IPC socket. It is idempotent.
func (m *MPV) Start(ctx context.Context) error {
	m.startMu.Lock()
	defer m.startMu.Unlock()

	if m.Snapshot().Running {
		return nil
	}
	if _, err := exec.LookPath(m.opts.Binary); err != nil {
		return fmt.Errorf("mpv not found (install it with: %s)", platform.InstallHint("mpv"))
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return errors.New("player has been closed")
	}
	m.gen++
	gen := m.gen
	m.mu.Unlock()

	socket := strings.TrimSpace(m.opts.Socket)
	if socket == "" {
		var err error
		socket, err = socketPath()
		if err != nil {
			return err
		}
	}
	_ = os.Remove(socket)

	cmd := exec.Command(m.opts.Binary, buildArgs(m.opts, socket)...)
	stderr := &tailBuffer{limit: 8 * 1024}
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start mpv: %w", err)
	}

	done := make(chan struct{})
	m.mu.Lock()
	m.cmd = cmd
	m.done = done
	m.socket = socket
	m.pid = cmd.Process.Pid
	m.mu.Unlock()

	m.update(func(s *Snapshot) {
		s.Running = true
		s.Idle = true
		s.Loading = false
		s.Error = ""
		s.Volume = m.opts.Volume
	})

	go m.waitProcess(gen, cmd, done, stderr)

	// mpv creates the socket asynchronously. Give it a generous window on slow
	// machines, but fail fast if the process dies.
	dialer := net.Dialer{Timeout: 250 * time.Millisecond}
	deadline := time.Now().Add(7 * time.Second)
	var conn net.Conn
	for {
		select {
		case <-ctx.Done():
			m.stopProcess(cmd)
			return ctx.Err()
		case <-done:
			return fmt.Errorf("mpv exited before its IPC socket was ready: %s", stderr.String())
		default:
		}
		var err error
		conn, err = dialer.DialContext(ctx, "unix", socket)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			m.stopProcess(cmd)
			return fmt.Errorf("timed out waiting for mpv IPC socket: %w", err)
		}
		select {
		case <-ctx.Done():
			m.stopProcess(cmd)
			return ctx.Err()
		case <-done:
			return fmt.Errorf("mpv exited before its IPC socket was ready: %s", stderr.String())
		case <-time.After(40 * time.Millisecond):
		}
	}

	client := newIPC(conn)
	pollCtx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	if m.gen != gen {
		m.mu.Unlock()
		cancel()
		client.Close()
		m.stopProcess(cmd)
		return errors.New("mpv start superseded")
	}
	m.ipc = client
	m.pollCancel = cancel
	m.mu.Unlock()

	go m.pollLoop(pollCtx, client, gen)
	return nil
}

func (m *MPV) waitProcess(gen int, cmd *exec.Cmd, done chan struct{}, stderr *tailBuffer) {
	err := cmd.Wait()
	close(done)

	m.mu.Lock()
	current := m.gen == gen
	var client *ipc
	if current {
		client = m.ipc
		m.ipc = nil
		if m.pollCancel != nil {
			m.pollCancel()
			m.pollCancel = nil
		}
		m.cmd = nil
		m.done = nil
		m.pid = 0
	}
	m.mu.Unlock()
	if client != nil {
		client.Close()
	}

	if !current {
		return
	}
	message := strings.TrimSpace(stderr.String())
	m.update(func(s *Snapshot) {
		s.Running = false
		s.Idle = true
		s.Paused = false
		s.Loading = false
		if err != nil {
			s.Error = "mpv exited"
			if message != "" {
				s.Error += ": " + firstLine(message)
			}
		}
	})
	kind := EventExited
	if err != nil {
		kind = EventError
	}
	event := Event{Kind: kind}
	if err != nil {
		event.Err = errors.New(firstLine(message))
	}
	m.emit(event)
}

func (m *MPV) pollLoop(ctx context.Context, client *ipc, gen int) {
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()
	failures := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		m.mu.Lock()
		if m.gen != gen || m.ipc != client {
			m.mu.Unlock()
			return
		}
		m.mu.Unlock()

		next := m.Snapshot()
		next.Running = true
		var failuresNow int
		if value, err := m.propertyBool(ctx, client, "idle-active"); err == nil {
			next.Idle = value
			if !value {
				next.Loading = false
			}
		} else {
			failuresNow++
		}
		if value, err := m.propertyBool(ctx, client, "pause"); err == nil {
			next.Paused = value
		} else {
			failuresNow++
		}
		if value, err := m.propertyBool(ctx, client, "mute"); err == nil {
			next.Muted = value
		} else {
			failuresNow++
		}
		if value, err := m.propertyFloat(ctx, client, "volume"); err == nil {
			next.Volume = clampVolume(int(value + 0.5))
		} else {
			failuresNow++
		}
		if value, err := m.propertyFloat(ctx, client, "time-pos"); err == nil && value >= 0 {
			next.Position = seconds(value)
		}
		if value, err := m.propertyFloat(ctx, client, "duration"); err == nil && value > 0 {
			next.Duration = seconds(value)
		}
		if value, err := m.propertyString(ctx, client, "media-title"); err == nil {
			next.MediaTitle = value
		}
		if value, err := m.propertyString(ctx, client, "path"); err == nil {
			next.Path = value
		}

		if failuresNow >= 4 {
			failures++
			if failures >= 3 {
				next.Error = "mpv is not responding"
			}
		} else {
			failures = 0
			next.Error = ""
		}
		m.snap.Store(&next)
	}
}

// Play asks mpv to replace the current file with uri.
func (m *MPV) Play(ctx context.Context, uri string) error {
	client := m.client()
	if client == nil {
		return errors.New("mpv is not running")
	}
	if _, err := client.Call(ctx, "loadfile", uri, "replace"); err != nil {
		return fmt.Errorf("load stream: %w", err)
	}
	if _, err := client.Call(ctx, "set_property", "pause", false); err != nil {
		return fmt.Errorf("resume playback: %w", err)
	}
	m.update(func(s *Snapshot) {
		s.Loading = true
		s.Idle = false
		s.Paused = false
		s.Position = 0
		s.Duration = 0
		s.MediaTitle = ""
		s.Error = ""
	})
	return nil
}

// Pause pauses or resumes the current file.
func (m *MPV) Pause(ctx context.Context, paused bool) error {
	client := m.client()
	if client == nil {
		return errors.New("mpv is not running")
	}
	if _, err := client.Call(ctx, "set_property", "pause", paused); err != nil {
		return fmt.Errorf("set pause: %w", err)
	}
	m.update(func(s *Snapshot) { s.Paused = paused })
	return nil
}

// Stop removes the current file but keeps mpv alive and idle.
func (m *MPV) Stop(ctx context.Context) error {
	client := m.client()
	if client == nil {
		return errors.New("mpv is not running")
	}
	if _, err := client.Call(ctx, "stop"); err != nil {
		return fmt.Errorf("stop playback: %w", err)
	}
	m.update(func(s *Snapshot) {
		s.Idle = true
		s.Paused = false
		s.Loading = false
		s.Position = 0
		s.Duration = 0
	})
	return nil
}

// SetVolume sets mpv volume from 0 to 100.
func (m *MPV) SetVolume(ctx context.Context, volume int) error {
	volume = clampVolume(volume)
	client := m.client()
	if client == nil {
		return errors.New("mpv is not running")
	}
	if _, err := client.Call(ctx, "set_property", "volume", volume); err != nil {
		return fmt.Errorf("set volume: %w", err)
	}
	m.update(func(s *Snapshot) { s.Volume = volume })
	return nil
}

// SetMute toggles mpv mute.
func (m *MPV) SetMute(ctx context.Context, muted bool) error {
	client := m.client()
	if client == nil {
		return errors.New("mpv is not running")
	}
	if _, err := client.Call(ctx, "set_property", "mute", muted); err != nil {
		return fmt.Errorf("set mute: %w", err)
	}
	m.update(func(s *Snapshot) { s.Muted = muted })
	return nil
}

// Snapshot returns the latest polled state. It never blocks.
func (m *MPV) Snapshot() Snapshot {
	if snapshot := m.snap.Load(); snapshot != nil {
		return *snapshot
	}
	return Snapshot{Idle: true}
}

// Events returns UI hints such as file-loaded and end-file.
func (m *MPV) Events() <-chan Event { return m.events }

// PID returns mpv's process id, or zero when stopped. The UI excludes this
// process from system audio detection.
func (m *MPV) PID() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pid
}

// Close gracefully quits mpv.
func (m *MPV) Close() error {
	m.startMu.Lock()
	defer m.startMu.Unlock()

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	m.gen++
	cmd := m.cmd
	client := m.ipc
	done := m.done
	socket := m.socket
	cancel := m.pollCancel
	m.pollCancel = nil
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if client != nil {
		_ = client.Notify("quit")
	}
	if done != nil {
		select {
		case <-done:
		case <-time.After(1500 * time.Millisecond):
			m.stopProcess(cmd)
		}
	}
	if client != nil {
		_ = client.Close()
	}
	if socket != "" {
		_ = os.Remove(socket)
	}
	m.mu.Lock()
	m.cmd = nil
	m.ipc = nil
	m.done = nil
	m.pid = 0
	m.mu.Unlock()
	m.update(func(s *Snapshot) {
		s.Running = false
		s.Idle = true
		s.Loading = false
	})
	return nil
}

func (m *MPV) client() *ipc {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ipc
}

func (m *MPV) update(mutate func(*Snapshot)) {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	current := m.Snapshot()
	mutate(&current)
	m.snap.Store(&current)
}

func (m *MPV) emit(event Event) {
	select {
	case m.events <- event:
	default:
	}
}

func (m *MPV) stopProcess(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func (m *MPV) propertyBool(ctx context.Context, client *ipc, name string) (bool, error) {
	raw, err := m.getProperty(ctx, client, name)
	if err != nil {
		return false, err
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, err
	}
	return value, nil
}

func (m *MPV) propertyFloat(ctx context.Context, client *ipc, name string) (float64, error) {
	raw, err := m.getProperty(ctx, client, name)
	if err != nil {
		return 0, err
	}
	if string(raw) == "null" {
		return 0, errors.New("property is null")
	}
	var value float64
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, err
	}
	return value, nil
}

func (m *MPV) propertyString(ctx context.Context, client *ipc, name string) (string, error) {
	raw, err := m.getProperty(ctx, client, name)
	if err != nil {
		return "", err
	}
	if string(raw) == "null" {
		return "", nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	return value, nil
}

func (m *MPV) getProperty(ctx context.Context, client *ipc, name string) (json.RawMessage, error) {
	callCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	return client.Call(callCtx, "get_property", name)
}

func buildArgs(opts Options, socket string) []string {
	args := []string{
		"--no-config",
		"--idle=yes",
		"--no-video",
		"--no-terminal",
		"--force-window=no",
		"--audio-display=no",
		"--no-resume-playback",
		"--keep-open=no",
		"--really-quiet",
		"--audio-client-name=lofi-hiphopper",
		"--title=lofi-hiphopper",
		"--network-timeout=20",
		"--cache=yes",
		"--cache-pause=yes",
		"--input-ipc-server=" + socket,
		"--volume=" + strconv.Itoa(clampVolume(opts.Volume)),
		"--mute=no",
		"--ytdl=no",
	}
	if strings.TrimSpace(opts.UserAgent) != "" {
		args = append(args, "--user-agent="+opts.UserAgent)
	}
	args = append(args, opts.ExtraArgs...)
	return args
}

func clampVolume(volume int) int {
	if volume < 0 {
		return 0
	}
	if volume > 100 {
		return 100
	}
	return volume
}

func seconds(value float64) time.Duration {
	return time.Duration(value * float64(time.Second))
}

func firstLine(text string) string {
	text = strings.TrimSpace(text)
	if index := strings.IndexByte(text, '\n'); index >= 0 {
		text = text[:index]
	}
	if len(text) > 300 {
		text = text[:300] + "…"
	}
	return text
}

func socketPath() (string, error) {
	name := fmt.Sprintf("lofi-hiphopper-%d-%d.sock", os.Getpid(), time.Now().UnixNano())
	path := filepath.Join(os.TempDir(), name)
	if len(path) < 100 {
		return path, nil
	}
	return filepath.Join("/tmp", name), nil
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
