package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input string
		ok    bool
		want  Version
	}{
		{"0.3.0", true, Version{0, 3, 0}},
		{"v1.2.3", true, Version{1, 2, 3}},
		{"v1.2", true, Version{1, 2, 0}},
		{"v1.2.3-beta.1", true, Version{1, 2, 3}},
		{"dev", false, Version{}},
		{"nightly", false, Version{}},
		{"1", false, Version{}},
		{"1.2.3.4", false, Version{}},
	}
	for _, tt := range tests {
		got, ok := ParseVersion(tt.input)
		if ok != tt.ok || (ok && got != tt.want) {
			t.Errorf("ParseVersion(%q) = %+v, %v; want %+v, %v", tt.input, got, ok, tt.want, tt.ok)
		}
	}
	if !(Version{0, 3, 0}).Less(Version{0, 4, 0}) {
		t.Fatal("0.3.0 should be older than 0.4.0")
	}
	if (Version{1, 0, 0}).Less(Version{0, 9, 9}) {
		t.Fatal("1.0.0 should not be older than 0.9.9")
	}
	if !IsReleaseVersion("0.3.0") || IsReleaseVersion("dev") {
		t.Fatal("release version detection is wrong")
	}
}

func TestCheckReportsNewerRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/releases/latest" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v0.4.0",
			"html_url": "https://example.test/releases/v0.4.0",
			"assets": []map[string]any{
				{"name": "lofi-hiphopper_darwin_arm64.tar.gz", "browser_download_url": "https://example.test/a.tar.gz", "size": 12},
				{"name": "checksums.txt", "browser_download_url": "https://example.test/checksums.txt", "size": 42},
			},
		})
	}))
	defer server.Close()
	service := &Service{
		CurrentVersion: "0.3.0",
		APIURL:         server.URL + "/releases/latest",
		Client:         server.Client(),
		GOOS:           "darwin",
		GOARCH:         "arm64",
	}
	release, available, err := service.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !available || release.Version != "0.4.0" {
		t.Fatalf("release = %+v available=%v", release, available)
	}
	if _, ok := release.Asset("checksums.txt"); !ok {
		t.Fatal("checksums asset missing")
	}
}

func TestCheckNoUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": "v0.3.0"})
	}))
	defer server.Close()
	service := &Service{CurrentVersion: "0.3.0", APIURL: server.URL, Client: server.Client()}
	_, available, err := service.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if available {
		t.Fatal("equal versions should not update")
	}
}

func TestCheckDevelopmentVersionSkipsNetwork(t *testing.T) {
	service := &Service{CurrentVersion: "dev", APIURL: "http://127.0.0.1:1/never", Client: &http.Client{Timeout: time.Millisecond}}
	_, available, err := service.Check(context.Background())
	if err != nil || available {
		t.Fatalf("dev check = %v, %v", available, err)
	}
}

func TestApplyReplacesExecutable(t *testing.T) {
	for _, platform := range [][2]string{{"darwin", "arm64"}, {"linux", "amd64"}, {"linux", "arm64"}} {
		t.Run(platform[0]+"_"+platform[1], func(t *testing.T) {
			testApplyReplacesExecutable(t, platform[0], platform[1])
		})
	}
}

func testApplyReplacesExecutable(t *testing.T, goos, goarch string) {
	name := ArchiveName(goos, goarch)
	payload := []byte("new-binary")
	archive := makeArchive(t, "lofi-hiphopper", payload)
	checksums := makeChecksum(name, archive)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/checksums.txt":
			_, _ = io.WriteString(w, checksums)
		case "/archive.tar.gz":
			_, _ = w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "lofi-hiphopper")
	if err := os.WriteFile(path, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	release := Release{
		Tag:     "v0.4.0",
		Version: "0.4.0",
		Assets: []Asset{
			{Name: "checksums.txt", URL: server.URL + "/checksums.txt"},
			{Name: "lofi-hiphopper_darwin_x86.tar.gz", URL: server.URL + "/wrong.tar.gz"},
			{Name: name, URL: server.URL + "/archive.tar.gz"},
		},
	}
	service := &Service{
		CurrentVersion: "0.3.0",
		Client:         server.Client(),
		GOOS:           goos,
		GOARCH:         goarch,
		Executable:     path,
	}
	if err := service.Apply(context.Background(), release); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("binary = %q, want %q", got, payload)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatal("replacement is not executable")
	}
}

func TestApplyChecksumMismatchLeavesOldBinary(t *testing.T) {
	archive := makeArchive(t, "lofi-hiphopper", []byte("new"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/checksums.txt":
			_, _ = io.WriteString(w, "0000000000000000000000000000000000000000000000000000000000000000  lofi-hiphopper_darwin_arm64.tar.gz\n")
		case "/archive.tar.gz":
			_, _ = w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "lofi-hiphopper")
	if err := os.WriteFile(path, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	release := Release{Assets: []Asset{
		{Name: "checksums.txt", URL: server.URL + "/checksums.txt"},
		{Name: "lofi-hiphopper_darwin_arm64.tar.gz", URL: server.URL + "/archive.tar.gz"},
	}}
	service := &Service{GOOS: "darwin", GOARCH: "arm64", Executable: path, Client: server.Client()}
	if err := service.Apply(context.Background(), release); err == nil {
		t.Fatal("expected checksum mismatch")
	}
	got, _ := os.ReadFile(path)
	if string(got) != "old" {
		t.Fatalf("old binary was modified: %q", got)
	}
}

func TestApplyUnsupportedPlatform(t *testing.T) {
	service := &Service{GOOS: "windows", GOARCH: "amd64", Executable: "/tmp/x"}
	err := service.Apply(context.Background(), Release{})
	if err == nil || !strings.Contains(err.Error(), "windows") {
		t.Fatalf("unsupported platform error = %v", err)
	}
}

func TestArchiveName(t *testing.T) {
	if got := ArchiveName("linux", "amd64"); got != "lofi-hiphopper_linux_amd64.tar.gz" {
		t.Fatalf("ArchiveName = %q", got)
	}
}

func TestApplyMissingAsset(t *testing.T) {
	service := &Service{GOOS: "darwin", GOARCH: "arm64", Executable: "/tmp/x"}
	err := service.Apply(context.Background(), Release{})
	if err == nil {
		t.Fatal("expected missing asset error")
	}
}

func makeArchive(t *testing.T, name string, payload []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0o755,
		Size: int64(len(payload)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func makeChecksum(name string, payload []byte) string {
	sum := sha256.Sum256(payload)
	return fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), name)
}
