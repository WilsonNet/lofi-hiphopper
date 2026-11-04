# Configuration

`lofi-hiphopper` uses a POSIX-style YAML config file. It is designed to work
with a partial file: every field has a built-in default, and a file only needs
to contain the parts you want to change.

## Location

Resolution order:

1. `$LOFI_CONFIG` if set;
2. `$XDG_CONFIG_HOME/lofi-hiphopper/config.yaml` if `XDG_CONFIG_HOME` is set;
3. `~/.config/lofi-hiphopper/config.yaml`.

On first run the app creates a commented default file. If the file is missing or
a field is omitted, the built-in default applies. The file is written with POSIX
permissions (`0644`) and the directory with `0700`.

Use `-config` to point at another file:

```sh
lofi-hiphopper -config ./radio.yaml
```

## Override rules

| Section | If omitted | If present |
| --- | --- | --- |
| `volume` | `55` | Use the value, including `0` |
| `volume_step` | `5` | Use the value |
| `poll_interval` | `1s` | Use the value |
| `autofill` | `true` | Use the value, including `false` |
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

Duration string. How often the macOS audio process probe runs. Range:
`250ms`-`30s`. Calls are detected on the next probe; other audio needs two
consecutive samples.

### `autofill`

Boolean. When true, the first radio starts when the Mac is silent and resumes
after calls or other audio stop. When false, silence does not start playback
automatically; calls still hold playback.

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

## Full default

```yaml
volume: 55
volume_step: 5
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
  - action: quit
    keys: ["q", "ctrl+c"]
```

## Validation

Invalid values fail at startup with a clear message. The app rejects:

- volume outside `0`-`100`;
- volume step outside `1`-`25`;
- poll interval outside `250ms`-`30s`;
- an empty radio list or duplicate URLs;
- unknown shortcut actions;
- empty keys or duplicate action entries;
- the same key bound to two actions.

Use `.agents/skills/documentation/scripts/check-docs` when editing this page.
