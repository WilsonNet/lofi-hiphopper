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
	lookPath  func(string) (string, error)
	run       func(ctx context.Context, name string, args ...string) ([]byte, error)
	exe       func(pid int) string
	ancestors func(pid int) []string
}

// New returns the Linux sound-server probe.
func New() *Linux {
	return &Linux{
		lookPath:  exec.LookPath,
		run:       runCommand,
		exe:       procExe,
		ancestors: procAncestors,
	}
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
		if l.ancestors != nil {
			processes[i].Ancestors = l.ancestors(processes[i].PID)
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

// maxAncestorDepth bounds a /proc walk so a process wedged into a deep helper
// tree cannot turn every poll into an unbounded syscall chain.
const maxAncestorDepth = 6

// procAncestors returns the names and executable paths of pid's parents,
// nearest first. Apps such as Anki hand audio playback to a child mpv, so the
// stream owner alone is not enough to recognise the app.
func procAncestors(pid int) []string {
	var tokens []string
	seen := make(map[int]struct{}, maxAncestorDepth)
	for depth := 0; depth < maxAncestorDepth; depth++ {
		parent, ok := procParent(pid)
		if !ok || parent <= 1 {
			break
		}
		if _, duplicate := seen[parent]; duplicate {
			break
		}
		seen[parent] = struct{}{}
		if name := procName(parent); name != "" {
			tokens = append(tokens, name)
		}
		if path := procExe(parent); path != "" {
			tokens = append(tokens, path)
		}
		pid = parent
	}
	return tokens
}

// procParent reads the parent PID (field 4) from /proc/<pid>/stat. The command
// name in field 2 is parenthesised and may contain spaces or parentheses, so
// parsing starts after the final ')'.
func procParent(pid int) (int, bool) {
	if pid <= 0 {
		return 0, false
	}
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0, false
	}
	index := strings.LastIndex(string(data), ")")
	if index < 0 {
		return 0, false
	}
	fields := strings.Fields(string(data[index+1:]))
	if len(fields) < 2 {
		return 0, false
	}
	parent, err := strconv.Atoi(fields[1])
	if err != nil || parent <= 0 {
		return 0, false
	}
	return parent, true
}

// procName reads the short command name. It is capped at 15 bytes by Linux,
// which is fine for substring bypass matching.
func procName(pid int) string {
	if pid <= 0 {
		return ""
	}
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func lastLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
