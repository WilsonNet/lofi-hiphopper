package detect

import "time"

// Tracker debounces noisy audio activity so notification pings do not make the
// TUI stutter. Other audio must persist for the configured yield delay before
// it holds playback; calls are immediate: they are the one thing we must never
// miss.
type Tracker struct {
	requiredSilence int
	yieldAfter      time.Duration
	primed          bool
	state           Activity
	otherSince      time.Time
	silenceRun      int
}

// NewTracker returns a tracker that waits yieldAfter before yielding to other
// audio. The first sample is always trusted so autoplay does not start while
// audio is already playing. Calls change the state immediately.
func NewTracker(yieldAfter time.Duration) *Tracker {
	if yieldAfter < 0 {
		yieldAfter = 0
	}
	return &Tracker{
		requiredSilence: 2,
		yieldAfter:      yieldAfter,
		state:           Silence,
	}
}

// Reset forgets all history, as if the app just launched.
func (t *Tracker) Reset() {
	t.primed = false
	t.state = Silence
	t.otherSince = time.Time{}
	t.silenceRun = 0
}

// Observe feeds a raw activity sample using the current time.
func (t *Tracker) Observe(raw Activity) Activity {
	return t.ObserveAt(raw, time.Now())
}

// ObserveAt feeds a raw activity sample at a given time. Other audio only
// becomes the stable state after it has persisted for the yield delay.
func (t *Tracker) ObserveAt(raw Activity, now time.Time) Activity {
	if raw == Call {
		t.primed = true
		t.state = Call
		t.otherSince = time.Time{}
		t.silenceRun = 0
		return t.state
	}

	if !t.primed {
		t.primed = true
		t.state = raw
		if raw == OtherAudio {
			t.otherSince = now
		}
		return t.state
	}

	switch raw {
	case OtherAudio:
		t.silenceRun = 0
		if t.state == OtherAudio {
			return t.state
		}
		if t.otherSince.IsZero() {
			t.otherSince = now
		}
		if now.Sub(t.otherSince) >= t.yieldAfter {
			t.state = OtherAudio
		}
	case Silence:
		t.otherSince = time.Time{}
		t.silenceRun++
		if t.state != Silence && t.silenceRun >= t.requiredSilence {
			t.state = Silence
		}
	}
	return t.state
}

// State is the last stable state without feeding a new sample.
func (t *Tracker) State() Activity {
	if !t.primed {
		return Silence
	}
	return t.state
}
