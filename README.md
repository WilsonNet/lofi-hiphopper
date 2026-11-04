# lofi hiphopper

A macOS terminal companion that fills silence with lofi, then gets out of the
way when you are on a call or already listening to something.

It is a [Charm](https://charm.sh) Bubble Tea TUI with six animated spectrum
visualizers, mpv playback, and system-wide audio detection through CoreAudio.
The default radio is Lofi Girl's lofi hip hop livestream.

```
  ◉ lofi hiphopper   silence → sound                    AUTO-FILL ON  ● FILLING  14:32
  ━━──────────────────────────────────────────────────────────────────────────────
  ╭─ RADIO ───────────────────── j/k · enter ╮  ╭─ BARS ────────── peak 102 Hz 1/6 h/l ╮
  │ ▸ ● Lofi Girl · lofi hip hop radio       │  │                                      │
  │   ○ Lofi Girl · live now                 │  │          ▃ ▅ █ ▇ ▅ ▃                  │
  │   ○ Lofi Girl · synthwave radio          │  │      ▂ ▅ █ █ █ █ █ ▅ ▂               │
  │   ○ Chillhop · jazzy lofi beats          │  │  ▁ ▄ █ █ █ █ █ █ █ █ ▆ ▄ ▂ ▁          │
  ╰──────────────────────────────────────────╯  │                                      │
  ╭─ SIGNAL ─────────────────────────────────╮  │      63   250   1k   4k   16k        │
  │ audio     silence                        │  ╰──────────────────────────────────────╯
  ╰──────────────────────────────────────────╯
  ╭─ NOW PLAYING ────────────────────── LIVE ╮
  │ Lofi Girl · lofi hip hop radio           │
  │ ● LIVE                                  │
  │ VOL   ████████░░░░ 55%                   │
  ╰──────────────────────────────────────────╯
  ━━──────────────────────────────────────────────────────────────────────────────
  ● SILENCE · filling with lofi                                      AUTO-FILL ON
  m mute · - / = volume · j / k select · enter play · h / l visualizer · a auto-fill
```

## What it does

- Reads radios from `~/.config/lofi-hiphopper/config.yaml`.
- Detects whether any app is producing **output** or capturing **input** (a call).
- If the Mac is silent and no call is active, it plays the selected radio.
- If another app plays audio, it yields and waits. Notification pings are
  debounced so they do not cause stutter.
- If a call starts, it holds playback immediately. Zoom, Meet, FaceTime, Teams
  and any other app that opens the microphone count as a call.
- It ignores its own mpv process, so its music does not count as "other audio".
- When the call or other audio ends, it resumes after two quiet samples. Live
  streams are re-resolved, so you return to the live edge rather than old audio.
- Keeps auto-filling for as long as the TUI is running.

Auto-fill off (`a`) still protects calls. With auto-fill off, other audio does
not pause your chosen radio, and playback does not start automatically.

## Requirements

- macOS 14.4 or newer (the per-process CoreAudio API)
- Go 1.24+
- `mpv`, `yt-dlp`, `ffmpeg`

```sh
brew install mpv yt-dlp ffmpeg
```

## Build and run

```sh
go build -o bin/lofi-hiphopper .
./bin/lofi-hiphopper
```

Or run directly:

```sh
go run .
```

The first launch writes the default config and starts filling silence. Use
`-demo` to see all six visualizers without playing audio or touching system
audio:

```sh
go run . -demo
```

Use a different config file:

```sh
./bin/lofi-hiphopper -config ~/radio.yaml
```

## Configuration

Default path: `~/.config/lofi-hiphopper/config.yaml` (or `$XDG_CONFIG_HOME`,
or `$LOFI_CONFIG`).

```yaml
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
```

Any YouTube video, live stream, channel `/live` URL, or direct audio URL works.
The first radio is the default silence filler. The stream video ID can change
when Lofi Girl restarts a broadcast; the channel live URL is a convenient
fallback.

## Keys

| Key | Action |
| --- | --- |
| `j` / `k` (or arrows) | move the radio cursor |
| `enter` | play the selected radio (queues during calls/other audio) |
| `h` / `l` (or arrows) | previous / next visualizer, no enter needed |
| `1` – `6` | jump straight to a visualizer |
| `m` | mute / unmute |
| `-` / `=` | volume down / up (also unmutes) |
| `a` | toggle auto-fill |
| `r` | restart / retry the selected radio |
| `q` / `ctrl+c` | quit |

## Visualizers

All six use real FFT data from a parallel ffmpeg decode and are smoothed with
fast attack / slow release envelopes with falling peak caps.

1. **BARS** — classic spectrum with glowing peaks and a live frequency ruler.
2. **MIRROR** — centre-out spectrum with a cooler, lagged reflection.
3. **SCOPE** — braille oscilloscope with ghost trails over a dim spectrum.
4. **HALO** — circular polar spectrum with a pulsing core and orbiting spark.
5. **AURORA** — layered translucent spectrum mountains under a twinkling sky.
6. **MATRIX** — dot-matrix LED equalizer with peak dots and a scanline.

The visualizer header shows the current dominant frequency (`peak`). The ruler
shows 63 Hz through 16 kHz on logarithmic positions.

## How detection works

`internal/detect` enumerates CoreAudio process objects
(`kAudioHardwarePropertyProcessObjectList`) and checks each process's
`IsRunningInput` / `IsRunningOutput` flags. It excludes the mpv PID owned by the
app. An active input stream is treated as a call immediately; other output must
persist for two polls before playback yields. This avoids notification pings
knocking the music out for a second.

The player exposes its mpv process as `lofi-hiphopper` in macOS audio controls.

## Development

```sh
go test ./...            # unit and component tests, including CoreAudio
go test -race ./...      # race detector
go vet ./...
go test ./internal/detect -run TestCoreAudio -v
LOFI_MPV_TEST=1 go test ./internal/player -run TestMPVIntegration -v
```

The mpv integration test plays a generated tone into `--ao=null`, so it is
silent. A manual end-to-end check can be done with `go run .` after pausing other
audio; the config's `volume` keeps it quiet if needed.

Layout:

```
main.go
internal/config      YAML defaults and validation
internal/detect      CoreAudio process probe + debounce tracker
internal/media       yt-dlp resolver
internal/player      mpv JSON IPC controller
internal/audio       ffmpeg decoder + FFT + spectrum frames
internal/ui          Bubble Tea model, six visualizers, canvas renderer
```

## Notes

- The analyzer decodes a second, silent audio stream with ffmpeg so the bars
  reflect the actual stream. Playback remains mpv's job.
- Live HLS streams report no duration; the UI shows a live ticker instead.
- If `yt-dlp` breaks after a YouTube change, run `brew upgrade yt-dlp`.
- `ffmpeg` is required for the spectrum. Playback still works if the analyzer
  fails, but the visualizer will show an offline notice.
- This is deliberately macOS-only. The CoreAudio probe has a stub for other
  platforms.
