//go:build darwin && cgo

package detect

import "testing"

func TestCoreAudioProcesses(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CoreAudio integration probe in -short mode")
	}
	processes, err := New().Processes()
	if err != nil {
		t.Fatalf("CoreAudio probe failed: %v", err)
	}
	if len(processes) == 0 {
		t.Fatal("CoreAudio returned zero processes; expected at least this test process")
	}
	for _, process := range processes {
		if process.PID <= 0 {
			t.Fatalf("process has invalid pid: %+v", process)
		}
	}
	t.Logf("CoreAudio sees %d audio processes", len(processes))
}

func BenchmarkCoreAudioProcesses(b *testing.B) {
	probe := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := probe.Processes(); err != nil {
			b.Fatal(err)
		}
	}
}
