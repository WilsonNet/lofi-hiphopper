# Project instructions

## Prime directive

The feedback loop is the most important part of this project. Every change must
be tested before it is considered done. Do not stop at reading code or getting a
successful compile. Run the fast checks, then exercise the real behavior. If a
test cannot be run, say exactly why and leave a reproducible command.

## What this project is

`lofi-hiphopper` is a macOS terminal companion that fills silence with lofi. It
plays YouTube or direct audio URLs through mpv, detects system audio activity
through CoreAudio, and yields during calls or other playback. The UI is Bubble
Tea with a custom truecolor canvas and six FFT-driven visualizers.

Read `docs/architecture.md` before changing playback, detection, or the state
machine. Read `docs/configuration.md` before changing config or shortcuts.

## Fast checks

Run these from the repository root after every substantive change:

```bash
gofmt -w .
go vet ./...
go test ./...
go test -race ./...
go build ./...
```

When touching the player integration:

```bash
LOFI_MPV_TEST=1 go test ./internal/player -run TestMPVIntegration -v
```

When touching CoreAudio detection:

```bash
go test ./internal/detect -run TestCoreAudio -v
```

The mpv integration test plays a generated tone into `--ao=null`; it is silent.

## Real tests with Herdr

Use the `herdr-testing` skill for interactive verification. Herdr is the
terminal workspace for this repository; do not use tmux for agent-driven real
tests.

```bash
.agents/skills/herdr-testing/scripts/herdr-tab smoke
```

Create an isolated tab, run the real app there, drive it with keys, and read
the pane. Never close a tab the user created. Never use loud audio for tests;
use a low volume or a silent test config.

For behavior that depends on system audio, verify both directions:

1. Silence: the app starts or resumes the selected radio.
2. Other audio: the app yields and stops live playback.
3. Silence again: the app resumes.
4. A call or microphone stream: playback is held immediately.

If a test needs a browser, use a real/headded browser. Verify the app's pane
output; a screenshot alone is not evidence of state.

## Configuration contract

- Defaults live in code and are embedded in the binary as commented YAML.
- The app never creates, writes, or rewrites a config file. A missing file is
  valid and means "use all defaults".
- `-print-config` prints the embedded YAML for a user who wants to create a file.
- A user-created config only overrides the parts it specifies.
- Scalars such as `volume`, `volume_step`, `poll_interval`, `autofill`, and
  `hold_mode` override their default when present.
- `bypass_apps` is an optional list of process name, bundle ID, or executable
  path substrings that must never interrupt lofi.
- `radios` replaces the default radio list when present.
- `shortcuts` merges by action: listed actions replace their keys; unlisted
  actions keep their built-in keys.
- Shortcut entries use `action` and `keys`. Mouse gestures are bindings too:
  `wheel_up`, `wheel_down`, `shift+wheel_down`, `ctrl+wheel_up`, and so on.
- Validate new config behavior with tests for omitted, partial, full, and
  invalid configuration.

## Architecture invariants

- Own-player exclusion: the UI passes mpv's PID to the detector so the app's
  own output never counts as "other audio".
- Calls win: any non-own input stream is a call and holds playback immediately.
- Other output yields after debounce; notification pings must not pause music.
- Default hold mode is stop: stop mpv and the analyser, then re-resolve on
  silence. `hold_mode: pause` and `hold_mode: mute` are opt-in alternatives.
- Preserve the user's mute intent across a mute hold.
- Bypassed apps are excluded from both output and microphone classification.
- The analyzer is a separate silent ffmpeg decode. Playback must continue if the
  analyzer or visualizer fails.
- UI drawing must stay within the canvas; all six visualizers must render at
  wide, compact, and minimal sizes without panics.
- The player and analyzer processes must be cleaned up on quit and on error.
- Update checks must not block the TUI or fail on offline/rate-limited networks.
  Updates are checksum-verified, atomically replace the binary, and restart via
  `syscall.Exec` after cleanup. Development versions do not self-update.

## Skills

- `.agents/skills/herdr-testing/SKILL.md` — real interactive tests in isolated
  Herdr tabs, with pane evidence and cleanup.
- `.agents/skills/documentation/SKILL.md` — human-friendly README, deep docs in
  `docs/`, and real TUI screenshots rendered with Charm freeze.

The `.agents/skills/` layout is intentionally tool-agnostic. Keep reusable
workflows there; keep project-specific commands in this file.

## Website

The marketing site is an Astro app in `site/`. It deploys to GitHub Pages.

```bash
cd site
npm ci
npm run build
npm run check
```

Keep the terminal aesthetic aligned with the TUI. Test the built site in a
browser at desktop and mobile widths. Check console errors, links, keyboard
visualizer switching, the shortcut dialog, and copy buttons. Update the site
version and release links when cutting a release.

## Documentation

- `README.md` is for humans: short, visual, quick start, keys, and links.
- `docs/` is for depth: configuration reference, architecture, testing, and
  development.
- Screenshots are captured from the real TUI with `freeze` and stored in
  `docs/images/`.
- Run `.agents/skills/documentation/scripts/check-docs` after documentation
  changes.

## Definition of done

A change is done only when:

- `gofmt`, `go vet`, tests, race tests, and build pass;
- new behavior has tests at the right level;
- real behavior was exercised in a Herdr tab when it touches the UI, audio,
  detection, shortcuts, or configuration;
- docs and the screenshot set are updated when user-visible behavior changes;
- no own processes, test tabs, screenshots, or temporary files are left behind.

## Safety

- Do not touch the user's audio processes, tabs, or browser windows.
- Do not run loud audio. Use a test config with low volume.
- Do not commit generated binaries, build output, or local config.
- Close only resources you created; use temporary directories for evidence.
