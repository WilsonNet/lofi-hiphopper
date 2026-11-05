//go:build linux

package detect

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const linuxProbeTimeout = 3 * time.Second

// Linux reads stream activity from PipeWire (pw-dump) and falls back to
// PulseAudio (pactl). It never records audio; it only reads stream metadata.
type Linux struct {
	lookPath func(string) (string, error)
	run      func(ctx context.Context, name string, args ...string) ([]byte, error)
	exe      func(pid int) string
}

// New returns the Linux sound-server probe.
func New() *Linux {
	return &Linux{lookPath: exec.LookPath, run: runCommand, exe: procExe}
}

// Backend names the tool the probe uses, or "" when none is installed.
func (l *Linux) Backend() string {
	if _, err := l.lookPath("pw-dump"); err == nil {
		return "pipewire"
	}
	if _, err := l.lookPath("pactl"); err == nil {
		return "pulseaudio"
	}
	return ""
}

// Processes returns one entry per active audio stream.
func (l *Linux) Processes() ([]Process, error) {
	ctx, cancel := context.WithTimeout(context.Background(), linuxProbeTimeout)
	defer cancel()

	var (
		processes []Process
		err       error
	)
	switch l.Backend() {
	case "pipewire":
		var out []byte
		out, err = l.run(ctx, "pw-dump")
		if err != nil {
			return nil, fmt.Errorf("pw-dump failed: %w (is PipeWire running?)", err)
		}
		processes, err = ParsePipeWireDump(out)
	case "pulseaudio":
		var sinks, sources []byte
		sinks, err = l.run(ctx, "pactl", "-f", "json", "list", "sink-inputs")
		if err != nil {
			return nil, fmt.Errorf("pactl failed: %w (is PulseAudio running?)", err)
		}
		sources, err = l.run(ctx, "pactl", "-f", "json", "list", "source-outputs")
		if err != nil {
			return nil, fmt.Errorf("pactl failed: %w", err)
		}
		processes, err = ParsePulseStreams(sinks, sources)
	default:
		return nil, errors.New("audio detection needs PipeWire (pw-dump) or PulseAudio (pactl)")
	}
	if err != nil {
		return nil, err
	}
	for i := range processes {
		if path := l.exe(processes[i].PID); path != "" {
			processes[i].Path = path
		}
	}
	return processes, nil
}

func runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if line := lastLine(stderr.String()); line != "" {
			return nil, fmt.Errorf("%w: %s", err, line)
		}
		return nil, err
	}
	return out, nil
}

func procExe(pid int) string {
	if pid <= 0 {
		return ""
	}
	path, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/exe")
	if err != nil {
		return ""
	}
	return path
}

func lastLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
