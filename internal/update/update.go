// Package update checks GitHub releases and replaces the running binary on
// macOS and Linux.
package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultRepo is the public release repository.
	DefaultRepo = "WilsonNet/lofi-hiphopper"
	binaryName  = "lofi-hiphopper"
	maxAsset    = 200 << 20
)

// Asset is one downloadable release file.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

// Release is the latest GitHub release and its assets.
type Release struct {
	Tag     string
	Version string
	URL     string
	Assets  []Asset
}

// Service checks releases and applies them to this executable.
type Service struct {
	CurrentVersion string
	Repo           string
	Client         *http.Client
	APIURL         string
	GOOS           string
	GOARCH         string
	Executable     string
}

// New returns a service for the current executable. Development builds whose
// version is not semver do not check for updates.
func New(currentVersion string) *Service {
	api := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", DefaultRepo)
	if custom := strings.TrimSpace(os.Getenv("LOFI_UPDATE_API")); custom != "" {
		api = custom
	}
	return &Service{
		CurrentVersion: currentVersion,
		Repo:           DefaultRepo,
		Client:         &http.Client{Timeout: 30 * time.Second},
		APIURL:         api,
		GOOS:           runtime.GOOS,
		GOARCH:         runtime.GOARCH,
	}
}

type releasePayload struct {
	TagName string  `json:"tag_name"`
	HTMLURL string  `json:"html_url"`
	Assets  []Asset `json:"assets"`
}

// Check returns the latest release and whether it is newer than the current
// version. A non-release development version disables the check.
func (s *Service) Check(ctx context.Context) (Release, bool, error) {
	if _, ok := ParseVersion(s.CurrentVersion); !ok {
		return Release{}, false, nil
	}
	client := s.httpClient()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.APIURL, nil)
	if err != nil {
		return Release{}, false, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "lofi-hiphopper-updater")
	response, err := client.Do(request)
	if err != nil {
		return Release{}, false, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return Release{}, false, fmt.Errorf("GitHub releases: %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	var payload releasePayload
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&payload); err != nil {
		return Release{}, false, fmt.Errorf("decode release: %w", err)
	}
	tag := strings.TrimSpace(payload.TagName)
	latest, ok := ParseVersion(tag)
	if !ok {
		return Release{}, false, fmt.Errorf("release tag %q is not a version", tag)
	}
	release := Release{
		Tag:     tag,
		Version: strings.TrimPrefix(tag, "v"),
		URL:     payload.HTMLURL,
		Assets:  payload.Assets,
	}
	current, _ := ParseVersion(s.CurrentVersion)
	return release, current.Less(latest), nil
}

// Apply downloads, verifies, extracts, and atomically replaces this binary.
func (s *Service) Apply(ctx context.Context, release Release) error {
	goos := s.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	if goos != "darwin" && goos != "linux" {
		return fmt.Errorf("self-update is not supported on %s", goos)
	}
	arch := s.GOARCH
	if arch == "" {
		arch = runtime.GOARCH
	}
	archiveName := ArchiveName(goos, arch)
	archive, ok := release.Asset(archiveName)
	if !ok {
		return fmt.Errorf("release has no %s asset", archiveName)
	}
	checksums, ok := release.Asset("checksums.txt")
	if !ok {
		return errors.New("release has no checksums.txt asset")
	}

	executable := strings.TrimSpace(s.Executable)
	if executable == "" {
		var err error
		executable, err = os.Executable()
		if err != nil {
			return fmt.Errorf("find executable: %w", err)
		}
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}

	dir, err := os.MkdirTemp("", "lofi-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	checksumPath := filepath.Join(dir, "checksums.txt")
	if err := s.download(ctx, checksums.URL, checksumPath); err != nil {
		return fmt.Errorf("download checksums: %w", err)
	}
	archivePath := filepath.Join(dir, archiveName)
	if err := s.download(ctx, archive.URL, archivePath); err != nil {
		return fmt.Errorf("download update: %w", err)
	}
	expected, err := readChecksum(checksumPath, archiveName)
	if err != nil {
		return err
	}
	actual, err := hashFile(archivePath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(expected, actual) {
		return errors.New("download checksum mismatch")
	}

	extracted := filepath.Join(dir, binaryName)
	if err := extractBinary(archivePath, extracted); err != nil {
		return err
	}

	execDir := filepath.Dir(executable)
	temp, err := os.CreateTemp(execDir, "."+binaryName+".update-*")
	if err != nil {
		return fmt.Errorf("write next binary next to %s: %w", executable, err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)

	source, err := os.Open(extracted)
	if err != nil {
		temp.Close()
		return err
	}
	if _, err := io.Copy(temp, source); err != nil {
		source.Close()
		temp.Close()
		return err
	}
	source.Close()
	if err := temp.Chmod(0o755); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempName, executable); err != nil {
		return fmt.Errorf("replace %s: %w", executable, err)
	}
	return nil
}

// ArchiveName is the release archive for a platform, for example
// lofi-hiphopper_linux_amd64.tar.gz.
func ArchiveName(goos, goarch string) string {
	return fmt.Sprintf("%s_%s_%s.tar.gz", binaryName, goos, goarch)
}

// Asset returns a named release asset.
func (r Release) Asset(name string) (Asset, bool) {
	for _, asset := range r.Assets {
		if asset.Name == name {
			return asset, true
		}
	}
	return Asset{}, false
}

func (s *Service) httpClient() *http.Client {
	if s.Client != nil {
		return s.Client
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (s *Service) download(ctx context.Context, url, destination string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "lofi-hiphopper-updater")
	response, err := s.httpClient().Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, response.Status)
	}
	file, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer file.Close()
	written, err := io.Copy(file, io.LimitReader(response.Body, maxAsset+1))
	if err != nil {
		return err
	}
	if written > maxAsset {
		return errors.New("release asset is too large")
	}
	return file.Sync()
}

func readChecksum(path, name string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if strings.TrimPrefix(fields[1], "*") == name {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("checksum for %s not found", name)
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func extractBinary(archive, destination string) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if filepath.Base(header.Name) != binaryName || header.Typeflag != tar.TypeReg {
			continue
		}
		out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, io.LimitReader(reader, maxAsset+1))
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		return nil
	}
	return fmt.Errorf("%s not found in archive", binaryName)
}

// Version is a simple MAJOR.MINOR.PATCH release version.
type Version struct {
	Major int
	Minor int
	Patch int
}

// ParseVersion accepts an optional v prefix and ignores a prerelease suffix.
func ParseVersion(value string) (Version, bool) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "v")
	if index := strings.IndexAny(value, "-+"); index >= 0 {
		value = value[:index]
	}
	parts := strings.Split(value, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return Version{}, false
	}
	numbers := make([]int, 3)
	for index, part := range parts {
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 {
			return Version{}, false
		}
		numbers[index] = number
	}
	return Version{Major: numbers[0], Minor: numbers[1], Patch: numbers[2]}, true
}

// Less reports whether v is older than other.
func (v Version) Less(other Version) bool {
	if v.Major != other.Major {
		return v.Major < other.Major
	}
	if v.Minor != other.Minor {
		return v.Minor < other.Minor
	}
	return v.Patch < other.Patch
}

// IsReleaseVersion reports whether value can participate in updates.
func IsReleaseVersion(value string) bool {
	_, ok := ParseVersion(value)
	return ok
}
