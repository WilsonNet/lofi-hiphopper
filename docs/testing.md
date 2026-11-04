# Testing

The feedback loop is the most important part of this project. This page explains
what to run and how to prove a change works.

## The ladder

1. **Format and compile** — catch mechanical errors.
2. **Unit tests** — exercise pure logic and component boundaries.
3. **Integration tests** — use real mpv, ffmpeg, CoreAudio, or IPC.
4. **Real interactive test in Herdr** — drive the actual TUI and the actual
   system state.
5. **Documentation and screenshots** — verify what a human will see.

Do not skip from step 1 to "done".

## Fast checks

```bash
gofmt -w .
go vet ./...
go test ./...
go test -race ./...
go build ./...
```

The current suite has 100+ tests and includes table-driven config, shortcut,
hold-mode, detection, tracker, FFT, band mapping, IPC, player, update, layout,
canvas, visualizer, and model tests.

## Real mpv test

```bash
LOFI_MPV_TEST=1 go test ./internal/player -run TestMPVIntegration -v
```

This writes a generated WAV, starts real mpv with `--ao=null`, and asserts
playing, position, volume, mute, pause, and stop. It is silent.

## CoreAudio test

```bash
go test ./internal/detect -run TestCoreAudio -v
```

This enumerates real CoreAudio process objects. It requires macOS 14.4+ and a
cgo build.

## Real TUI test with Herdr

Use the `herdr-testing` skill for the full workflow. The helper creates only
tabs it owns and closes them afterward.

```bash
.agents/skills/herdr-testing/scripts/herdr-tab smoke
```

For this app:

```bash
S=.agents/skills/herdr-testing/scripts/herdr-tab
$S create "lofi-real-test" "$PWD"
# tab_id=... pane_id=...
$S run "$PANE" "/tmp/lofi-hiphopper -config /tmp/lofi-test.yaml"
$S wait "$PANE" "FILLING" 30000
$S read "$PANE" 160
```

Use a temporary config with a low volume:

```yaml
volume: 10
poll_interval: 500ms
autofill: true
radios:
  - name: Lofi Girl · lofi hip hop radio
    url: https://www.youtube.com/watch?v=rFZHOHl-L8A
```

### Required behavior scenarios

**Auto-fill**

1. Ensure no other audio is playing.
2. Start the app.
3. Wait for `FILLING`, `spectrum live`, and moving bars.
4. Confirm mpv and ffmpeg child processes exist.

**Yield and recovery**

1. Open a YouTube video with real/headded audio (or the Playwright browser if
   it produces a real CoreAudio stream).
2. Within a few seconds, the app pane must show `OTHER AUDIO` and `STOPPED`
   with the default `hold_mode: stop`, or `PAUSED`/`MUTED` with the other modes.
3. In stop/pause mode, confirm the analyser stops. In mute mode, confirm mpv and
   ffmpeg are still running but muted.
4. Close the video.
5. Within two probes, the app must show `FILLING` again. Stop mode re-resolves;
   mute mode must not re-resolve.
6. Stop the app and confirm no `lofi-hiphopper` mpv/ffmpeg processes remain.

**Shortcuts and mouse**

1. In the demo TUI, press `?`; verify the shortcut sheet lists every action and
   the current config path.
2. Press `?` or `esc`; verify it closes.
3. Use a real mouse wheel or send an SGR wheel sequence to the pane:
   - plain wheel changes volume;
   - shift+wheel moves the radio cursor;
   - ctrl+wheel changes the visualizer.
4. Press `?` again and confirm mouse wheel is ignored while the modal is open.

**Updates**

1. Build a copy with `-ldflags "-X main.version=0.2.0"` and run it against a
   local release API fixture (`LOFI_UPDATE_API`) that serves a newer archive
   and checksums.
2. Confirm the footer shows the update key, `u` opens the dialog, `n` cancels,
   and `y` downloads, verifies, replaces the binary, and re-execs.
3. Confirm a bad checksum leaves the old binary intact.
4. Unit tests in `internal/update` cover semver comparison, API parsing,
   checksum verification, extraction, and replacement.

**Bypass list**

1. Add `bypass_apps: ["CoreSimulator"]` to a temp config.
2. Start the app while a simulator process is producing output.
3. Confirm detection ignores it and playback starts.
4. Confirm a non-bypassed app still causes a hold.

**Configuration**

1. Start with no config in a temporary HOME; confirm defaults are used and no
   file or directory is created.
2. Run `lofi-hiphopper -print-config`; confirm it prints valid YAML and does not
   write a file.
3. Create a config containing only `volume: 17`; confirm radios, shortcuts, and
   poll interval still use defaults.
4. Add a partial `shortcuts` block changing `mute` to `x`; confirm `m` no longer
   mutes and `x` does, while other keys work.
5. Confirm invalid duplicate bindings fail with a clear error.

## Evidence

Save pane reads under a temporary directory:

```bash
EVIDENCE="$(mktemp -d)"
$S read "$PANE" 200 > "$EVIDENCE/filling.txt"
```

A code reading, a screenshot, or a successful build is not enough. Keep the
pane transcript that shows the transition.

## Screenshots

Use Charm `freeze` on a real captured frame, not a mock:

```bash
tmux capture-pane -e -p -t <session> > /tmp/tui.ansi
freeze -x "cat /tmp/tui.ansi" -o docs/images/example.png \
  -b "#07070e" --margin 12 --padding 12 --font.size 13
```

If freeze fails, use Herdr to focus the test tab and capture the macOS screen
with `screencapture`, then crop to the terminal window. Do not capture unrelated
desktop content.

Run the docs checker after adding or moving images:

```bash
.agents/skills/documentation/scripts/check-docs
```

## Website check

```bash
cd site
npm ci
npm run build
npm run check
```

Then load the built site in a browser at desktop and mobile widths. Verify:
- no console errors or horizontal overflow;
- title, description, canonical, OG image, JSON-LD, sitemap, and robots;
- all six preview modes switch with `h` / `l`;
- the shortcut dialog opens with `?`;
- copy buttons show feedback;
- the layout matches the TUI at 390px and 1200px widths.

## Release check

The release workflow runs the installer against the freshly built local release
before publishing. To reproduce it, see [releasing](releasing.md). A tag is not
done until the published assets install and `--version` returns the tag version.

## Cleanup

- Close only Herdr tabs/panes created by the test.
- Stop test audio and browsers.
- Remove temporary configs and screenshots from `/tmp`.
- Confirm no test-owned `lofi-hiphopper` mpv or ffmpeg process remains.

## CI

`.github/workflows/test.yml` runs formatting, vet, tests, race tests, build, and
the docs link check on macOS for pushes and pull requests.
