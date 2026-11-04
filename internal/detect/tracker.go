package detect

// Tracker debounces noisy audio activity so notification pings do not make the
// TUI stutter. Calls are immediate: they are the one thing we must never miss.
type Tracker struct {
	requiredOther   int
	requiredSilence int
	primed          bool
	state           Activity
	otherRun        int
	silenceRun      int
}

// NewTracker returns a tracker that waits for two samples before changing to
// or from silence. The first sample is always trusted.
func NewTracker() *Tracker {
	return &Tracker{
		requiredOther:   2,
		requiredSilence: 2,
		state:           Silence,
	}
}

// Reset forgets all history, as if the app just launched.
func (t *Tracker) Reset() {
	t.primed = false
	t.state = Silence
	t.otherRun = 0
	t.silenceRun = 0
}

// Observe feeds a raw activity sample and returns the stable state.
func (t *Tracker) Observe(raw Activity) Activity {
	if raw == Call {
		t.primed = true
		t.state = Call
		t.otherRun = 0
		t.silenceRun = 0
		return t.state
	}

	if !t.primed {
		t.primed = true
		t.state = raw
		return t.state
	}

	switch raw {
	case OtherAudio:
		t.otherRun++
		t.silenceRun = 0
		if t.state != OtherAudio && t.otherRun >= t.requiredOther {
			t.state = OtherAudio
		}
	case Silence:
		t.silenceRun++
		t.otherRun = 0
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
