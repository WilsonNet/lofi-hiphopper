# Configuration

`lofi-hiphopper` ships with its defaults inside the binary. No config file is
created on install or first launch. If you want to customize something, create
a POSIX-style YAML file yourself; every field is optional and only the parts
you write override the defaults.

## Location

Resolution order:

1. `$LOFI_CONFIG` if set;
2. `$XDG_CONFIG_HOME/lofi-hiphopper/config.yaml` if `XDG_CONFIG_HOME` is set;
3. `~/.config/lofi-hiphopper/config.yaml`.

If the file is missing, the built-in defaults apply and nothing is written. If
the file exists, a field that is omitted keeps its built-in default. There is no
automatic config creation, no backup file, and no rewrite.

To create a starting point manually:

```sh
mkdir -p "${XDG_CONFIG_HOME:-$HOME/.config}/lofi-hiphopper"
lofi-hiphopper -print-config > "${XDG_CONFIG_HOME:-$HOME/.config}/lofi-hiphopper/config.yaml"
```

Then edit the file. `-print-config` prints the commented embedded default and
exits without touching disk.

Use `-config` to point at another file:

```sh
lofi-hiphopper -config ./radio.yaml
```

## Override rules

| Section | If omitted | If present |
| --- | --- | --- |
| `volume` | `55` | Use the value, including `0` |
| `volume_step` | `5` | Use the value |
| `poll_interval` | `500ms` | Use the value |
| `autofill` | `true` | Use the value, including `false` |
| `hold_mode` | `stop` | `stop`, `pause`, or `mute` |
| `bypass_apps` | `[]` | Replace the list of ignored apps |
| `radios` | Built-in Lofi Girl list | Replace the radio list |
| `shortcuts` | Built-in shortcut table | Merge by action; listed actions replace their keys, unlisted actions keep defaults |

A config with only this content is valid:

```yaml
volume: 25
```

Everything else stays at its default.

## Fields

### `volume`

Integer `0`-`100`. The initial mpv volume. `0` is valid.

### `volume_step`

Integer `1`-`25`. One keyboard volume press or one wheel notch.

### `poll_interval`

Duration string. How often the audio probe runs (CoreAudio on macOS,
`pw-dump` or `pactl` on Linux). Range:
`250ms`-`30s`. Calls are detected on the next probe; other audio needs two
consecutive samples.

### `autofill`

Boolean. When true, the first radio starts when the computer is silent and resumes
after calls or other audio stop. When false, silence does not start playback
automatically; calls still hold playback.

### `hold_mode`

`stop` (default), `pause`, or `mute`. This controls what happens while the app
yields to a call or other audio.

- `stop` stops mpv and the analyser, then resolves and plays the stream again
  when silence returns. This is the regular start/stop behavior.
- `pause` pauses mpv and stops the analyser. Playback resumes on silence. A
  paused livestream may resume behind the live edge.
- `mute` keeps the stream and analyser running, mutes mpv, and unmutes on
  silence. This is opt-in for people with fast internet who want a seamless
  silent hold.

```yaml
hold_mode: stop
```

### `bypass_apps`

Apps that must never interrupt lofi. Matching is case-insensitive and checks the
process name, bundle ID, and executable path. A substring is enough.

```yaml
bypass_apps:
  - "CoreSimulator"
  - "com.example.MyApp"
  - "com.apple.Music"
```

On Linux the name is the stream's `application.name` (what `pavucontrol` or
`wpctl status` shows), the bundle ID is the `application.id` or Flatpak app ID,
and the path is the process executable:

```yaml
bypass_apps:
  - "easyeffects"
  - "com.discordapp.Discord"
  - "/usr/bin/cava"
```

This is useful for the iOS Simulator or helper apps that keep an output stream
open even when you cannot hear them. Linux capture streams that only read
desktop audio (a sink monitor, as cava does) are already ignored. A bypassed app is ignored for both output
and microphone activity. Leave it as `[]` to disable.

### `radios`

A list of `name` and `url` entries. The first entry is the default silence
filler. YouTube videos, channel `/live` pages, and direct HTTP(S) audio URLs are
supported.

```yaml
radios:
  - name: Lofi Girl · lofi hip hop radio
    url: https://www.youtube.com/watch?v=rFZHOHl-L8A
  - name: My direct stream
    url: https://example.com/radio.m3u8
```

When `radios` is present it replaces the built-in list. URLs must be unique.

### `shortcuts`

A list of action bindings. This is editable in exactly the same spirit as the
radio list:

```yaml
shortcuts:
  - action: mute
    keys: ["x"]
  - action: next_radio
    keys: ["j", "shift+wheel_down"]
  - action: next_visualizer
    keys: ["l", "ctrl+wheel_down"]
```

Shortcut merging is per action. In the example above, `mute` replaces `m` with
`x`; every action you did not mention keeps its default keys. To see the live
table, press `?` in the TUI.

Actions:

| Action | Default keys | Meaning |
| --- | --- | --- |
| `play` | `enter` | Play the selected radio, or queue it during a call/other audio |
| `next_radio` | `j`, `down`, `shift+wheel_down` | Move the radio cursor down |
| `previous_radio` | `k`, `up`, `shift+wheel_up` | Move the radio cursor up |
| `next_visualizer` | `l`, `right`, `ctrl+wheel_down` | Next visualizer |
| `previous_visualizer` | `h`, `left`, `ctrl+wheel_up` | Previous visualizer |
| `volume_up` | `=`, `+`, `wheel_up` | Volume up |
| `volume_down` | `-`, `_`, `wheel_down` | Volume down |
| `mute` | `m` | Toggle mute |
| `toggle_autofill` | `a` | Toggle auto-fill |
| `retry` | `r` | Restart/retry the selected radio |
| `help` | `?` | Show the shortcut sheet |
| `update` | `u` | Install a newer release when one is available |
| `quit` | `q`, `ctrl+c` | Quit |

Key syntax follows Bubble Tea key names: `enter`, `esc`, `tab`, `space` (write
`" "`), `up`, `down`, `left`, `right`, `ctrl+c`, `alt+x`, `f1`, and literal
characters such as `"?"`, `"+"`, `"-"`, `"="`. Mouse gestures are binding tokens:

| Token | Event |
| --- | --- |
| `wheel_up` / `wheel_down` | Plain vertical wheel |
| `wheel_left` / `wheel_right` | Horizontal wheel |
| `shift+wheel_up` / `shift+wheel_down` | Shift plus wheel |
| `ctrl+wheel_up` / `ctrl+wheel_down` | Control plus wheel |
| `alt+wheel_up` / `alt+wheel_down` | Option plus wheel |

Conflicting bindings are rejected: a key or gesture can belong to only one
action. `ctrl+c` is always an emergency quit even if removed from `quit`.

The app reads the config at startup. Relaunch after editing it.

## Updates

The release build checks the public GitHub releases endpoint on launch. When a
newer version exists, a `u update` hint appears in the footer. Press `u`, then
`y` to install. The updater downloads the archive for your OS and CPU, verifies its
SHA-256 checksum, replaces the executable, and restarts the app. Press `n` or
`esc` to cancel.

Development builds (`version = "dev"`) do not check for updates. Only the
published release binaries self-update.

## Full default

```yaml
volume: 55
volume_step: 5
poll_interval: 500ms
autofill: true
hold_mode: stop
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
```

## Validation

Invalid values fail at startup with a clear message. The app rejects:

- volume outside `0`-`100`;
- volume step outside `1`-`25`;
- poll interval outside `250ms`-`30s`;
- hold mode other than `mute` or `pause`;
- an empty `bypass_apps` entry;
- an empty radio list or duplicate URLs;
- unknown shortcut actions;
- empty keys or duplicate action entries;
- the same key bound to two actions.

Use `.agents/skills/documentation/scripts/check-docs` when editing this page.
