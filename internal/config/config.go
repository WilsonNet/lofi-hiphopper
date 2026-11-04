// Package config loads and validates lofi-hiphopper's on-disk settings.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// DefaultVolume is a friendly starting level: audible, not shouty.
	DefaultVolume = 55
	// DefaultPollInterval is how often the audio activity probe runs.
	DefaultPollInterval = time.Second
	appDir              = "lofi-hiphopper"
)

// Duration exists so YAML can say "1s" instead of nanoseconds.
type Duration time.Duration

// UnmarshalYAML decodes duration strings such as "750ms" and "2s".
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var raw string
	if err := value.Decode(&raw); err == nil {
		parsed, err := time.ParseDuration(strings.TrimSpace(raw))
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", raw, err)
		}
		*d = Duration(parsed)
		return nil
	}

	var seconds float64
	if err := value.Decode(&seconds); err != nil {
		return errors.New("duration must be a string such as 1s or 1500ms")
	}
	*d = Duration(time.Duration(seconds * float64(time.Second)))
	return nil
}

// MarshalYAML writes human-readable durations back to disk.
func (d Duration) MarshalYAML() (any, error) {
	return time.Duration(d).String(), nil
}

// Radio is one playlist entry shown in the TUI.
type Radio struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

// Config is the complete user configuration.
type Config struct {
	Volume       int      `yaml:"volume"`
	PollInterval Duration `yaml:"poll_interval"`
	Autofill     bool     `yaml:"autofill"`
	Radios       []Radio  `yaml:"radios"`
}

// DefaultRadios points at Lofi Girl's evergreen streams first, then a few
// reliable neighbours. The first entry is the stream that fills silence.
func DefaultRadios() []Radio {
	return []Radio{
		{
			Name: "Lofi Girl · lofi hip hop radio",
			URL:  "https://www.youtube.com/watch?v=rFZHOHl-L8A",
		},
		{
			Name: "Lofi Girl · live now",
			URL:  "https://www.youtube.com/@LofiGirl/live",
		},
		{
			Name: "Lofi Girl · synthwave radio",
			URL:  "https://www.youtube.com/watch?v=4xDzrJKXOOY",
		},
		{
			Name: "Chillhop · jazzy lofi beats",
			URL:  "https://www.youtube.com/watch?v=5yx6BWlEVcY",
		},
		{
			Name: "Relaxing jazz piano radio",
			URL:  "https://www.youtube.com/watch?v=Dx5qFachd3A",
		},
	}
}

// Default returns a fully populated configuration.
func Default() Config {
	return Config{
		Volume:       DefaultVolume,
		PollInterval: Duration(DefaultPollInterval),
		Autofill:     true,
		Radios:       DefaultRadios(),
	}
}

const defaultYAML = `# lofi-hiphopper · fills Mac silence with lofi.
# Add any YouTube video, live stream, or direct audio URL.
# The first radio is the default silence filler.
volume: 55
poll_interval: 1s
autofill: true

radios:
  - name: Lofi Girl · lofi hip hop radio
    url: https://www.youtube.com/watch?v=rFZHOHl-L8A
  - name: Lofi Girl · live now
    url: https://www.youtube.com/@LofiGirl/live
  - name: Lofi Girl · synthwave radio
    url: https://www.youtube.com/watch?v=4xDzrJKXOOY
  - name: Chillhop · jazzy lofi beats
    url: https://www.youtube.com/watch?v=5yx6BWlEVcY
  - name: Relaxing jazz piano radio
    url: https://www.youtube.com/watch?v=Dx5qFachd3A
`

// Path resolves the config location. LOFI_CONFIG wins, then XDG_CONFIG_HOME,
// then ~/.config. macOS CLIs feel more at home in ~/.config than Application
// Support, so we intentionally do not use os.UserConfigDir.
func Path() string {
	if custom := strings.TrimSpace(os.Getenv("LOFI_CONFIG")); custom != "" {
		return expandHome(custom)
	}
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(expandHome(xdg), appDir, "config.yaml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", appDir, "config.yaml")
	}
	return filepath.Join(home, ".config", appDir, "config.yaml")
}

func expandHome(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// LoadOrCreate reads path, writing the commented default on first launch.
func LoadOrCreate(path string) (cfg Config, created bool, err error) {
	if strings.TrimSpace(path) == "" {
		path = Path()
	}
	path = expandHome(path)

	data, readErr := os.ReadFile(path)
	if errors.Is(readErr, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return Config{}, false, fmt.Errorf("create config directory: %w", err)
		}
		if err := os.WriteFile(path, []byte(defaultYAML), 0o644); err != nil {
			return Config{}, false, fmt.Errorf("write default config: %w", err)
		}
		data = []byte(defaultYAML)
		created = true
	} else if readErr != nil {
		return Config{}, false, fmt.Errorf("read config %s: %w", path, readErr)
	}

	cfg = Default()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, created, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, created, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return cfg, created, nil
}

// Validate rejects configurations that would fail mysteriously at runtime.
func (c *Config) Validate() error {
	if c.Volume < 0 || c.Volume > 100 {
		return fmt.Errorf("volume must be between 0 and 100 (got %d)", c.Volume)
	}
	interval := time.Duration(c.PollInterval)
	if interval < 250*time.Millisecond || interval > 30*time.Second {
		return fmt.Errorf("poll_interval must be between 250ms and 30s (got %s)", interval)
	}
	if len(c.Radios) == 0 {
		return errors.New("radios must contain at least one entry")
	}

	seen := make(map[string]int, len(c.Radios))
	for i := range c.Radios {
		radio := &c.Radios[i]
		radio.Name = strings.TrimSpace(radio.Name)
		radio.URL = strings.TrimSpace(radio.URL)
		if radio.Name == "" {
			return fmt.Errorf("radios[%d].name is empty", i)
		}
		if radio.URL == "" {
			return fmt.Errorf("radios[%d].url is empty", i)
		}
		parsed, err := url.Parse(radio.URL)
		if err != nil {
			return fmt.Errorf("radios[%d].url: %w", i, err)
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return fmt.Errorf("radios[%d].url must be http(s), got %q", i, radio.URL)
		}
		if parsed.Host == "" {
			return fmt.Errorf("radios[%d].url has no host: %q", i, radio.URL)
		}
		if previous, ok := seen[radio.URL]; ok {
			return fmt.Errorf("radios[%d] duplicates radios[%d] (%s)", i, previous, radio.URL)
		}
		seen[radio.URL] = i
	}
	return nil
}

// Poll returns the probe interval as a standard duration.
func (c Config) Poll() time.Duration {
	if c.PollInterval <= 0 {
		return DefaultPollInterval
	}
	return time.Duration(c.PollInterval)
}
