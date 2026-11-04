package detect

import (
	"reflect"
	"testing"
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

func TestTrackerStartupIsImmediate(t *testing.T) {
	tracker := NewTracker()
	if got := tracker.Observe(Silence); got != Silence {
		t.Fatalf("startup silence = %v", got)
	}
	tracker.Reset()
	if got := tracker.Observe(OtherAudio); got != OtherAudio {
		t.Fatalf("startup other = %v", got)
	}
}

func TestTrackerCallIsImmediateAndHeld(t *testing.T) {
	tracker := NewTracker()
	tracker.Observe(Silence)
	if got := tracker.Observe(Call); got != Call {
		t.Fatalf("call = %v, want call", got)
	}
	// A single quiet sample after a call should not resume playback.
	if got := tracker.Observe(Silence); got != Call {
		t.Fatalf("call + 1 silence = %v, want call", got)
	}
	if got := tracker.Observe(Silence); got != Silence {
		t.Fatalf("call + 2 silence = %v, want silence", got)
	}
}

func TestTrackerDebouncesNotificationPing(t *testing.T) {
	tracker := NewTracker()
	tracker.Observe(Silence)
	tracker.Observe(OtherAudio)
	if got := tracker.Observe(Silence); got != Silence {
		t.Fatalf("single ping changed state: %v", got)
	}
	if got := tracker.State(); got != Silence {
		t.Fatalf("state = %v, want silence", got)
	}
}

func TestTrackerSustainedAudioWins(t *testing.T) {
	tracker := NewTracker()
	tracker.Observe(Silence)
	tracker.Observe(OtherAudio)
	if got := tracker.Observe(OtherAudio); got != OtherAudio {
		t.Fatalf("sustained audio = %v, want other", got)
	}
	if got := tracker.Observe(Silence); got != OtherAudio {
		t.Fatalf("one gap resumed too soon: %v", got)
	}
	if got := tracker.Observe(Silence); got != Silence {
		t.Fatalf("two gaps = %v, want silence", got)
	}
}

func TestTrackerCallInterruptsAnyState(t *testing.T) {
	tracker := NewTracker()
	tracker.Observe(Silence)
	tracker.Observe(OtherAudio)
	tracker.Observe(OtherAudio)
	if got := tracker.Observe(Call); got != Call {
		t.Fatalf("call = %v", got)
	}
	tracker.Observe(OtherAudio)
	if got := tracker.Observe(OtherAudio); got != OtherAudio {
		t.Fatalf("post-call sustained audio = %v", got)
	}
}
