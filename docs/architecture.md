# Architecture

This page is for maintainers. For user setup, start with the
[README](../README.md); for every config field, see
[configuration](configuration.md).

## Data flow

```
config.yaml
    │
    ▼
┌─────────────┐     ┌──────────────────┐
│ CoreAudio / │     │ Bubble Tea model │◄──── keyboard / mouse
│ PipeWire    │────►│  auto-fill       │
│ probe       │     │  state machine   │
└─────────────┘     └──────┬───────────┘
                           │ commands
             ┌─────────────┼──────────────┐
             ▼             ▼              ▼
        ┌─────────┐   ┌─────────┐   ┌────────────┐
        │ yt-dlp  │   │  mpv    │   │  ffmpeg    │
        │ resolve │   │  play   │   │  analyse   │
        └─────────┘   └─────────┘   └─────┬──────┘
                                           │ PCM
                                           ▼
                                    FFT → 48 bands
                                           │
                                           ▼
                                     Visualizer
```

Playback and analysis are independent. The analyser is a second, silent decode
of the same stream; if it fails, playback continues and the visualizer reports
that it is offline.

## Packages

| Package | Responsibility |
| --- | --- |
| `internal/config` | YAML defaults, partial overrides, validation, shortcut merge |
| `internal/detect` | CoreAudio (macOS) and PipeWire/PulseAudio (Linux) probes, classification, debounce |
| `internal/platform` | OS-specific dependency install hints |
| `internal/media` | yt-dlp invocation and direct URL resolution |
| `internal/player` | mpv process lifecycle and JSON IPC |
| `internal/audio` | ffmpeg PCM decode, FFT, bands, waveform frames |
| `internal/update` | GitHub release check, checksum verification, binary replacement |
| `internal/ui` | Bubble Tea model, canvas, layout, help, six visualizers |
| `main.go` | flags, dependency checks, process wiring, cleanup |

## Configuration model

`config.Default()` is the single source of built-in defaults. `Load` reads the
file only when it exists and never writes it. Missing fields keep their
defaults. `radios` replaces the default slice when present. `shortcuts` is
normalized by action: defaults fill every action first, then user entries
replace matching actions. Duplicate keys are rejected across actions.

The commented YAML is embedded in the binary for `-print-config`. It is never
written automatically; a user redirects that output only when they choose to
create a config.

## Audio activity detection

Each platform probe returns the same `[]detect.Process` shape, so
classification, bypass, and debounce are shared.

### macOS: CoreAudio

`internal/detect/coreaudio_darwin.go` calls the macOS HAL process-object API
through cgo. For each process it reads:

- PID;
- process name;
- bundle ID;
- `kAudioProcessPropertyIsRunningInput`;
- `kAudioProcessPropertyIsRunningOutput`.

The detector needs macOS 14.4+. A non-cgo build returns a clear error.

### Linux: PipeWire, then PulseAudio

`internal/detect/linux.go` runs `pw-dump` once per poll (about 25ms) and
`internal/detect/pipewire.go` parses the JSON. Each `Stream/Output/Audio` or
`Stream/Input/Audio` node becomes one process entry:

- active only while the node `state` is `running`; paused and corked streams
  go `idle` and do not count;
- PID comes from the node's `application.process.id`, then the owning client's
  `application.process.id`, then the client's `pipewire.sec.pid` for native
  PipeWire clients (pipewire-pulse clients share the server's socket PID, so
  that fallback is skipped for them);
- name is `application.name`, then `node.name`; bundle ID is `application.id`
  or the Flatpak app ID; path is `/proc/<pid>/exe`, then
  `application.process.binary`;
- capture streams that read a sink monitor (`stream.capture.sink`,
  `stream.monitor`, or a `.monitor` target) are desktop-audio visualizers or
  recorders, not microphones, and are skipped.

mpv uses PipeWire natively and its client reports mpv's PID, so own-player
exclusion works unchanged. When `pw-dump` is missing, the probe runs
`pactl -f json list sink-inputs` and `source-outputs` and treats uncorked
streams as active. Without either tool detection reports offline and manual play
still works. Linux builds need no cgo.

### Classification

`AssessWithBypass` filters out the player's own mpv PID and any process matching
`bypass_apps` (case-insensitive substring of name, bundle ID, or executable
path), then classifies:

- any input process -> `Call`;
- otherwise any output process -> `OtherAudio`;
- otherwise `Silence`.

The bypass list is for simulator/helper audio that should never interrupt lofi.
It applies to both output and input, so list only apps you trust to ignore.

`Tracker` debounces output/silence transitions for two samples. A call is
immediate. This is what prevents notification pings from pausing the music.

The app never records audio. It only reads process flags and stream metadata.

## Playback state machine

Player states are `idle`, `tuning`, `playing`, `suspended`, and `retrying`.

- Startup waits for the first detector sample before auto-starting.
- `Silence` plus auto-fill starts the current radio.
- `OtherAudio` with auto-fill on suspends playback; with auto-fill off it keeps
  playing unless a call is active.
- `Call` always suspends.
- The default `hold_mode: stop` stops mpv and the analyser, then resolves and
  plays again on silence.
- `hold_mode: pause` pauses mpv and stops the analyser; resume restarts the
  analyser and unpauses. A paused livestream may be behind the live edge.
- `hold_mode: mute` keeps mpv and the analyser running, mutes the output, and
  unmutes on resume. The user's manual mute intent is preserved.
- Stream loss schedules exponential retries (3s → 30s). A successful resolve
  resets the backoff.
- Tuning is cancellable. An epoch and mutex prevent a slow resolve from
  overwriting a newer selection.

Suspension sets `resumeOnSilence`, so playback that existed before a call
returns after the call even when auto-fill is off. Explicit queued requests also
resume when silence returns.

## mpv controller

`internal/player` launches one long-lived mpv:

```
mpv --no-config --idle=yes --no-video --no-terminal \
    --input-ipc-server=<socket> --volume=55 ...
```

It connects to the Unix socket and speaks mpv's line-delimited JSON IPC.
Commands carry request IDs; events are buffered as UI hints. A 400ms poller
reads `idle-active`, `pause`, `mute`, `volume`, `time-pos`, `duration`,
`media-title`, and `path` into an atomic snapshot.

`--audio-client-name=lofi-hiphopper` makes the app identifiable in macOS audio
controls and in PipeWire tools such as `pavucontrol` or `wpctl status`. The player PID is what the detector excludes.

Socket paths are kept under the platform temporary directory and checked against
the Unix socket path limit. The child is closed with `quit`, then killed if it
does not exit. The UI never blocks on IPC; commands run in Bubble Tea commands.

## Stream resolution

`internal/media` detects YouTube hosts and invokes:

```
yt-dlp --no-playlist --no-warnings --no-progress --socket-timeout 20 \
       -j -f bestaudio/best <url>
```

The direct manifest URL is passed to both mpv and ffmpeg. Non-YouTube URLs pass
through unchanged. Errors keep the last stderr line for the UI. yt-dlp is
required for YouTube and can be upgraded independently when YouTube changes.

## Spectrum analyser

`internal/audio` launches:

```
ffmpeg -re -i <manifest> -vn -ac 1 -ar 44100 -f s16le pipe:1
```

It reads 1024-sample hops into a 2048-sample ring. A Hann-windowed radix-2 FFT
produces magnitudes. Forty-eight logarithmically spaced bands cover 40 Hz to
16 kHz. Values are normalized in dB with a mild high-frequency tilt. A separate
RMS gate removes silence. A 512-sample window is downsampled to 256 points for
the scope.

Frames are published through a one-element channel that always keeps the newest
frame. A generation counter prevents an old analyser process from feeding a new
selection. `-re` keeps analysis close to real time rather than decoding a file
at maximum speed.

## UI and rendering

The UI is a Bubble Tea model with an immutable dependency set (player, analyser,
resolver, probe). Inputs become actions; actions produce commands; commands
update state with typed messages.

Rendering uses a custom cell canvas rather than per-widget style strings. Each
cell has a rune, foreground, background, and bold flag. `Render` emits grouped
ANSI truecolor sequences. This makes gradients, braille line art, and layered
transparency practical at 30 FPS. Panels, borders, rulers, and modal help are
all drawn into the same canvas.

The visualizers consume smoothed bands with fast attack and slow release:
`BARS`, `MIRROR`, `SCOPE`, `HALO`, `AURORA`, and `MATRIX`. All six render
inside the same rectangle and adapt to compact layouts. Peak caps fall with
gravity; the dominant frequency is shown in the panel header.

Mouse wheel events come from `tea.WithMouseCellMotion`. Modifier bits are
translated into the same binding tokens as keyboard keys, so shortcuts and mouse
gestures share one editable table.

## Updates

`internal/update` checks `GET /repos/WilsonNet/lofi-hiphopper/releases/latest`
with a short timeout. The UI runs this check as a Bubble Tea command at launch;
network failures are silent and never block the TUI.

When the latest semver tag is newer, the footer exposes the configured update
key. Confirming downloads the matching `<os>_<arch>` archive (for example
`lofi-hiphopper_linux_amd64.tar.gz`) and
`checksums.txt`, verifies SHA-256, extracts the binary, writes a temporary file
next to the running executable, and renames it into place. The TUI then exits,
main cleans up mpv/ffmpeg, and `syscall.Exec` replaces the process with the new
binary using the same arguments. Development builds are not release versions,
so they do not self-update.

## Failure behavior

| Failure | Behavior |
| --- | --- |
| yt-dlp fails | Error notice; retry with backoff if auto-fill and silence |
| mpv exits | UI reports it; next tune restarts mpv |
| ffmpeg fails | Playback continues; visualizer shows offline |
| Audio probe fails (CoreAudio, or no `pw-dump`/`pactl`) | Detection notice; manual play still works |
| Update check fails | Silent; the TUI works normally |
| Update download/checksum fails | Dialog shows the error; the old binary stays intact |
| Live stream ends | Retry/backoff while silence; reset on success |
| Terminal too small | Minimal layout; canvas stays in bounds |

## Testing hooks

The code avoids hidden globals so tests can inject fakes:

- `ui.Player` / `ui.Analyzer` / `ui.Resolver` / `ui.Probe`;
- pure `detect.Assess` and `detect.Tracker`;
- injectable analyser binary for missing-ffmpeg tests;
- mpv integration test with `--ao=null`;
- CoreAudio integration test (macOS) and PipeWire probe tests (Linux), plus
  platform-neutral `pw-dump`/`pactl` parser tests;
- fake IPC server over `net.Pipe`.

See [testing](testing.md) for the full feedback loop.
