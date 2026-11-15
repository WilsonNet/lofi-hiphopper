# Changelog

All notable changes to lofi-hiphopper are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.5.1] - 2026-11-15

### Added

- `bypass_apps` now matches parent processes on Linux. Anki plays audio
  through a child `mpv`, so a single `anki` entry keeps flashcard audio from
  pausing lofi. Ancestor names and executable paths are checked for up to six
  levels.

### Changed

- Documented parent-process matching in the configuration reference and README.
- Updated the website for v0.5.1.

## [0.5.0] - 2026-11-08

### Added

- `visualizer` config option to choose the startup visualizer: `bars`,
  `mirror`, `scope`, `halo`, `aurora`, or `matrix`.

## [0.4.2] - 2026-11-06

### Fixed

- macOS: ignore `corespeechd` microphone use so playback through built-in
  speakers is not mistaken for a call. Thanks to @wagnerjs for the first
  contribution ([#1](https://github.com/WilsonNet/lofi-hiphopper/pull/1)).

## [0.4.1] - 2026-11-06

### Added

- `yield_delay` setting (default `3s`). Other audio must persist for this long
  before lofi yields, so notification pings no longer stop the music. Calls
  still hold playback immediately.

### Changed

- Release CI moved to macOS 15 runners.

## [0.4.0] - 2026-11-05

### Added

- Linux support with PipeWire (`pw-dump`) and PulseAudio (`pactl`). Detection
  reads stream metadata only and never records audio. Linux builds are part of
  every release and supported by the installer.

## [0.3.3] - 2026-11-05

### Added

- `bypass_apps` config list for apps that must never interrupt lofi, such as
  simulators and background tools.
- Marketing website with a terminal-workspace design, deployed to GitHub
  Pages.

### Changed

- Default hold mode is now `stop`, and the audio probe runs more frequently.

## [0.3.0] - 2026-11-04

### Added

- `hold_mode: mute` for a seamless silent hold. `stop` and `pause` remain
  available.
- Built-in configuration embedded in the binary. A user config is optional and
  overrides only the fields it contains.
- Self-update: `u` checks GitHub releases, verifies checksums, replaces the
  binary, and restarts into the new version.

### Changed

- The app never creates or rewrites a config file. `-print-config` prints the
  embedded defaults.

## [0.2.0] - 2026-11-04

### Added

- Initial public release: a terminal companion that fills silence with Lofi
  Girl through mpv and yields to calls and other system audio.
- Six FFT-driven visualizers with adaptive wide, compact, and minimal layouts.
- Editable keyboard shortcuts and mouse gestures.
- macOS CoreAudio detection with own-player exclusion.
- Tag-driven releases and a curl installer.

[Unreleased]: https://github.com/WilsonNet/lofi-hiphopper/compare/v0.5.1...HEAD
[0.5.1]: https://github.com/WilsonNet/lofi-hiphopper/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/WilsonNet/lofi-hiphopper/compare/v0.4.2...v0.5.0
[0.4.2]: https://github.com/WilsonNet/lofi-hiphopper/compare/v0.4.1...v0.4.2
[0.4.1]: https://github.com/WilsonNet/lofi-hiphopper/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/WilsonNet/lofi-hiphopper/compare/v0.3.3...v0.4.0
[0.3.3]: https://github.com/WilsonNet/lofi-hiphopper/compare/v0.3.0...v0.3.3
[0.3.0]: https://github.com/WilsonNet/lofi-hiphopper/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/WilsonNet/lofi-hiphopper/releases/tag/v0.2.0
