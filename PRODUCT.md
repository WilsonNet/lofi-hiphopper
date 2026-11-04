# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Terminal-first developers on macOS 14.4 or newer who work at a desk with a
terminal open for long stretches. They want background lofi without managing a
player, and they do not want music to talk over calls, screen sharing, videos,
or anything else already making sound.

The marketing website is for the same audience: developers evaluating a small
open-source tool from a README, a GitHub release, or a search result.

## Product Purpose

`lofi-hiphopper` fills silence with lofi and gets out of the way when audio
matters. It watches macOS audio activity, starts the selected radio when the
machine is quiet, and holds playback during calls or other output.

Success means the user forgets it is running: music appears when the desk is
quiet, disappears during a call, and returns afterward without babysitting.

## Positioning

A silence-aware terminal companion, not a playlist or a timer. It reads
macOS process audio activity, excludes its own player, treats any non-own
microphone input as a call, and yields before the user has to reach for pause.
The default hold is a real stop and re-resolve on silence, not a paused stream
that may drift or resume stale.

## Operating Context

- Runs inside a macOS terminal as a Bubble Tea TUI.
- Requires `mpv`, `yt-dlp`, and `ffmpeg`, typically installed with Homebrew.
- Plays YouTube livestreams or direct audio URLs through mpv.
- Uses an optional POSIX-style YAML config at
  `~/.config/lofi-hiphopper/config.yaml` (or `$LOFI_CONFIG` /
  `$XDG_CONFIG_HOME`).
- Checks GitHub Releases on launch and can self-update, verify a checksum,
  replace the binary atomically, and restart.
- The website is a static Astro site deployed to GitHub Pages at
  https://wilsonnet.github.io/lofi-hiphopper/.
- Users commonly arrive from the public repository, a release page, or search.

## Capabilities and Constraints

- macOS only for now. Linux support is planned but not shipped.
- Default radio is Lofi Girl's lofi hip hop livestream; the radio list is
  configurable.
- Detection is based on CoreAudio process input/output flags, not audio
  fingerprinting.
- Own-player exclusion is an invariant: the app's mpv process must never count
  as "other audio".
- Calls win immediately. Any non-own input stream is treated as a call.
- Other output yields after debounce, so short notification pings do not pause
  music.
- Hold modes: `stop` (default), `pause`, and `mute`. Mute must preserve the
  user's mute intent across the hold.
- Bypassed apps are excluded from both output and microphone classification.
  Matching is case-insensitive against process name, bundle ID, or executable
  path.
- Six FFT-driven visualizers: bars, mirror, scope, halo, aurora, matrix.
- Keyboard and mouse bindings are editable through config; `?` opens the live
  shortcut sheet.
- No config file is created, written, or rewritten by the app. Missing config
  is valid and means all defaults.
- The analyzer is a separate silent ffmpeg decode. Playback must continue if
  the analyzer or visualizer fails.
- Update checks must not block the TUI or fail on offline or rate-limited
  networks. Development builds do not self-update.
- The website is built with Astro 7, uses a live canvas demo, and deploys
  through GitHub Actions.
- System-audio visualization of other apps is not implemented. Visualizers
  currently show lofi playback FFT data only.

## Brand Commitments

- The name is `lofi-hiphopper`.
- The website's visual language must stay aligned with the app's terminal/TUI
  aesthetic.
- Voice is plain, concise, and quietly playful. It explains behavior rather
  than overselling it.

## Evidence on Hand

- `docs/images/hero-bars.png`, `docs/images/visualizers.png`, and
  `docs/images/shortcuts.png` are real TUI captures made with Charm `freeze`.
- The public repository is https://github.com/WilsonNet/lofi-hiphopper.
- Public releases include `v0.3.2` at
  https://github.com/WilsonNet/lofi-hiphopper/releases/tag/v0.3.2.
- The live site is https://wilsonnet.github.io/lofi-hiphopper/.
- `README.md`, `docs/`, and `install.sh` are the current product documentation
  and install path.

There are no testimonials, customer logos, user counts, benchmarks, press
mentions, pricing, or third-party case studies. Future work must not invent
them.

## Product Principles

1. Silence is the trigger; real audio is the veto.
2. Never fight the user's own output, mute intent, or bypass list.
3. One command to start. No required config, account, or setup wizard.
4. Terminal-first and legible at a glance, including at compact sizes.
5. Fail soft: analyzer, visualizer, network, and update failures must not stop
   playback.

## Accessibility & Inclusion

The website must keep keyboard access to the interactive demo and respect
`prefers-reduced-motion`. No formal accessibility standard or WCAG level has
been confirmed.
