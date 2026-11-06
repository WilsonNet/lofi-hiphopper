# Releasing

Releases are tag-driven. Pushing a tag that starts with `v` publishes a public
GitHub release for macOS and Linux on arm64 and amd64.

## Cut a release

1. Make sure `main` is clean and all checks pass.
2. Confirm the version in `main.go` (the release workflow overrides it from the
   tag via `-ldflags`).
3. Create and push an annotated tag:

```sh
git tag -a v0.4.1 -m "lofi-hiphopper v0.4.1"
git push origin v0.4.1
```

The `.github/workflows/release.yml` workflow then:

- runs `go vet` and `go test`;
- cross-compiles `darwin/arm64` and `darwin/amd64` with cgo enabled
  (CoreAudio), and `linux/amd64` and `linux/arm64` static with
  `CGO_ENABLED=0`;
- creates `lofi-hiphopper_<os>_<arch>.tar.gz` for each target;
- creates `checksums.txt`;
- runs the installer against the local archives on macOS and on Ubuntu;
- publishes the archives, `install.sh`, and checksums;
- generates release notes from commits.

## Self-update contract

Release binaries check the GitHub latest release at launch. The updater expects:

- `lofi-hiphopper_darwin_arm64.tar.gz`
- `lofi-hiphopper_darwin_amd64.tar.gz`
- `lofi-hiphopper_linux_amd64.tar.gz`
- `lofi-hiphopper_linux_arm64.tar.gz`
- `checksums.txt`
- a `vMAJOR.MINOR.PATCH` tag
- `main.version` injected by the release workflow

The archive must contain `lofi-hiphopper` at its root. Do not rename assets
without updating `internal/update` and `install.sh` together. Development builds
(`version = "dev"`) do not check for updates.

## Installer

`install.sh` detects the OS (`uname -s`) and CPU (`uname -m`, accepting
`arm64`/`aarch64` and `x86_64`/`amd64`), resolves the latest tag, downloads the
matching archive, verifies the SHA-256 checksum with `sha256sum` or `shasum`,
extracts the binary, and installs it to `~/.local/bin` by default. It rejects
other operating systems and macOS versions below 14.4. Missing runtime tools are
reported with the right command for Homebrew, pacman, apt, dnf, zypper, apk, or
xbps, and on Linux it warns when neither `pw-dump` nor `pactl` is installed.

Users can install a release with:

```sh
curl -fsSL https://raw.githubusercontent.com/WilsonNet/lofi-hiphopper/main/install.sh | bash
```

Or pin a version:

```sh
curl -fsSL .../install.sh | bash -s -- --version v0.4.1
```

## Test the release locally

Build the same archives and exercise the installer without GitHub:

```sh
rm -rf /tmp/lofi-release
mkdir -p /tmp/lofi-release/dist
OS=$(go env GOOS)   # darwin needs CGO_ENABLED=1; linux works with 0
for ARCH in arm64 amd64; do
  mkdir -p /tmp/lofi-release/package
  CGO_ENABLED=$([ "$OS" = darwin ] && echo 1 || echo 0) GOOS=$OS GOARCH=$ARCH go build \
    -trimpath -ldflags "-s -w -X main.version=0.2.0" \
    -o /tmp/lofi-release/package/lofi-hiphopper .
  cp README.md install.sh /tmp/lofi-release/package/
  (cd /tmp/lofi-release/package && \
    tar -czf "../dist/lofi-hiphopper_${OS}_${ARCH}.tar.gz" \
      lofi-hiphopper README.md install.sh)
done
cp install.sh /tmp/lofi-release/dist/install.sh
(cd /tmp/lofi-release/dist && \
  sha256sum *.tar.gz install.sh > checksums.txt)   # or shasum -a 256

VERSION=v0.4.1 BASE_URL=file:///tmp/lofi-release/dist \
  INSTALL_DIR=/tmp/lofi-install ./install.sh
/tmp/lofi-install/lofi-hiphopper --version
```

The workflow repeats this installer check before publishing.

## Notes

- Do not reuse or move a published tag. Cut a new patch version instead.
- macOS archives use cgo for CoreAudio; Linux archives are static and need
  `pw-dump` or `pactl` at runtime for detection.
- The release asset names are part of the installer contract. Update both the
  workflow and `install.sh` together if they change.
- Verify the release from a clean shell after publishing:

```sh
curl -fsSL https://raw.githubusercontent.com/WilsonNet/lofi-hiphopper/main/install.sh | bash -s -- --install-dir /tmp/lofi-install
/tmp/lofi-install/lofi-hiphopper --version
```
