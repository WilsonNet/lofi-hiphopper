package player

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildArgs(t *testing.T) {
	args := buildArgs(Options{
		Volume:    120,
		UserAgent: "test-agent",
		ExtraArgs: []string{"--ao=null"},
	}, "/tmp/x.sock")
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"--idle=yes",
		"--no-video",
		"--input-ipc-server=/tmp/x.sock",
		"--volume=100",
		"--user-agent=test-agent",
		"--ao=null",
		"--ytdl=no",
		"--audio-client-name=lofi-hiphopper",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("args missing %q: %v", want, args)
		}
	}
}

func TestClampVolume(t *testing.T) {
	tests := map[int]int{-10: 0, 0: 0, 55: 55, 100: 100, 101: 100}
	for input, want := range tests {
		if got := clampVolume(input); got != want {
			t.Errorf("clampVolume(%d) = %d, want %d", input, got, want)
		}
	}
}

func TestNewDefaults(t *testing.T) {
	player := New(Options{Volume: 140})
	defer player.Close()
	snapshot := player.Snapshot()
	if snapshot.Volume != 100 || !snapshot.Idle || snapshot.Running {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	if player.PID() != 0 {
		t.Fatalf("pid = %d, want 0", player.PID())
	}
}

func TestPropertyParsers(t *testing.T) {
	values := map[string]string{
		"idle-active": "false",
		"pause":       "true",
		"mute":        "false",
		"volume":      "51.6",
		"time-pos":    "12.25",
		"duration":    "null",
		"media-title": `"a title"`,
		"path":        `"/tmp/a.mp3"`,
	}
	clientConn, serverConn := startFakeServer(t, func(reader *bufio.Reader, conn net.Conn) {
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			var request struct {
				Command   []any `json:"command"`
				RequestID int   `json:"request_id"`
			}
			if err := json.Unmarshal([]byte(line), &request); err != nil || len(request.Command) < 2 {
				return
			}
			name, _ := request.Command[1].(string)
			value, ok := values[name]
			if !ok {
				value = "null"
			}
			fmt.Fprintf(conn, `{"request_id":%d,"error":"success","data":%s}`+"\n", request.RequestID, value)
		}
	})
	defer clientConn.Close()
	defer serverConn.Close()
	client := newIPC(clientConn)
	defer client.Close()
	m := New(Options{})

	ctx := context.Background()
	if value, err := m.propertyBool(ctx, client, "pause"); err != nil || !value {
		t.Errorf("pause = %v, %v", value, err)
	}
	if value, err := m.propertyFloat(ctx, client, "volume"); err != nil || math.Abs(value-51.6) > 0.001 {
		t.Errorf("volume = %v, %v", value, err)
	}
	if value, err := m.propertyString(ctx, client, "media-title"); err != nil || value != "a title" {
		t.Errorf("media-title = %q, %v", value, err)
	}
	if _, err := m.propertyFloat(ctx, client, "duration"); err == nil {
		t.Error("null duration should return an error")
	}
}

func TestSeconds(t *testing.T) {
	if got := seconds(12.25); got != 12250*time.Millisecond {
		t.Fatalf("seconds = %s", got)
	}
}

func TestMPVIntegration(t *testing.T) {
	if os.Getenv("LOFI_MPV_TEST") == "" {
		t.Skip("set LOFI_MPV_TEST=1 to exercise a real mpv process")
	}
	tonePath := filepath.Join(t.TempDir(), "tone.wav")
	if err := writeTone(tonePath, 2.5); err != nil {
		t.Fatal(err)
	}

	player := New(Options{
		Binary:    "mpv",
		Volume:    30,
		ExtraArgs: []string{"--ao=null"},
	})
	defer player.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := player.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if player.PID() == 0 {
		t.Fatal("expected a live mpv pid")
	}
	if err := player.Play(ctx, tonePath); err != nil {
		t.Fatalf("Play: %v", err)
	}
	waitFor(t, 8*time.Second, func() bool {
		snapshot := player.Snapshot()
		return snapshot.Running && !snapshot.Idle && !snapshot.Loading
	}, "playback to start")

	waitFor(t, 5*time.Second, func() bool {
		return player.Snapshot().Position > 0
	}, "position to advance")

	if err := player.SetVolume(ctx, 42); err != nil {
		t.Fatalf("SetVolume: %v", err)
	}
	waitFor(t, 3*time.Second, func() bool { return player.Snapshot().Volume == 42 }, "volume to update")

	if err := player.SetMute(ctx, true); err != nil {
		t.Fatalf("SetMute: %v", err)
	}
	waitFor(t, 3*time.Second, func() bool { return player.Snapshot().Muted }, "mute to update")

	if err := player.Pause(ctx, true); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	waitFor(t, 3*time.Second, func() bool { return player.Snapshot().Paused }, "pause to update")

	if err := player.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	waitFor(t, 3*time.Second, func() bool { return player.Snapshot().Idle }, "stop to go idle")
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func writeTone(path string, duration float64) error {
	const sampleRate = 44100
	samples := int(duration * sampleRate)
	dataSize := samples * 2
	data := make([]byte, 44+dataSize)
	copy(data[0:4], "RIFF")
	putUint32(data[4:8], uint32(36+dataSize))
	copy(data[8:12], "WAVE")
	copy(data[12:16], "fmt ")
	putUint32(data[16:20], 16)
	putUint16(data[20:22], 1)
	putUint16(data[22:24], 1)
	putUint32(data[24:28], sampleRate)
	putUint32(data[28:32], sampleRate*2)
	putUint16(data[32:34], 2)
	putUint16(data[34:36], 16)
	copy(data[36:40], "data")
	putUint32(data[40:44], uint32(dataSize))
	for i := 0; i < samples; i++ {
		value := int16(math.Sin(2*math.Pi*440*float64(i)/sampleRate) * 0.2 * 32767)
		putUint16(data[44+i*2:], uint16(value))
	}
	return os.WriteFile(path, data, 0o644)
}

func putUint16(buffer []byte, value uint16) {
	buffer[0] = byte(value)
	buffer[1] = byte(value >> 8)
}

func putUint32(buffer []byte, value uint32) {
	buffer[0] = byte(value)
	buffer[1] = byte(value >> 8)
	buffer[2] = byte(value >> 16)
	buffer[3] = byte(value >> 24)
}
