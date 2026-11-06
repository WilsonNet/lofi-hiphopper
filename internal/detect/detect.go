// Package detect answers one question: is this computer making or capturing sound?
package detect

import (
	"fmt"
	"sort"
	"strings"
)

// Process is one audio client: a CoreAudio process object on macOS, or a
// PipeWire/PulseAudio stream on Linux.
type Process struct {
	PID      int
	Name     string
	BundleID string
	Path     string
	Role     string
	Input    bool
	Output   bool
}

// Activity is the raw, undebounced state of the machine.
type Activity int

const (
	// Silence means no process other than our player is using audio.
	Silence Activity = iota
	// OtherAudio means something else is producing sound.
	OtherAudio
	// Call means a process has an active microphone stream.
	Call
)

func (a Activity) String() string {
	switch a {
	case Silence:
		return "silence"
	case OtherAudio:
		return "audio"
	case Call:
		return "call"
	default:
		return "unknown"
	}
}

// Assessment is a human-friendly summary of a process snapshot.
type Assessment struct {
	Activity  Activity
	CallApps  []string
	AudioApps []string
	Processes []Process
}

// Assess classifies a snapshot, ignoring our own player by PID.
func Assess(processes []Process, ownPIDs ...int) Assessment {
	return AssessWithBypass(processes, nil, ownPIDs...)
}

// AssessWithBypass classifies a snapshot while ignoring processes matching the
// bypass list. Patterns are case-insensitive substrings of process name, bundle
// ID, or executable path.
func AssessWithBypass(processes []Process, bypass []string, ownPIDs ...int) Assessment {
	own := make(map[int]struct{}, len(ownPIDs))
	for _, pid := range ownPIDs {
		if pid > 0 {
			own[pid] = struct{}{}
		}
	}
	patterns := make([]string, 0, len(bypass))
	for _, pattern := range bypass {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern != "" {
			patterns = append(patterns, pattern)
		}
	}

	assessment := Assessment{Activity: Silence, Processes: processes}
	calls := map[string]struct{}{}
	audio := map[string]struct{}{}
	for _, process := range processes {
		if _, isOurs := own[process.PID]; isOurs {
			continue
		}
		if matchesBypass(process, patterns) {
			continue
		}
		// Linux sound servers label system alerts with an event role. They
		// must not hold playback, but a stream that also records stays a call.
		if process.Output && !process.Input && isNotificationRole(process.Role) {
			continue
		}
		label := processLabel(process)
		if process.Input {
			calls[label] = struct{}{}
		}
		if process.Output {
			audio[label] = struct{}{}
		}
	}

	assessment.CallApps = sortedKeys(calls)
	assessment.AudioApps = sortedKeys(audio)
	switch {
	case len(calls) > 0:
		assessment.Activity = Call
	case len(audio) > 0:
		assessment.Activity = OtherAudio
	default:
		assessment.Activity = Silence
	}
	return assessment
}

func processLabel(process Process) string {
	name := strings.TrimSpace(process.Name)
	if name != "" {
		// Chrome helpers are noisy; the browser name is more useful.
		if index := strings.Index(name, " Helper"); index > 3 {
			name = name[:index]
		}
		return name
	}
	if bundle := strings.TrimSpace(process.BundleID); bundle != "" {
		return bundle
	}
	return fmt.Sprintf("pid %d", process.PID)
}

func sortedKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return strings.ToLower(keys[i]) < strings.ToLower(keys[j])
	})
	return keys
}

// isNotificationRole reports whether a stream carries a Linux notification
// role. PipeWire uses "Notification"; libcanberra event sounds use "event".
func isNotificationRole(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "notification", "event":
		return true
	default:
		return false
	}
}

func matchesBypass(process Process, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}
	values := []string{
		strings.ToLower(process.Name),
		strings.ToLower(process.BundleID),
		strings.ToLower(process.Path),
	}
	for _, pattern := range patterns {
		for _, value := range values {
			if value != "" && strings.Contains(value, pattern) {
				return true
			}
		}
	}
	return false
}
