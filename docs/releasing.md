# Releasing

Releases are tag-driven. Pushing a tag that starts with `v` publishes a public
GitHub release with both macOS architectures.

## Cut a release

1. Make sure `main` is clean and all checks pass.
2. Confirm the version in `main.go` (the release workflow overrides it from the
   tag via `-ldflags`).
3. Create and push an annotated tag:

```sh
git tag -a v0.3.3 -m "lofi-hiphopper v0.3.3"
git push origin v0.3.3
```

The `.github/workflows/release.yml` workflow then:

- runs `go vet` and `go test`;
- cross-compiles `darwin/arm64` and `darwin/amd64` with cgo enabled;
- creates `lofi-hiphopper_darwin_arm64.tar.gz` and
  `lofi-hiphopper_darwin_amd64.tar.gz`;
- creates `checksums.txt`;
- publishes the archives, `install.sh`, and checksums;
- generates release notes from commits.

## Self-update contract

Release binaries check the GitHub latest release at launch. The updater expects:

- `lofi-hiphopper_darwin_arm64.tar.gz`
- `lofi-hiphopper_darwin_amd64.tar.gz`
- `checksums.txt`
- a `vMAJOR.MINOR.PATCH` tag
- `main.version` injected by the release workflow

The archive must contain `lofi-hiphopper` at its root. Do not rename assets
without updating `internal/update` and `install.sh` together. Development builds
(`version = "dev"`) do not check for updates.

## Installer

`install.sh` resolves the latest tag, downloads the matching archive, verifies
the SHA-256 checksum, extracts the binary, and installs it to `~/.local/bin` by
default. It rejects non-macOS systems and macOS versions below 14.4.

Users can install a release with:

```sh
curl -fsSL https://raw.githubusercontent.com/WilsonNet/lofi-hiphopper/main/install.sh | bash
```

Or pin a version:

```sh
curl -fsSL .../install.sh | bash -s -- --version v0.3.3
```

## Test the release locally

Build the same archives and exercise the installer without GitHub:

```sh
rm -rf /tmp/lofi-release
mkdir -p /tmp/lofi-release/dist
for ARCH in arm64 amd64; do
  mkdir -p /tmp/lofi-release/package
  CGO_ENABLED=1 GOOS=darwin GOARCH=$ARCH go build \
    -trimpath -ldflags "-s -w -X main.version=0.2.0" \
    -o /tmp/lofi-release/package/lofi-hiphopper .
  cp README.md install.sh /tmp/lofi-release/package/
  (cd /tmp/lofi-release/package && \
    tar -czf "../dist/lofi-hiphopper_darwin_${ARCH}.tar.gz" \
      lofi-hiphopper README.md install.sh)
done
cp install.sh /tmp/lofi-release/dist/install.sh
(cd /tmp/lofi-release/dist && \
  shasum -a 256 *.tar.gz install.sh > checksums.txt)

VERSION=v0.3.3 BASE_URL=file:///tmp/lofi-release/dist \
  INSTALL_DIR=/tmp/lofi-install ./install.sh
/tmp/lofi-install/lofi-hiphopper --version
```

The workflow repeats this installer check before publishing.

## Notes

- Do not reuse or move a published tag. Cut a new patch version instead.
- The binary is macOS-only. Linux support is a future change.
- The release asset names are part of the installer contract. Update both the
  workflow and `install.sh` together if they change.
- Verify the release from a clean shell after publishing:

```sh
curl -fsSL https://raw.githubusercontent.com/WilsonNet/lofi-hiphopper/main/install.sh | bash -s -- --install-dir /tmp/lofi-install
/tmp/lofi-install/lofi-hiphopper --version
```
