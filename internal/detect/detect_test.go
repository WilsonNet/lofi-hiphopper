package detect

import (
	"reflect"
	"testing"
	"time"
)

func TestAssessSilenceIgnoresOwnPlayer(t *testing.T) {
	got := Assess([]Process{
		{PID: 100, Name: "mpv", Output: true},
	}, 100)
	if got.Activity != Silence {
		t.Fatalf("activity = %v, want silence", got.Activity)
	}
	if len(got.AudioApps) != 0 || len(got.CallApps) != 0 {
		t.Fatalf("own player leaked into apps: %+v", got)
	}
}

func TestAssessOtherAudio(t *testing.T) {
	got := Assess([]Process{
		{PID: 100, Name: "mpv", Output: true},
		{PID: 200, Name: "Spotify", BundleID: "com.spotify.client", Output: true},
	}, 100)
	if got.Activity != OtherAudio {
		t.Fatalf("activity = %v, want other audio", got.Activity)
	}
	if !reflect.DeepEqual(got.AudioApps, []string{"Spotify"}) {
		t.Fatalf("audio apps = %v", got.AudioApps)
	}
}

func TestAssessCallWinsOverOutput(t *testing.T) {
	got := Assess([]Process{
		{PID: 200, Name: "zoom.us", Output: true, Input: true},
		{PID: 300, Name: "Spotify", Output: true},
	})
	if got.Activity != Call {
		t.Fatalf("activity = %v, want call", got.Activity)
	}
	if !reflect.DeepEqual(got.CallApps, []string{"zoom.us"}) {
		t.Fatalf("call apps = %v", got.CallApps)
	}
}

func TestAssessGroupsChromeHelpers(t *testing.T) {
	got := Assess([]Process{
		{PID: 1, Name: "Google Chrome Helper (Renderer)", Output: true},
		{PID: 2, Name: "Google Chrome Helper (GPU)", Output: true},
	})
	if !reflect.DeepEqual(got.AudioApps, []string{"Google Chrome"}) {
		t.Fatalf("audio apps = %v", got.AudioApps)
	}
}

func TestAssessFallsBackToBundleOrPID(t *testing.T) {
	got := Assess([]Process{
		{PID: 7, BundleID: "com.example.audio", Output: true},
		{PID: 8, Output: true},
	})
	want := []string{"com.example.audio", "pid 8"}
	if !reflect.DeepEqual(got.AudioApps, want) {
		t.Fatalf("audio apps = %v, want %v", got.AudioApps, want)
	}
}

func TestAssessIgnoresLinuxNotificationRoles(t *testing.T) {
	got := Assess([]Process{
		{PID: 1, Name: "Notifications", Role: "Notification", Output: true},
		{PID: 2, Name: "libcanberra", Role: "event", Output: true},
		{PID: 3, Name: "Spotify", Role: "Music", Output: true},
	})
	if got.Activity != OtherAudio {
		t.Fatalf("activity = %v, want other audio", got.Activity)
	}
	if !reflect.DeepEqual(got.AudioApps, []string{"Spotify"}) {
		t.Fatalf("audio apps = %v, want only Spotify", got.AudioApps)
	}
}

func TestAssessNotificationRoleWithInputStaysCall(t *testing.T) {
	got := Assess([]Process{
		{PID: 1, Name: "Alert call", Role: "Notification", Input: true, Output: true},
	})
	if got.Activity != Call {
		t.Fatalf("activity = %v, want call", got.Activity)
	}
}

func TestTrackerStartupIsImmediate(t *testing.T) {
	tracker := NewTracker(3 * time.Second)
	at := time.Unix(100, 0)
	if got := tracker.ObserveAt(Silence, at); got != Silence {
		t.Fatalf("startup silence = %v", got)
	}
	tracker.Reset()
	if got := tracker.ObserveAt(OtherAudio, at); got != OtherAudio {
		t.Fatalf("startup other = %v", got)
	}
}

func TestTrackerCallIsImmediateAndHeld(t *testing.T) {
	tracker := NewTracker(3 * time.Second)
	at := time.Unix(100, 0)
	tracker.ObserveAt(Silence, at)
	if got := tracker.ObserveAt(Call, at.Add(time.Second)); got != Call {
		t.Fatalf("call = %v, want call", got)
	}
	// A single quiet sample after a call should not resume playback.
	if got := tracker.ObserveAt(Silence, at.Add(2*time.Second)); got != Call {
		t.Fatalf("call + 1 silence = %v, want call", got)
	}
	if got := tracker.ObserveAt(Silence, at.Add(3*time.Second)); got != Silence {
		t.Fatalf("call + 2 silence = %v, want silence", got)
	}
}

func TestTrackerNotificationPingIsIgnored(t *testing.T) {
	tracker := NewTracker(3 * time.Second)
	at := time.Unix(100, 0)
	tracker.ObserveAt(Silence, at)
	if got := tracker.ObserveAt(OtherAudio, at.Add(time.Second)); got != Silence {
		t.Fatalf("first ping sample changed state: %v", got)
	}
	if got := tracker.ObserveAt(OtherAudio, at.Add(2*time.Second)); got != Silence {
		t.Fatalf("short alert changed state: %v", got)
	}
	if got := tracker.ObserveAt(Silence, at.Add(2500*time.Millisecond)); got != Silence {
		t.Fatalf("alert tail changed state: %v", got)
	}
	if got := tracker.State(); got != Silence {
		t.Fatalf("state = %v, want silence", got)
	}
}

func TestTrackerGraceResetsAfterAnAudioGap(t *testing.T) {
	tracker := NewTracker(3 * time.Second)
	at := time.Unix(100, 0)
	tracker.ObserveAt(Silence, at)
	tracker.ObserveAt(OtherAudio, at.Add(time.Second))
	tracker.ObserveAt(OtherAudio, at.Add(3*time.Second))
	if got := tracker.ObserveAt(Silence, at.Add(3500*time.Millisecond)); got != Silence {
		t.Fatalf("gap should keep silence, got %v", got)
	}
	if got := tracker.ObserveAt(OtherAudio, at.Add(4*time.Second)); got != Silence {
		t.Fatalf("gap did not reset the grace timer: %v", got)
	}
	if got := tracker.ObserveAt(OtherAudio, at.Add(7*time.Second)); got != OtherAudio {
		t.Fatalf("sustained audio after reset = %v, want other", got)
	}
}

func TestTrackerSustainedAudioWins(t *testing.T) {
	tracker := NewTracker(3 * time.Second)
	at := time.Unix(100, 0)
	tracker.ObserveAt(Silence, at)
	tracker.ObserveAt(OtherAudio, at.Add(time.Second))
	if got := tracker.ObserveAt(OtherAudio, at.Add(3*time.Second)); got != Silence {
		t.Fatalf("audio yielded before the grace period: %v", got)
	}
	if got := tracker.ObserveAt(OtherAudio, at.Add(4*time.Second)); got != OtherAudio {
		t.Fatalf("sustained audio = %v, want other", got)
	}
	if got := tracker.ObserveAt(Silence, at.Add(4500*time.Millisecond)); got != OtherAudio {
		t.Fatalf("one gap resumed too soon: %v", got)
	}
	if got := tracker.ObserveAt(Silence, at.Add(5*time.Second)); got != Silence {
		t.Fatalf("two gaps = %v, want silence", got)
	}
}

func TestTrackerCallInterruptsAnyState(t *testing.T) {
	tracker := NewTracker(3 * time.Second)
	at := time.Unix(100, 0)
	tracker.ObserveAt(Silence, at)
	tracker.ObserveAt(OtherAudio, at.Add(time.Second))
	tracker.ObserveAt(OtherAudio, at.Add(4*time.Second))
	if got := tracker.ObserveAt(Call, at.Add(5*time.Second)); got != Call {
		t.Fatalf("call = %v", got)
	}
	tracker.ObserveAt(OtherAudio, at.Add(6*time.Second))
	if got := tracker.ObserveAt(OtherAudio, at.Add(7*time.Second)); got != Call {
		t.Fatalf("post-call audio should still be inside grace: %v", got)
	}
	if got := tracker.ObserveAt(OtherAudio, at.Add(9*time.Second)); got != OtherAudio {
		t.Fatalf("post-call sustained audio = %v", got)
	}
}

func TestTrackerZeroGraceYieldsOnFirstSample(t *testing.T) {
	tracker := NewTracker(0)
	at := time.Unix(100, 0)
	tracker.ObserveAt(Silence, at)
	if got := tracker.ObserveAt(OtherAudio, at.Add(time.Second)); got != OtherAudio {
		t.Fatalf("zero grace = %v, want other", got)
	}
}

func TestAssessWithBypassIgnoresMatchingAudio(t *testing.T) {
	processes := []Process{
		{PID: 12536, Name: "ExampleApp", BundleID: "com.example.MyApp", Path: "/Users/me/Library/Developer/CoreSimulator/Devices/X/ExampleApp.app/ExampleApp", Output: true},
		{PID: 5000, Name: "Spotify", BundleID: "com.spotify.client", Output: true},
	}
	got := AssessWithBypass(processes, []string{"coresimulator"})
	if got.Activity != OtherAudio {
		t.Fatalf("activity = %v, want other audio from Spotify", got.Activity)
	}
	if !reflect.DeepEqual(got.AudioApps, []string{"Spotify"}) {
		t.Fatalf("audio apps = %v", got.AudioApps)
	}
	got = AssessWithBypass(processes[:1], []string{"com.example.MyApp"})
	if got.Activity != Silence {
		t.Fatalf("bundle bypass failed: %v", got.Activity)
	}
	got = AssessWithBypass(processes[:1], []string{"EXAMPLEAPP"})
	if got.Activity != Silence {
		t.Fatalf("name bypass is not case-insensitive: %v", got.Activity)
	}
}

func TestBypassAlsoSilencesInput(t *testing.T) {
	got := AssessWithBypass([]Process{
		{PID: 1, Name: "ExampleApp", BundleID: "com.example.MyApp", Path: "/CoreSimulator/ExampleApp.app/ExampleApp", Input: true, Output: true},
	}, []string{"CoreSimulator"})
	if got.Activity != Silence {
		t.Fatalf("bypassed input activity = %v", got.Activity)
	}
}
