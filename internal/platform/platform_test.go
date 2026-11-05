package platform

import "testing"

func TestInstallHint(t *testing.T) {
	cases := []struct {
		name, goos, release, want string
	}{
		{"mac", "darwin", "", "brew install mpv ffmpeg"},
		{"arch", "linux", "NAME=\"Arch Linux\"\nID=arch\n", "sudo pacman -S --needed mpv ffmpeg"},
		{"ubuntu", "linux", "ID=ubuntu\nID_LIKE=debian\n", "sudo apt install mpv ffmpeg"},
		{"derivative", "linux", "ID=zorin\nID_LIKE=\"ubuntu debian\"\n", "sudo apt install mpv ffmpeg"},
		{"fedora", "linux", "ID=fedora\n", "sudo dnf install mpv ffmpeg"},
		{"unknown linux", "linux", "ID=gentoo\n", "install mpv ffmpeg with your package manager"},
		{"no os-release", "linux", "", "install mpv ffmpeg with your package manager"},
		{"other", "freebsd", "", "install mpv ffmpeg with your package manager"},
	}
	for _, tc := range cases {
		if got := installHint(tc.goos, tc.release, "mpv", "ffmpeg"); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
	if InstallHint("mpv") == "" {
		t.Fatal("empty hint")
	}
}
