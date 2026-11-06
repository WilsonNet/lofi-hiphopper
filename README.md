# lofi hiphopper

A tiny terminal companion for macOS and Linux that fills silence with lofi,
then gets out of the way when you are on a call or already listening to
something.

[Website](https://wilsonnet.github.io/lofi-hiphopper/) · [Releases](https://github.com/WilsonNet/lofi-hiphopper/releases)

![lofi hiphopper playing the BARS visualizer](docs/images/hero-bars.png)

*Screenshots use demo mode so they can be captured without playing audio. The
live app uses real system audio and real FFT data.*

It watches your computer's audio activity, starts Lofi Girl when things are quiet,
and holds playback when another app is making sound or using the microphone.
It is a Charm Bubble Tea TUI with six animated spectrum visualizers.

## What it does

- Plays Lofi Girl's lofi hip hop livestream by default.
- Starts automatically when your computer is silent.
- Yields to Spotify, YouTube, a call, or any other app using audio.
- Holds immediately when Zoom, Meet, FaceTime, Teams, Discord, or another app
  opens the microphone.
- Stops the stream while holding and resolves it again on return (default).
  Mute mode is opt-in for a seamless silent hold.
- Comes back after the call or other audio ends.
- Checks GitHub releases on launch. When a newer release exists, `u` appears
  in the footer; `u` opens an update dialog and can install and restart.
- Ignores its own player, so it never pauses itself.
- Lets you bypass apps that should never interrupt lofi, such as simulators or
  background tools.
- Keeps filling silence for as long as the TUI is open.
- Lets you mute, change volume, pick a radio, or change visualizers from the
  keyboard or mouse.

## Install

Runs on **macOS 14.4+** and **Linux** (PipeWire or PulseAudio), on arm64 and
amd64.

Install the runtime dependencies:

```sh
# macOS (Homebrew)
brew install mpv yt-dlp ffmpeg

# Arch Linux (PipeWire ships pw-dump)
sudo pacman -S --needed mpv yt-dlp ffmpeg pipewire

# Debian / Ubuntu
sudo apt install mpv yt-dlp ffmpeg pipewire-bin

# Fedora
sudo dnf install mpv yt-dlp ffmpeg pipewire-utils
```

On Linux, audio detection reads stream metadata with `pw-dump` (PipeWire) and
falls back to `pactl` (PulseAudio). Distro-packaged `yt-dlp` can lag behind
YouTube; `pipx install yt-dlp` keeps it current.

Then install the latest release with curl. The installer detects your OS and
CPU automatically:

```sh
curl -fsSL https://raw.githubusercontent.com/WilsonNet/lofi-hiphopper/main/install.sh | bash
```

The installer puts the binary in `~/.local/bin`. If that directory is not on
your `PATH`, it prints the line to add. Pin a release or choose a location with:

```sh
curl -fsSL https://raw.githubusercontent.com/WilsonNet/lofi-hiphopper/main/install.sh | bash -s -- --version v0.4.2
curl -fsSL https://raw.githubusercontent.com/WilsonNet/lofi-hiphopper/main/install.sh | bash -s -- --prefix /opt/homebrew
```

Prefer building from source? Go 1.24+ is required:

```sh
git clone https://github.com/WilsonNet/lofi-hiphopper.git
cd lofi-hiphopper
go build -o bin/lofi-hiphopper .
./bin/lofi-hiphopper
```

Or run it directly:

```sh
go run .
```

No config file is required. The built-in defaults are embedded in the binary,
so the first launch simply starts filling silence. To customize, create the
file yourself:

```sh
mkdir -p ~/.config/lofi-hiphopper
lofi-hiphopper -print-config > ~/.config/lofi-hiphopper/config.yaml
```

Then edit only the parts you want; everything else stays at its default. Use
`-demo` to try the UI without any audio:

```sh
go run . -demo
```

Releases live at
[github.com/WilsonNet/lofi-hiphopper/releases](https://github.com/WilsonNet/lofi-hiphopper/releases).

## Keys

| Key | Action |
| --- | --- |
| `j` / `k` or `↓` / `↑` | Move the radio cursor |
| `enter` | Play the selected radio (queues during calls/other audio) |
| `h` / `l` or `←` / `→` | Previous / next visualizer |
| `m` | Mute / unmute |
| `-` / `=` | Volume down / up |
| `a` | Toggle auto-fill |
| `r` | Retry / restart the selected radio |
| `?` | Full shortcut sheet |
| `u` | Update to a newer release, when available |
| `q` / `ctrl+c` | Quit |

Mouse gestures work too:

| Gesture | Action |
| --- | --- |
| Wheel | Volume |
| Shift + wheel | Radio |
| Ctrl + wheel | Visualizer |

Every binding, including the mouse gestures, is editable in the optional config
file. Press `?` to see the live table.

![the shortcut sheet](docs/images/shortcuts.png)

## Visualizers

Six visualizers use the real frequency spectrum, not a canned animation. They
are smoothed and include falling peak caps and a dominant-frequency readout.

![all six visualizers](docs/images/visualizers.png)

Switch with `h` / `l` or the mouse. The current names are:

- **BARS** — glowing bars and a live frequency ruler
- **MIRROR** — centre-out spectrum with a cool reflection
- **SCOPE** — braille oscilloscope over a dim spectrum
- **HALO** — circular polar spectrum with a pulsing core
- **AURORA** — layered spectrum clouds under a star field
- **MATRIX** — dot-matrix equalizer with a scanline

*Screenshots are real demo-mode frames rendered with Charm freeze.*

## Configuration

The config is an optional YAML file. If it does not exist, the app uses the
built-in defaults and creates nothing. If it exists, every section is optional:
omitted values use the built-in defaults. A radio list replaces the default
list. Shortcut entries merge by action, so you can change one key without
rewriting the whole table.

A small config can be just this:

```yaml
volume: 25
```

The default yield behavior is:

```yaml
hold_mode: stop
```

To ignore an app completely, list a process name, bundle ID (Flatpak app ID on
Linux), or path substring:

```yaml
bypass_apps:
  - "CoreSimulator"
  - "com.example.MyApp"
```

A radio and shortcut example:

```yaml
radios:
  - name: Lofi Girl · lofi hip hop radio
    url: https://www.youtube.com/watch?v=rFZHOHl-L8A
  - name: Lofi Girl · synthwave radio
    url: https://www.youtube.com/watch?v=4xDzrJKXOOY

shortcuts:
  - action: mute
    keys: ["x"]
  - action: next_radio
    keys: ["j", "shift+wheel_down"]
```

See [the full configuration reference](docs/configuration.md) for every field,
action, mouse token, and the complete default file.

When the window is narrow, the UI still works:

![compact lofi hiphopper layout](docs/images/compact.png)

## How it works, briefly

On macOS it uses CoreAudio process flags to see which apps are producing
output or using a microphone. On Linux it reads PipeWire stream nodes with
`pw-dump` (or PulseAudio streams with `pactl`): a running playback stream is
other audio, a running capture stream is a call. Notification pings are ignored
on both platforms: other audio must last for `yield_delay` (3 seconds by
default) before lofi yields. A separate silent ffmpeg decode feeds the
visualizers, while mpv handles playback. While yielding it stops the stream by
default and resolves it again when silence returns. Set `hold_mode: mute` to
keep streaming silently, or `hold_mode: pause` to pause. It never records your
microphone.

For the deep version, read:

- [Configuration](docs/configuration.md)
- [Architecture](docs/architecture.md)
- [Testing](docs/testing.md)
- [Development](docs/development.md)

## Development

```sh
gofmt -w .
go vet ./...
go test ./...
go test -race ./...
go build ./...
```

Real interactive tests use Herdr tabs. See [Testing](docs/testing.md) and the
portable skills in [`.agents/skills/`](.agents/README.md).

## Notes

- Lofi Girl livestream IDs change. Use a channel `/live` URL or edit the radio
  list when a specific stream retires.
- If YouTube breaks, upgrade yt-dlp (`brew upgrade yt-dlp`,
  `sudo pacman -Syu yt-dlp`, or `pipx upgrade yt-dlp`).
- `ffmpeg` is needed for the real spectrum. Playback still works if the analyser
  fails.
- macOS and Linux are supported. Other platforms build, but audio detection
  reports offline there and manual play still works.
- On Linux, apps that capture desktop audio (visualizers such as cava) are not
  treated as calls. Add anything else that should never interrupt lofi to
  `bypass_apps`.
