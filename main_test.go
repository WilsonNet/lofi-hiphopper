package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckTools(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	err := checkTools("mpv", "yt-dlp", "ffmpeg")
	if err == nil {
		t.Fatal("expected missing tools error")
	}
	for _, name := range []string{"mpv", "yt-dlp", "ffmpeg"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error missing %q: %v", name, err)
		}
	}

	for _, name := range []string{"mpv", "yt-dlp", "ffmpeg"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := checkTools("mpv", "yt-dlp", "ffmpeg"); err != nil {
		t.Fatalf("preexisting tools rejected: %v", err)
	}
}
