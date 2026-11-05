package detect

import "testing"

const pwDumpFixture = `[
  {"id": 30, "type": "PipeWire:Interface:Node", "info": {"state": "running", "props": {"media.class": "Audio/Sink", "node.name": "alsa_output"}}},
  {"id": 117, "type": "PipeWire:Interface:Client", "info": {"props": {"application.name": "Brave", "application.process.id": 2789, "client.api": "pipewire-pulse", "pipewire.sec.pid": 2799}}},
  {"id": 118, "type": "PipeWire:Interface:Node", "info": {"state": "running", "props": {"media.class": "Stream/Output/Audio", "client.id": 117, "application.name": "Brave", "application.process.id": "2789", "application.process.binary": "brave", "node.name": "Brave"}}},
  {"id": 107, "type": "PipeWire:Interface:Client", "info": {"props": {"application.name": "mpv", "application.process.id": 16998, "pipewire.sec.pid": 16998}}},
  {"id": 108, "type": "PipeWire:Interface:Node", "info": {"state": "running", "props": {"media.class": "Stream/Output/Audio", "client.id": 107, "node.name": "lofi-hiphopper"}}},
  {"id": 120, "type": "PipeWire:Interface:Client", "info": {"props": {"application.name": "Spotify", "pipewire.sec.pid": 4000}}},
  {"id": 121, "type": "PipeWire:Interface:Node", "info": {"state": "idle", "props": {"media.class": "Stream/Output/Audio", "client.id": 120, "application.name": "Spotify"}}},
  {"id": 130, "type": "PipeWire:Interface:Client", "info": {"props": {"application.name": "Zoom", "pipewire.sec.pid": 5000, "pipewire.access.portal.app_id": "us.zoom.Zoom"}}},
  {"id": 131, "type": "PipeWire:Interface:Node", "info": {"state": "running", "props": {"media.class": "Stream/Input/Audio", "client.id": 130}}},
  {"id": 140, "type": "PipeWire:Interface:Node", "info": {"state": "running", "props": {"media.class": "Stream/Input/Audio", "application.name": "cava", "stream.capture.sink": true}}},
  {"id": 141, "type": "PipeWire:Interface:Node", "info": {"state": "running", "props": {"media.class": "Stream/Input/Audio", "application.name": "pavucontrol", "target.object": "alsa_output.analog-stereo.monitor"}}},
  {"id": 150, "type": "PipeWire:Interface:Client", "info": {"props": {"client.api": "pipewire-pulse", "pipewire.sec.pid": 2799}}},
  {"id": 151, "type": "PipeWire:Interface:Node", "info": {"state": "running", "props": {"media.class": "Stream/Output/Audio", "client.id": 150, "node.name": "anon"}}},
  {"id": 160, "type": "PipeWire:Interface:Link", "info": {"state": "active"}},
  {"id": 161, "type": "PipeWire:Interface:Node", "info": null}
]`

func TestParsePipeWireDump(t *testing.T) {
	processes, err := ParsePipeWireDump([]byte(pwDumpFixture))
	if err != nil {
		t.Fatal(err)
	}
	want := []Process{
		{PID: 2789, Name: "Brave", Path: "brave", Output: true},
		{PID: 16998, Name: "mpv", Output: true},
		{PID: 5000, Name: "Zoom", BundleID: "us.zoom.Zoom", Input: true},
		{PID: 0, Name: "anon", Output: true},
	}
	if len(processes) != len(want) {
		t.Fatalf("got %d processes, want %d: %+v", len(processes), len(want), processes)
	}
	for i := range want {
		if processes[i] != want[i] {
			t.Errorf("process %d = %+v, want %+v", i, processes[i], want[i])
		}
	}

	assessment := Assess(processes, 16998)
	if assessment.Activity != Call {
		t.Fatalf("activity = %v, want call", assessment.Activity)
	}
	if got := assessment.AudioApps; len(got) != 2 || got[0] != "anon" || got[1] != "Brave" {
		t.Fatalf("audio apps = %v (own mpv must be excluded)", got)
	}
}

func TestParsePipeWireDumpOwnPlayerOnlyIsSilence(t *testing.T) {
	dump := `[
	  {"id": 1, "type": "PipeWire:Interface:Client", "info": {"props": {"application.process.id": 42, "pipewire.sec.pid": 42}}},
	  {"id": 2, "type": "PipeWire:Interface:Node", "info": {"state": "running", "props": {"media.class": "Stream/Output/Audio", "client.id": 1, "application.name": "mpv"}}}
	]`
	processes, err := ParsePipeWireDump([]byte(dump))
	if err != nil {
		t.Fatal(err)
	}
	if got := Assess(processes, 42).Activity; got != Silence {
		t.Fatalf("activity = %v, want silence", got)
	}
}

func TestParsePipeWireDumpErrors(t *testing.T) {
	if _, err := ParsePipeWireDump([]byte("not json")); err == nil {
		t.Fatal("expected decode error")
	}
	processes, err := ParsePipeWireDump([]byte("[]"))
	if err != nil || len(processes) != 0 {
		t.Fatalf("empty dump = %v, %v", processes, err)
	}
}

func TestParsePulseStreams(t *testing.T) {
	sinks := `[
	  {"index": 1, "corked": false, "properties": {"application.name": "Firefox", "application.process.id": "900", "application.process.binary": "firefox"}},
	  {"index": 2, "corked": true, "properties": {"application.name": "Paused", "application.process.id": "901"}},
	  {"index": 3, "corked": false, "properties": {"application.name": "mpv", "application.process.id": "42"}}
	]`
	sources := `[
	  {"index": 4, "corked": false, "properties": {"application.name": "Discord", "application.process.id": "777"}},
	  {"index": 5, "corked": false, "properties": {"application.name": "visualizer", "stream.capture.sink": "true"}}
	]`
	processes, err := ParsePulseStreams([]byte(sinks), []byte(sources))
	if err != nil {
		t.Fatal(err)
	}
	if len(processes) != 3 {
		t.Fatalf("got %+v", processes)
	}
	assessment := Assess(processes, 42)
	if assessment.Activity != Call || len(assessment.CallApps) != 1 || assessment.CallApps[0] != "Discord" {
		t.Fatalf("assessment = %+v", assessment)
	}
	if len(assessment.AudioApps) != 1 || assessment.AudioApps[0] != "Firefox" {
		t.Fatalf("audio apps = %v", assessment.AudioApps)
	}

	if _, err := ParsePulseStreams([]byte("{"), nil); err == nil {
		t.Fatal("expected decode error")
	}
	if processes, err := ParsePulseStreams(nil, []byte("  ")); err != nil || len(processes) != 0 {
		t.Fatalf("empty = %v, %v", processes, err)
	}
}

func TestBypassMatchesLinuxStreams(t *testing.T) {
	processes, err := ParsePipeWireDump([]byte(pwDumpFixture))
	if err != nil {
		t.Fatal(err)
	}
	got := AssessWithBypass(processes, []string{"zoom", "brave", "anon"}, 16998)
	if got.Activity != Silence {
		t.Fatalf("activity = %v, want silence with every app bypassed", got.Activity)
	}
}
