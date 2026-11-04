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
	// DefaultVolumeStep is one wheel notch or volume key press.
	DefaultVolumeStep = 5
	// DefaultPollInterval is how often the audio activity probe runs.
	DefaultPollInterval = 500 * time.Millisecond
	// DefaultHoldMode keeps the stream alive but muted while yielding.
	DefaultHoldMode = "stop"
	appDir          = "lofi-hiphopper"
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

// Shortcut binds one or more key or mouse tokens to an action.
type Shortcut struct {
	Action string   `yaml:"action"`
	Keys   []string `yaml:"keys"`
}

// Config is the complete user configuration. Every field has a built-in
// default; a user config only needs to contain the parts it wants to change.
type Config struct {
	Volume       int        `yaml:"volume"`
	VolumeStep   int        `yaml:"volume_step"`
	PollInterval Duration   `yaml:"poll_interval"`
	Autofill     bool       `yaml:"autofill"`
	HoldMode     string     `yaml:"hold_mode"`
	BypassApps   []string   `yaml:"bypass_apps"`
	Radios       []Radio    `yaml:"radios"`
	Shortcuts    []Shortcut `yaml:"shortcuts"`
}

// shortcutOrder is the canonical display and merge order.
var shortcutOrder = []string{
	"play",
	"next_radio",
	"previous_radio",
	"next_visualizer",
	"previous_visualizer",
	"volume_up",
	"volume_down",
	"mute",
	"toggle_autofill",
	"retry",
	"help",
	"update",
	"quit",
}

var shortcutLabels = map[string]string{
	"play":                "play selection",
	"next_radio":          "next radio",
	"previous_radio":      "previous radio",
	"next_visualizer":     "next visualizer",
	"previous_visualizer": "previous visualizer",
	"volume_up":           "volume up",
	"volume_down":         "volume down",
	"mute":                "mute",
	"toggle_autofill":     "auto-fill",
	"retry":               "retry stream",
	"help":                "shortcut list",
	"update":              "update",
	"quit":                "quit",
}

// DefaultShortcuts includes keyboard and mouse gestures. A terminal reports
// wheel events with modifiers, so the same binding table covers both.
func DefaultShortcuts() []Shortcut {
	return []Shortcut{
		{Action: "play", Keys: []string{"enter"}},
		{Action: "next_radio", Keys: []string{"j", "down", "shift+wheel_down"}},
		{Action: "previous_radio", Keys: []string{"k", "up", "shift+wheel_up"}},
		{Action: "next_visualizer", Keys: []string{"l", "right", "ctrl+wheel_down"}},
		{Action: "previous_visualizer", Keys: []string{"h", "left", "ctrl+wheel_up"}},
		{Action: "volume_up", Keys: []string{"=", "+", "wheel_up"}},
		{Action: "volume_down", Keys: []string{"-", "_", "wheel_down"}},
		{Action: "mute", Keys: []string{"m"}},
		{Action: "toggle_autofill", Keys: []string{"a"}},
		{Action: "retry", Keys: []string{"r"}},
		{Action: "help", Keys: []string{"?"}},
		{Action: "update", Keys: []string{"u"}},
		{Action: "quit", Keys: []string{"q", "ctrl+c"}},
	}
}

// ShortcutActions returns the canonical action order.
func ShortcutActions() []string {
	return append([]string(nil), shortcutOrder...)
}

// ShortcutLabel returns a human-readable action label.
func ShortcutLabel(action string) string {
	if label, ok := shortcutLabels[action]; ok {
		return label
	}
	return action
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
		VolumeStep:   DefaultVolumeStep,
		PollInterval: Duration(DefaultPollInterval),
		Autofill:     true,
		HoldMode:     DefaultHoldMode,
		BypassApps:   []string{},
		Radios:       DefaultRadios(),
		Shortcuts:    DefaultShortcuts(),
	}
}

const defaultYAML = `# lofi-hiphopper · fills Mac silence with lofi.
# POSIX config path: ${XDG_CONFIG_HOME:-$HOME/.config}/lofi-hiphopper/config.yaml
#
# Every section is optional. Omitted values use built-in defaults; specified
# values override only that part. The first radio fills silence.
volume: 55
volume_step: 5
poll_interval: 500ms
autofill: true

# While yielding to a call or other audio:
#   stop  = stop the stream and resolve it again on return (default)
#   pause = pause the stream (live streams may resume behind the live edge)
#   mute  = keep streaming and muted, then unmute seamlessly (opt-in)
hold_mode: stop

# Apps that should never interrupt lofi. Entries match process name,
# bundle ID, or executable path, case-insensitively.
# Example: ["CoreSimulator", "com.apple.Music"]
bypass_apps: []

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

# Shortcuts are editable exactly like the radio list. Missing actions keep
# their built-in keys. Mouse gestures are bindings too.
shortcuts:
  - action: play
    keys: ["enter"]
  - action: next_radio
    keys: ["j", "down", "shift+wheel_down"]
  - action: previous_radio
    keys: ["k", "up", "shift+wheel_up"]
  - action: next_visualizer
    keys: ["l", "right", "ctrl+wheel_down"]
  - action: previous_visualizer
    keys: ["h", "left", "ctrl+wheel_up"]
  - action: volume_up
    keys: ["=", "+", "wheel_up"]
  - action: volume_down
    keys: ["-", "_", "wheel_down"]
  - action: mute
    keys: ["m"]
  - action: toggle_autofill
    keys: ["a"]
  - action: retry
    keys: ["r"]
  - action: help
    keys: ["?"]
  - action: update
    keys: ["u"]
  - action: quit
    keys: ["q", "ctrl+c"]
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

// Load reads path. A missing file is not an error and never creates anything:
// the built-in defaults stay in the binary. A file that exists only overrides
// the parts it contains.
func Load(path string) (Config, error) {
	if strings.TrimSpace(path) == "" {
		path = Path()
	}
	path = expandHome(path)

	data, readErr := os.ReadFile(path)
	if errors.Is(readErr, os.ErrNotExist) {
		cfg := Default()
		if err := cfg.Validate(); err != nil {
			return Config{}, err
		}
		return cfg, nil
	}
	if readErr != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, readErr)
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return cfg, nil
}

// DefaultYAML returns the commented built-in configuration. Nothing writes it
// automatically; users can redirect it when they choose to create a file.
func DefaultYAML() string {
	return defaultYAML
}

// Validate rejects configurations that would fail mysteriously at runtime.
func (c *Config) Validate() error {
	if c.Volume < 0 || c.Volume > 100 {
		return fmt.Errorf("volume must be between 0 and 100 (got %d)", c.Volume)
	}
	if c.VolumeStep < 1 || c.VolumeStep > 25 {
		return fmt.Errorf("volume_step must be between 1 and 25 (got %d)", c.VolumeStep)
	}
	c.HoldMode = strings.ToLower(strings.TrimSpace(c.HoldMode))
	if c.HoldMode != "stop" && c.HoldMode != "pause" && c.HoldMode != "mute" {
		return fmt.Errorf("hold_mode must be \"stop\", \"pause\", or \"mute\" (got %q)", c.HoldMode)
	}
	interval := time.Duration(c.PollInterval)
	if interval < 250*time.Millisecond || interval > 30*time.Second {
		return fmt.Errorf("poll_interval must be between 250ms and 30s (got %s)", interval)
	}
	if len(c.Radios) == 0 {
		return errors.New("radios must contain at least one entry")
	}

	seen := make(map[string]int, len(c.Radios))
	seenBypass := make(map[string]struct{}, len(c.BypassApps))
	bypass := make([]string, 0, len(c.BypassApps))
	for index, entry := range c.BypassApps {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			return fmt.Errorf("bypass_apps[%d] is empty", index)
		}
		key := strings.ToLower(entry)
		if _, exists := seenBypass[key]; exists {
			continue
		}
		seenBypass[key] = struct{}{}
		bypass = append(bypass, entry)
	}
	c.BypassApps = bypass

	shortcuts, err := normalizeShortcuts(c.Shortcuts)
	if err != nil {
		return err
	}
	c.Shortcuts = shortcuts

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

// normalizeShortcuts keeps built-in bindings for actions the user did not
// mention, while letting a listed action replace its defaults completely.
func normalizeShortcuts(user []Shortcut) ([]Shortcut, error) {
	defaults := DefaultShortcuts()
	known := make(map[string]struct{}, len(defaults))
	byAction := make(map[string]Shortcut, len(defaults))
	for _, shortcut := range defaults {
		known[shortcut.Action] = struct{}{}
		byAction[shortcut.Action] = shortcut
	}

	seenActions := make(map[string]int, len(user))
	for index, shortcut := range user {
		action := strings.TrimSpace(shortcut.Action)
		if action == "" {
			return nil, fmt.Errorf("shortcuts[%d].action is empty", index)
		}
		if _, ok := known[action]; !ok {
			return nil, fmt.Errorf("shortcuts[%d].action %q is unknown", index, action)
		}
		if previous, ok := seenActions[action]; ok {
			return nil, fmt.Errorf("shortcuts[%d] duplicates shortcuts[%d] (%s)", index, previous, action)
		}
		seenActions[action] = index
		keys := make([]string, 0, len(shortcut.Keys))
		for keyIndex, key := range shortcut.Keys {
			key = strings.TrimSpace(key)
			if key == "" {
				return nil, fmt.Errorf("shortcuts[%d].keys[%d] is empty", index, keyIndex)
			}
			keys = append(keys, key)
		}
		if len(keys) == 0 {
			return nil, fmt.Errorf("shortcuts[%d].keys must not be empty", index)
		}
		byAction[action] = Shortcut{Action: action, Keys: keys}
	}

	seenKeys := make(map[string]string, len(shortcutOrder)*3)
	result := make([]Shortcut, 0, len(shortcutOrder))
	for _, action := range shortcutOrder {
		shortcut := byAction[action]
		deduped := make([]string, 0, len(shortcut.Keys))
		for _, key := range shortcut.Keys {
			if previous, ok := seenKeys[key]; ok && previous != action {
				return nil, fmt.Errorf("key %q is bound to both %s and %s", key, previous, action)
			}
			seenKeys[key] = action
			duplicate := false
			for _, existing := range deduped {
				if existing == key {
					duplicate = true
					break
				}
			}
			if !duplicate {
				deduped = append(deduped, key)
			}
		}
		result = append(result, Shortcut{Action: action, Keys: deduped})
	}
	return result, nil
}
