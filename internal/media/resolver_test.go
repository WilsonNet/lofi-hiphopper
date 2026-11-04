package media

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestIsYouTube(t *testing.T) {
	tests := map[string]bool{
		"https://www.youtube.com/watch?v=abc":            true,
		"https://youtu.be/abc":                           true,
		"https://music.youtube.com/watch?v=abc":          true,
		"https://www.youtube-nocookie.com/embed/abc":     true,
		"https://www.youtube.com/@LofiGirl/live":         true,
		"https://notyoutube.com/watch?v=abc":             false,
		"https://example.com/youtube.com/watch?v=abc":    false,
		"https://example.com/audio.mp3":                  false,
		"https://sub.youtube.com.example/watch?v=abc":    false,
		"https://www.youtube.com.evil.example/watch?v=x": false,
	}
	for raw, want := range tests {
		parsed, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("parse %q: %v", raw, err)
		}
		if got := IsYouTube(parsed); got != want {
			t.Errorf("IsYouTube(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestResolvePassesThroughDirectURL(t *testing.T) {
	resolver := &Resolver{YTDLP: "/definitely/missing"}
	stream, err := resolver.Resolve(context.Background(), "https://example.com/radio.m3u8")
	if err != nil {
		t.Fatal(err)
	}
	if stream.URL != "https://example.com/radio.m3u8" || !stream.Live {
		t.Fatalf("stream = %+v", stream)
	}
}

func TestParseYTDLPBasic(t *testing.T) {
	stream, err := parseYTDLP([]byte(`{
		"title": "  lofi  radio\nbeats ",
		"url": "https://manifest.example/live.m3u8",
		"is_live": true
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if stream.URL != "https://manifest.example/live.m3u8" {
		t.Fatalf("url = %q", stream.URL)
	}
	if stream.Title != "lofi radio beats" {
		t.Fatalf("title = %q", stream.Title)
	}
	if !stream.Live || stream.Duration != 0 {
		t.Fatalf("live/duration = %v/%v", stream.Live, stream.Duration)
	}
}

func TestParseYTDLPFallsBackToRequestedFormats(t *testing.T) {
	stream, err := parseYTDLP([]byte(`{
		"title": "Song",
		"is_live": false,
		"duration": 123.5,
		"requested_formats": [
			{"url": "https://video.example/v.mp4", "acodec": "none", "vcodec": "h264"},
			{"url": "https://audio.example/a.m4a", "acodec": "aac", "vcodec": "none"}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if stream.URL != "https://audio.example/a.m4a" {
		t.Fatalf("url = %q", stream.URL)
	}
	if stream.Duration != 123500*time.Millisecond {
		t.Fatalf("duration = %s", stream.Duration)
	}
}

func TestParseYTDLPRejectsMissingURL(t *testing.T) {
	if _, err := parseYTDLP([]byte(`{"title":"x"}`)); err == nil {
		t.Fatal("expected error for missing URL")
	}
}

func TestResolveUsesYTDLP(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake is not portable to Windows")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "yt-dlp")
	script := `#!/bin/sh
for arg in "$@"; do
  if [ "$arg" = "--no-playlist" ]; then found=1; fi
done
if [ "$found" != "1" ]; then echo "missing --no-playlist" >&2; exit 2; fi
printf '%s' '{"title":"Fake stream","url":"https://audio.example/live.m3u8","is_live":true}'
`
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	resolver := &Resolver{YTDLP: fake, Timeout: 10 * time.Second}
	stream, err := resolver.Resolve(context.Background(), "https://www.youtube.com/watch?v=abc")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if stream.Title != "Fake stream" || !stream.Live {
		t.Fatalf("stream = %+v", stream)
	}
}

func TestResolveReportsYTDLPStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake is not portable to Windows")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "yt-dlp")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho 'ERROR: nope' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	resolver := &Resolver{YTDLP: fake, Timeout: 10 * time.Second}
	_, err := resolver.Resolve(context.Background(), "https://youtu.be/abc")
	if err == nil || !strings.Contains(err.Error(), "ERROR: nope") {
		t.Fatalf("error = %v", err)
	}
}

func TestResolveMissingYTDLP(t *testing.T) {
	resolver := &Resolver{YTDLP: "/definitely/missing/yt-dlp"}
	_, err := resolver.Resolve(context.Background(), "https://youtu.be/abc")
	if err == nil || !strings.Contains(err.Error(), "yt-dlp not found") {
		t.Fatalf("error = %v", err)
	}
}
