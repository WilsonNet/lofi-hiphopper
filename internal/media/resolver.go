// Package media turns radio page URLs into playable direct streams.
package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

// Stream is a resolved playable URL plus display metadata.
type Stream struct {
	URL      string
	Title    string
	Live     bool
	Duration time.Duration
}

// Resolver invokes yt-dlp for YouTube URLs and passes other URLs through.
type Resolver struct {
	YTDLP     string
	UserAgent string
	Timeout   time.Duration
}

// NewResolver returns a resolver using tools from PATH.
func NewResolver() *Resolver {
	return &Resolver{
		YTDLP:     "yt-dlp",
		UserAgent: DefaultUserAgent,
		Timeout:   60 * time.Second,
	}
}

const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

type ytdlpInfo struct {
	Title            string  `json:"title"`
	URL              string  `json:"url"`
	IsLive           bool    `json:"is_live"`
	LiveStatus       string  `json:"live_status"`
	Duration         float64 `json:"duration"`
	RequestedFormats []struct {
		URL    string `json:"url"`
		VCodec string `json:"vcodec"`
		ACodec string `json:"acodec"`
	} `json:"requested_formats"`
}

// Resolve returns a direct stream URL. YouTube URLs are extracted with
// yt-dlp; other URLs are assumed to be playable as-is.
func (r *Resolver) Resolve(ctx context.Context, pageURL string) (Stream, error) {
	pageURL = strings.TrimSpace(pageURL)
	if pageURL == "" {
		return Stream{}, errors.New("empty URL")
	}
	parsed, err := url.Parse(pageURL)
	if err != nil {
		return Stream{}, fmt.Errorf("parse URL: %w", err)
	}
	if !IsYouTube(parsed) {
		stream := Stream{URL: pageURL}
		if strings.HasSuffix(strings.ToLower(parsed.Path), ".m3u8") {
			stream.Live = true
		}
		return stream, nil
	}

	binary := strings.TrimSpace(r.YTDLP)
	if binary == "" {
		binary = "yt-dlp"
	}
	if _, err := exec.LookPath(binary); err != nil {
		return Stream{}, fmt.Errorf("yt-dlp not found (install it with: brew install yt-dlp)")
	}

	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := []string{
		"--no-playlist",
		"--no-warnings",
		"--no-progress",
		"--socket-timeout", "20",
		"-j",
		"-f", "bestaudio/best",
		pageURL,
	}
	if strings.TrimSpace(r.UserAgent) != "" {
		args = append([]string{"--user-agent", r.UserAgent}, args...)
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return Stream{}, fmt.Errorf("yt-dlp timed out after %s", timeout)
		}
		detail := lastLine(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return Stream{}, fmt.Errorf("yt-dlp: %s", detail)
	}

	stream, err := parseYTDLP(stdout.Bytes())
	if err != nil {
		return Stream{}, err
	}
	if strings.TrimSpace(stream.URL) == "" {
		return Stream{}, errors.New("yt-dlp returned no playable audio URL")
	}
	return stream, nil
}

func parseYTDLP(data []byte) (Stream, error) {
	var info ytdlpInfo
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&info); err != nil {
		return Stream{}, fmt.Errorf("decode yt-dlp output: %w", err)
	}

	direct := strings.TrimSpace(info.URL)
	if direct == "" {
		for _, format := range info.RequestedFormats {
			if strings.TrimSpace(format.URL) != "" && format.ACodec != "none" {
				direct = strings.TrimSpace(format.URL)
				break
			}
		}
	}
	if direct == "" {
		return Stream{}, errors.New("yt-dlp returned no playable audio URL")
	}

	stream := Stream{
		URL:   direct,
		Title: cleanTitle(info.Title),
		Live:  info.IsLive || info.LiveStatus == "is_live",
	}
	if info.Duration > 0 && !stream.Live {
		stream.Duration = time.Duration(info.Duration * float64(time.Second))
	}
	return stream, nil
}

// IsYouTube reports whether the URL needs extractor support.
func IsYouTube(parsed *url.URL) bool {
	host := strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
	switch host {
	case "youtube.com", "m.youtube.com", "music.youtube.com", "youtu.be", "youtube-nocookie.com":
		return true
	}
	return strings.HasSuffix(host, ".youtube.com") || strings.HasSuffix(host, ".youtube-nocookie.com")
}

func cleanTitle(title string) string {
	title = strings.NewReplacer("\n", " ", "\r", " ", "\t", " ").Replace(title)
	title = strings.Join(strings.Fields(title), " ")
	runes := []rune(title)
	if len(runes) > 200 {
		title = string(runes[:200])
	}
	return strings.TrimSpace(title)
}

func lastLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(lines[len(lines)-1])
}
