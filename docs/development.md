# Development

## Requirements

- macOS 14.4 or newer, or Linux with PipeWire (`pw-dump`) or PulseAudio
  (`pactl`)
- Go 1.24+
- `mpv`, `yt-dlp`, `ffmpeg`

```bash
brew install mpv yt-dlp ffmpeg                      # macOS
sudo pacman -S --needed mpv yt-dlp ffmpeg pipewire  # Arch Linux
```

macOS builds need cgo for CoreAudio. Linux builds are pure Go
(`CGO_ENABLED=0` works) and shell out to `pw-dump` or `pactl`.

## Run

```bash
go build -o bin/lofi-hiphopper .
./bin/lofi-hiphopper
```

Demo mode is safe and useful for UI work:

```bash
go run . -demo
```

Use a temporary config:

```bash
go run . -config /tmp/lofi.yaml
```

## Project layout

```
main.go
internal/config      YAML defaults, partial overrides, validation
internal/detect      CoreAudio (macOS) / PipeWire + PulseAudio (Linux) probe, debounce
internal/platform    OS-specific install hints
internal/media       yt-dlp resolver
internal/player      mpv JSON IPC controller
internal/audio       ffmpeg decode + FFT
internal/ui          Bubble Tea model, canvas, layout, help, visualizers
.agents/skills       portable agent workflows
docs/                deep documentation and screenshots
```

## Adding a shortcut action

1. Add the action and label in `internal/config/config.go`.
2. Add default keys to `DefaultShortcuts()` and the default YAML.
3. Add the dispatch case in `Model.performAction`.
4. Add the action to the relevant section in `internal/ui/help.go`.
5. Add a model test and a config validation test.
6. Update `docs/configuration.md`.
7. Run the fast checks and a Herdr real test for the new control.

## Adding a visualizer

1. Add the mode constant and name in `internal/ui/visualizer.go`.
2. Add a `drawX` method that stays inside the supplied rectangle.
3. Add it to the mode cycle and `TestVisualizerDrawsEveryMode`.
4. Verify at 120x40, 60x20, 46x18, and in the help overlay.
5. Regenerate the visualizer screenshot grid.
6. Update README and docs.

## Changing config behavior

1. Update `Config`, `Default()`, and `defaultYAML`.
2. Preserve the override contract: omitted fields keep defaults; `radios`
   replaces; `shortcuts` merges by action.
3. Add tests for omitted, partial, full, and invalid cases.
4. Update `docs/configuration.md`.
5. Run `go test ./internal/config`.

## Changing playback or detection

Read `docs/architecture.md` first. Preserve these invariants:

- the player's own PID is excluded;
- calls suspend immediately;
- output yields only after debounce;
- the default stop hold stops mpv and the analyser and re-resolves on resume;
- pause mode stops the analyser and unpauses on resume;
- mute mode keeps the stream alive and restores the user's mute intent;
- analyzer failure does not stop playback;
- retries use backoff and reset on success.

Test with fakes first, then with real mpv and the real platform probe
(CoreAudio on macOS, PipeWire on Linux). Use a Herdr tab and a low-volume
config.

## Updates

The updater lives in `internal/update`. Release assets must keep the names
`lofi-hiphopper_<os>_<arch>.tar.gz` (`darwin`/`linux`, `arm64`/`amd64`) and
`checksums.txt`; `update.ArchiveName` builds the name. The TUI exposes the update check as a command so it never
blocks input. Test checks and downloads with an `httptest` server; keep the
restart path behind `syscall.Exec` in `main`.

## Screenshots

Follow the documentation skill's screenshot recipe. The standard size is
120x40 for wide shots and 60x22 for compact. Render with Charm `freeze` from
real captured ANSI. Store in `docs/images/`.

## Troubleshooting

**No sound / no start**

- Confirm a radio URL is valid.
- Run `yt-dlp -g -f bestaudio <url>` manually.
- Check `mpv <direct-url>`.
- Check the footer for detection state.

**Detection always says other audio**

- Some other app is holding an output stream.
- macOS: `go test ./internal/detect -run TestCoreAudio -v` lists processes.
- Linux: `go test ./internal/detect -run TestLinuxAudioProbe -v` lists active
  streams; `pw-dump | grep -A3 Stream/` shows the raw nodes.
- Add the culprit to `bypass_apps` if it should never interrupt lofi.
- Confirm the app's own mpv is excluded by PID.

**Visualizer offline**

- Install/upgrade ffmpeg.
- Confirm the direct manifest URL can be opened by ffmpeg.
- Playback should continue even when the analyser fails.

**YouTube errors**

```bash
brew upgrade yt-dlp       # macOS
sudo pacman -Syu yt-dlp   # Arch Linux
pipx upgrade yt-dlp       # any distro, if installed with pipx
```

Live video IDs change. Prefer a channel `/live` URL or update the radio list.
