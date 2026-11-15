//go:build linux

package detect

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func fakeLinux(found map[string]bool, outputs map[string]string, fail error) *Linux {
	return &Linux{
		lookPath: func(name string) (string, error) {
			if found[name] {
				return "/usr/bin/" + name, nil
			}
			return "", exec.ErrNotFound
		},
		run: func(_ context.Context, name string, args ...string) ([]byte, error) {
			if fail != nil {
				return nil, fail
			}
			return []byte(outputs[strings.Join(append([]string{name}, args...), " ")]), nil
		},
		exe: func(pid int) string {
			if pid == 2789 {
				return "/opt/brave/brave"
			}
			return ""
		},
	}
}

func TestLinuxPrefersPipeWire(t *testing.T) {
	probe := fakeLinux(map[string]bool{"pw-dump": true, "pactl": true}, map[string]string{"pw-dump": pwDumpFixture}, nil)
	if got := probe.Backend(); got != "pipewire" {
		t.Fatalf("backend = %q", got)
	}
	processes, err := probe.Processes()
	if err != nil {
		t.Fatal(err)
	}
	if len(processes) != 4 || processes[0].Path != "/opt/brave/brave" || processes[1].Path != "" {
		t.Fatalf("processes = %+v", processes)
	}
}

func TestLinuxEnrichesAncestors(t *testing.T) {
	probe := fakeLinux(map[string]bool{"pw-dump": true}, map[string]string{"pw-dump": pwDumpFixture}, nil)
	probe.ancestors = func(pid int) []string {
		if pid == 2789 {
			return []string{"anki", "/usr/local/share/anki/anki"}
		}
		return nil
	}
	processes, err := probe.Processes()
	if err != nil {
		t.Fatal(err)
	}
	if len(processes) == 0 {
		t.Fatal("no processes parsed")
	}
	if got := processes[0].Ancestors; len(got) != 2 || got[0] != "anki" {
		t.Fatalf("ancestors = %v", got)
	}
	if got := AssessWithBypass(processes[:1], []string{"anki"}).Activity; got != Silence {
		t.Fatalf("bypassing the parent app = %v, want silence", got)
	}
	if got := AssessWithBypass(processes[:1], []string{"cava"}).Activity; got != OtherAudio {
		t.Fatalf("unrelated bypass = %v, want other audio", got)
	}
}

func TestLinuxFallsBackToPulseAudio(t *testing.T) {
	probe := fakeLinux(map[string]bool{"pactl": true}, map[string]string{
		"pactl -f json list sink-inputs":    `[{"corked": false, "properties": {"application.name": "Firefox", "application.process.id": "900"}}]`,
		"pactl -f json list source-outputs": `[]`,
	}, nil)
	if got := probe.Backend(); got != "pulseaudio" {
		t.Fatalf("backend = %q", got)
	}
	processes, err := probe.Processes()
	if err != nil {
		t.Fatal(err)
	}
	if got := Assess(processes).Activity; got != OtherAudio {
		t.Fatalf("activity = %v", got)
	}
}

func TestLinuxErrors(t *testing.T) {
	if _, err := fakeLinux(nil, nil, nil).Processes(); err == nil || !strings.Contains(err.Error(), "PipeWire") {
		t.Fatalf("missing tools error = %v", err)
	}
	_, err := fakeLinux(map[string]bool{"pw-dump": true}, nil, errors.New("boom")).Processes()
	if err == nil || !strings.Contains(err.Error(), "pw-dump failed") {
		t.Fatalf("pw-dump failure = %v", err)
	}
	_, err = fakeLinux(map[string]bool{"pactl": true}, nil, errors.New("boom")).Processes()
	if err == nil || !strings.Contains(err.Error(), "pactl failed") {
		t.Fatalf("pactl failure = %v", err)
	}
}

// TestLinuxAudioProbe runs the real sound-server probe. It reads metadata
// only and plays nothing.
func TestLinuxAudioProbe(t *testing.T) {
	probe := New()
	if probe.Backend() == "" {
		t.Skip("neither pw-dump nor pactl is installed")
	}
	processes, err := probe.Processes()
	if err != nil {
		t.Fatalf("%s probe failed: %v", probe.Backend(), err)
	}
	for _, process := range processes {
		t.Logf("pid=%d name=%q path=%q input=%v output=%v", process.PID, process.Name, process.Path, process.Input, process.Output)
	}
	t.Logf("%s sees %d active streams", probe.Backend(), len(processes))
}

// TestLinuxAudioProbeSeesMPV plays a silent tone through mpv and checks that
// the probe reports it under mpv's PID, which is how own-player exclusion
// works. Run with LOFI_MPV_TEST=1.
func TestLinuxAudioProbeSeesMPV(t *testing.T) {
	if os.Getenv("LOFI_MPV_TEST") != "1" {
		t.Skip("set LOFI_MPV_TEST=1 to play a silent mpv stream")
	}
	probe := New()
	if probe.Backend() == "" {
		t.Skip("neither pw-dump nor pactl is installed")
	}
	if _, err := exec.LookPath("mpv"); err != nil {
		t.Skip("mpv is not installed")
	}
	cmd := exec.Command("mpv", "--no-config", "--no-terminal", "--volume=0",
		"--audio-client-name=lofi-hiphopper-test", "av://lavfi:sine=f=220:d=20")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		processes, err := probe.Processes()
		if err != nil {
			t.Fatal(err)
		}
		for _, process := range processes {
			if process.PID == cmd.Process.Pid && process.Output {
				own := Assess([]Process{process}, cmd.Process.Pid)
				if own.Activity != Silence {
					t.Fatalf("own mpv not excluded: %+v", own)
				}
				t.Logf("found mpv stream: %+v", process)
				return
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("mpv pid %d never appeared as an output stream", cmd.Process.Pid)
}

// TestProcAncestorsReadsParentChain walks a real process tree so the /proc
// parsing is exercised, not just the matcher.
func TestProcAncestorsReadsParentChain(t *testing.T) {
	dir := t.TempDir()
	parent := filepath.Join(dir, "anki-parent")
	body, err := os.ReadFile("/bin/sh")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(parent, body, 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(parent, "-c", "sleep 10 & echo $!; wait")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_ = cmd.Wait()
	})

	line, err := bufio.NewReader(output).ReadString('\n')
	if err != nil {
		t.Fatalf("read child pid: %v", err)
	}
	child, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatalf("parse child pid %q: %v", line, err)
	}

	ancestors := procAncestors(child)
	if len(ancestors) == 0 {
		t.Fatal("no ancestors found")
	}
	if ancestors[0] != "anki-parent" {
		t.Fatalf("nearest ancestor = %q, want anki-parent (%v)", ancestors[0], ancestors)
	}
	found := false
	for _, token := range ancestors {
		if token == parent {
			found = true
		}
	}
	if !found {
		t.Fatalf("ancestor path missing from %v", ancestors)
	}
}

// TestLinuxAudioProbeBypassesAnkiStyleParent plays silent audio from mpv whose
// parent is named "anki". Anki hands playback to a child mpv, so the bypass
// list must match the parent chain. Run with LOFI_MPV_TEST=1.
func TestLinuxAudioProbeBypassesAnkiStyleParent(t *testing.T) {
	if os.Getenv("LOFI_MPV_TEST") != "1" {
		t.Skip("set LOFI_MPV_TEST=1 to play a silent mpv stream")
	}
	mpv, err := exec.LookPath("mpv")
	if err != nil {
		t.Skip("mpv is not installed")
	}
	probe := New()
	if probe.Backend() == "" {
		t.Skip("neither pw-dump nor pactl is installed")
	}

	dir := t.TempDir()
	parent := filepath.Join(dir, "anki")
	body, err := os.ReadFile("/bin/sh")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(parent, body, 0o755); err != nil {
		t.Fatal(err)
	}

	command := fmt.Sprintf("%q --no-config --no-terminal --volume=0 --audio-client-name=lofi-bypass-test av://lavfi:anullsrc=r=48000:cl=stereo & wait", mpv)
	cmd := exec.Command(parent, "-c", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_ = cmd.Wait()
	})

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		processes, err := probe.Processes()
		if err != nil {
			t.Fatal(err)
		}
		for _, process := range processes {
			if !process.Output || !hasAncestor(process, parent) {
				continue
			}
			if got := Assess([]Process{process}).Activity; got != OtherAudio {
				t.Fatalf("raw assessment = %v, want other audio", got)
			}
			if got := AssessWithBypass([]Process{process}, []string{"anki"}).Activity; got != Silence {
				t.Fatalf("ancestor bypass = %v, want silence (%+v)", got, process)
			}
			t.Logf("bypassed mpv child stream: %+v", process)
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("mpv child of %s never appeared as an output stream", parent)
}

func hasAncestor(process Process, token string) bool {
	for _, ancestor := range process.Ancestors {
		if ancestor == token {
			return true
		}
	}
	return false
}
