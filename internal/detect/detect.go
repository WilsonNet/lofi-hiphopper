// Package detect answers one question: is this Mac making or capturing sound?
package detect

import (
	"fmt"
	"sort"
	"strings"
)

// Process is one CoreAudio process object.
type Process struct {
	PID      int
	Name     string
	BundleID string
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
	own := make(map[int]struct{}, len(ownPIDs))
	for _, pid := range ownPIDs {
		if pid > 0 {
			own[pid] = struct{}{}
		}
	}

	assessment := Assessment{Activity: Silence, Processes: processes}
	calls := map[string]struct{}{}
	audio := map[string]struct{}{}
	for _, process := range processes {
		if _, isOurs := own[process.PID]; isOurs {
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
