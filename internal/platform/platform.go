// Package platform holds small OS-specific hints shared by the app.
package platform

import (
	"os"
	"runtime"
	"strings"
)

// InstallHint returns a command that installs the given tools on this system,
// for example "brew install mpv" or "sudo pacman -S mpv".
func InstallHint(tools ...string) string {
	return installHint(runtime.GOOS, readOSRelease(), tools...)
}

func installHint(goos, osRelease string, tools ...string) string {
	list := strings.Join(tools, " ")
	if goos == "darwin" {
		return "brew install " + list
	}
	if goos != "linux" {
		return "install " + list + " with your package manager"
	}
	ids := osReleaseIDs(osRelease)
	for _, id := range ids {
		switch id {
		case "arch", "manjaro", "endeavouros", "cachyos":
			return "sudo pacman -S --needed " + list
		case "debian", "ubuntu", "linuxmint", "pop":
			return "sudo apt install " + list
		case "fedora", "rhel", "centos":
			return "sudo dnf install " + list
		case "opensuse", "opensuse-tumbleweed", "opensuse-leap", "suse":
			return "sudo zypper install " + list
		case "alpine":
			return "sudo apk add " + list
		case "void":
			return "sudo xbps-install " + list
		case "nixos":
			return "nix-shell -p " + list
		}
	}
	return "install " + list + " with your package manager"
}

func readOSRelease() string {
	for _, path := range []string{"/etc/os-release", "/usr/lib/os-release"} {
		if data, err := os.ReadFile(path); err == nil {
			return string(data)
		}
	}
	return ""
}

// osReleaseIDs returns ID followed by ID_LIKE entries from os-release text.
func osReleaseIDs(text string) []string {
	var id, like []string
	for _, line := range strings.Split(text, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		value = strings.ToLower(strings.Trim(value, `"'`))
		switch key {
		case "ID":
			id = append(id, value)
		case "ID_LIKE":
			like = append(like, strings.Fields(value)...)
		}
	}
	return append(id, like...)
}
