package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestDefaultIsValid(t *testing.T) {
	cfg := Default()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config invalid: %v", err)
	}
	if cfg.Radios[0].URL != "https://www.youtube.com/watch?v=rFZHOHl-L8A" {
		t.Fatalf("first radio should be the default lofi stream, got %q", cfg.Radios[0].URL)
	}
	if cfg.Volume != 55 || cfg.Poll() != DefaultPollInterval || !cfg.Autofill {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.Yield() != DefaultYieldDelay {
		t.Fatalf("yield delay = %s, want %s", cfg.Yield(), DefaultYieldDelay)
	}
	if cfg.HoldMode != DefaultHoldMode {
		t.Fatalf("hold mode = %q, want %q", cfg.HoldMode, DefaultHoldMode)
	}
}

func TestLoadMissingUsesDefaultsWithoutCreatingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Radios) != len(DefaultRadios()) {
		t.Fatalf("got %d radios, want %d", len(cfg.Radios), len(DefaultRadios()))
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("missing config should stay missing, stat error = %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Dir(path)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("missing config directory should stay missing, stat error = %v", statErr)
	}
}

func TestDefaultYAMLIsExportable(t *testing.T) {
	if !strings.Contains(DefaultYAML(), "poll_interval: 500ms") || !strings.Contains(DefaultYAML(), "yield_delay: 3s") || !strings.Contains(DefaultYAML(), "shortcuts:") {
		t.Fatal("default YAML is missing expected sections")
	}
}

func TestLoadAppliesDefaultsToOmittedFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := `radios:
  - name: One
    url: https://example.com/stream
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Volume != DefaultVolume {
		t.Errorf("volume = %d, want %d", cfg.Volume, DefaultVolume)
	}
	if cfg.Poll() != DefaultPollInterval {
		t.Errorf("poll = %s, want %s", cfg.Poll(), DefaultPollInterval)
	}
	if !cfg.Autofill {
		t.Error("autofill should default to true")
	}
}

func TestExplicitFalseAutofillSurvives(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := `autofill: false
radios:
  - name: One
    url: https://example.com/stream
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Autofill {
		t.Fatal("explicit autofill: false was overwritten by defaults")
	}
}

func TestDurationParsing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := `poll_interval: 1500ms
yield_delay: 750ms
radios:
  - name: One
    url: https://example.com/stream
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Poll(); got != 1500*time.Millisecond {
		t.Fatalf("poll = %s, want 1.5s", got)
	}
	if got := cfg.Yield(); got != 750*time.Millisecond {
		t.Fatalf("yield = %s, want 750ms", got)
	}
}

func TestZeroYieldDelayDisablesGrace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := `yield_delay: 0s
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Yield(); got != 0 {
		t.Fatalf("yield = %s, want zero", got)
	}
}

func TestValidation(t *testing.T) {
	base := Default()
	tests := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"empty radios", func(c *Config) { c.Radios = nil }, "at least one"},
		{"volume high", func(c *Config) { c.Volume = 101 }, "volume"},
		{"volume low", func(c *Config) { c.Volume = -1 }, "volume"},
		{"poll too fast", func(c *Config) { c.PollInterval = Duration(10 * time.Millisecond) }, "poll_interval"},
		{"poll too slow", func(c *Config) { c.PollInterval = Duration(time.Minute) }, "poll_interval"},
		{"yield negative", func(c *Config) { c.YieldDelay = Duration(-time.Second) }, "yield_delay"},
		{"yield too slow", func(c *Config) { c.YieldDelay = Duration(31 * time.Second) }, "yield_delay"},
		{"empty name", func(c *Config) { c.Radios[0].Name = "  " }, "name is empty"},
		{"empty url", func(c *Config) { c.Radios[0].URL = "" }, "url is empty"},
		{"bad scheme", func(c *Config) { c.Radios[0].URL = "file:///tmp/x" }, "http(s)"},
		{"no host", func(c *Config) { c.Radios[0].URL = "https:///x" }, "no host"},
		{"duplicate", func(c *Config) { c.Radios[1].URL = c.Radios[0].URL }, "duplicates"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := base
			cfg.Radios = append([]Radio(nil), base.Radios...)
			tt.mutate(&cfg)
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() = %v, want error containing %q", err, tt.want)
			}
		})
	}
}

func TestPathPrecedence(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOFI_CONFIG", "~/custom.yaml")
	if got := Path(); !strings.HasSuffix(got, "custom.yaml") || strings.HasPrefix(got, "~") {
		t.Fatalf("LOFI_CONFIG path = %q", got)
	}

	t.Setenv("LOFI_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", dir)
	want := filepath.Join(dir, appDir, "config.yaml")
	if got := Path(); got != want {
		t.Fatalf("XDG path = %q, want %q", got, want)
	}

	t.Setenv("XDG_CONFIG_HOME", "")
	got := Path()
	if !strings.HasSuffix(got, filepath.Join(".config", appDir, "config.yaml")) {
		t.Fatalf("home path = %q", got)
	}
}

func TestDefaultShortcutsAndVolumeStep(t *testing.T) {
	cfg := Default()
	if cfg.VolumeStep != DefaultVolumeStep {
		t.Fatalf("volume step = %d", cfg.VolumeStep)
	}
	if len(cfg.Shortcuts) != len(ShortcutActions()) {
		t.Fatalf("shortcuts = %d, want %d", len(cfg.Shortcuts), len(ShortcutActions()))
	}
	seen := map[string]string{}
	for _, shortcut := range cfg.Shortcuts {
		if ShortcutLabel(shortcut.Action) == "" {
			t.Errorf("action %q has no label", shortcut.Action)
		}
		for _, key := range shortcut.Keys {
			if previous, ok := seen[key]; ok {
				t.Errorf("key %q bound to %s and %s", key, previous, shortcut.Action)
			}
			seen[key] = shortcut.Action
		}
	}
	for _, want := range []string{"wheel_up", "wheel_down", "shift+wheel_down", "ctrl+wheel_down", "?"} {
		if _, ok := seen[want]; !ok {
			t.Errorf("default binding %q missing", want)
		}
	}
}

func TestPartialConfigKeepsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := `volume: 30
autofill: false
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Volume != 30 || cfg.Autofill {
		t.Fatalf("specified overrides not applied: %+v", cfg)
	}
	if cfg.VolumeStep != DefaultVolumeStep {
		t.Errorf("volume_step = %d, want default", cfg.VolumeStep)
	}
	if cfg.Poll() != DefaultPollInterval {
		t.Errorf("poll = %s, want default", cfg.Poll())
	}
	if cfg.Yield() != DefaultYieldDelay {
		t.Errorf("yield = %s, want default", cfg.Yield())
	}
	if len(cfg.Radios) != len(DefaultRadios()) {
		t.Errorf("radios = %d, want defaults", len(cfg.Radios))
	}
	if len(cfg.Shortcuts) != len(DefaultShortcuts()) {
		t.Errorf("shortcuts = %d, want defaults", len(cfg.Shortcuts))
	}
}

func TestRadiosReplaceDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := `radios:
  - name: Only station
    url: https://example.com/only
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Radios) != 1 || cfg.Radios[0].Name != "Only station" {
		t.Fatalf("radios = %+v", cfg.Radios)
	}
	if len(cfg.Shortcuts) != len(DefaultShortcuts()) {
		t.Errorf("shortcuts should keep defaults")
	}
}

func TestShortcutMerge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := `shortcuts:
  - action: mute
    keys: ["x"]
  - action: quit
    keys: ["ctrl+c", "Q"]
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	byAction := map[string][]string{}
	for _, shortcut := range cfg.Shortcuts {
		byAction[shortcut.Action] = shortcut.Keys
	}
	if got := byAction["mute"]; len(got) != 1 || got[0] != "x" {
		t.Fatalf("mute = %v", got)
	}
	if got := byAction["quit"]; len(got) != 2 || got[1] != "Q" {
		t.Fatalf("quit = %v", got)
	}
	if got := byAction["help"]; len(got) != 1 || got[0] != "?" {
		t.Fatalf("help defaults lost: %v", got)
	}
}

func TestShortcutValidation(t *testing.T) {
	base := Default()
	tests := []struct {
		name      string
		shortcuts []Shortcut
		want      string
	}{
		{"unknown action", []Shortcut{{Action: "explode", Keys: []string{"x"}}}, "unknown"},
		{"empty action", []Shortcut{{Keys: []string{"x"}}}, "action is empty"},
		{"empty keys", []Shortcut{{Action: "mute"}}, "must not be empty"},
		{"blank key", []Shortcut{{Action: "mute", Keys: []string{" "}}}, "is empty"},
		{"duplicate action", []Shortcut{{Action: "mute", Keys: []string{"x"}}, {Action: "mute", Keys: []string{"y"}}}, "duplicates"},
		{"conflicting key", []Shortcut{{Action: "mute", Keys: []string{"j"}}}, "bound to both"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := base
			cfg.Shortcuts = tt.shortcuts
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestVolumeStepValidation(t *testing.T) {
	cfg := Default()
	cfg.VolumeStep = 0
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "volume_step") {
		t.Fatalf("Validate() = %v", err)
	}
	cfg.VolumeStep = 26
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "volume_step") {
		t.Fatalf("Validate() = %v", err)
	}
}

func TestDefaultYAMLMatchesDefaults(t *testing.T) {
	got := Default()
	if err := yaml.Unmarshal([]byte(defaultYAML), &got); err != nil {
		t.Fatalf("parse default YAML: %v", err)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("validate default YAML: %v", err)
	}
	if !reflect.DeepEqual(got, Default()) {
		t.Fatalf("default YAML drifted from Default():\ngot  %+v\nwant %+v", got, Default())
	}
}

func TestHoldModeParsing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := "hold_mode: PAUSE\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HoldMode != "pause" {
		t.Fatalf("hold mode = %q", cfg.HoldMode)
	}
}

func TestHoldModeValidation(t *testing.T) {
	cfg := Default()
	cfg.HoldMode = "yield"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "hold_mode") {
		t.Fatalf("Validate() = %v", err)
	}
}

func TestBypassAppsParsing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := `bypass_apps:
  - " CoreSimulator "
  - "com.example.MyApp"
  - "coresimulator"
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"CoreSimulator", "com.example.MyApp"}
	if !reflect.DeepEqual(cfg.BypassApps, want) {
		t.Fatalf("bypass apps = %#v, want %#v", cfg.BypassApps, want)
	}
}

func TestBypassAppsRejectsEmptyEntry(t *testing.T) {
	cfg := Default()
	cfg.BypassApps = []string{" "}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "bypass_apps") {
		t.Fatalf("Validate() = %v", err)
	}
}
