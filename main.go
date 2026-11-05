package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/wilsonpsn/lofi-hiphopper/internal/audio"
	"github.com/wilsonpsn/lofi-hiphopper/internal/config"
	"github.com/wilsonpsn/lofi-hiphopper/internal/detect"
	"github.com/wilsonpsn/lofi-hiphopper/internal/media"
	"github.com/wilsonpsn/lofi-hiphopper/internal/platform"
	"github.com/wilsonpsn/lofi-hiphopper/internal/player"
	"github.com/wilsonpsn/lofi-hiphopper/internal/ui"
	"github.com/wilsonpsn/lofi-hiphopper/internal/update"
)

var version = "dev"

func main() {
	var (
		configPath  = flag.String("config", "", "config file (default ~/.config/lofi-hiphopper/config.yaml)")
		demo        = flag.Bool("demo", false, "run the visualizer with simulated audio")
		showVer     = flag.Bool("version", false, "print version and exit")
		printConfig = flag.Bool("print-config", false, "print the built-in YAML config and exit")
	)
	flag.Parse()

	if *showVer {
		fmt.Println("lofi-hiphopper " + version)
		return
	}
	if *printConfig {
		fmt.Print(config.DefaultYAML())
		return
	}

	path := *configPath
	if strings.TrimSpace(path) == "" {
		path = config.Path()
	}
	cfg, err := config.Load(path)
	if err != nil {
		fatal(err)
	}

	if !*demo {
		if err := checkTools("mpv", "yt-dlp", "ffmpeg"); err != nil {
			fatal(err)
		}
	}

	var (
		playerService ui.Player
		analyzer      ui.Analyzer
		resolver      ui.Resolver
		probe         ui.Probe
		updater       ui.Updater
		cleanup       = func() {}
	)
	if !*demo {
		mpv := player.New(player.Options{
			Binary:    "mpv",
			Volume:    cfg.Volume,
			UserAgent: media.DefaultUserAgent,
		})
		fft := audio.NewAnalyzer()
		resolver = media.NewResolver()
		probe = detect.New()
		playerService = mpv
		if update.IsReleaseVersion(version) {
			updater = update.New(version)
		}
		analyzer = fft
		cleanup = func() {
			fft.Close()
			_ = mpv.Close()
		}
	}
	defer cleanup()

	model := ui.New(cfg, ui.Deps{
		Player:   playerService,
		Analyzer: analyzer,
		Resolver: resolver,
		Probe:    probe,
		Updater:  updater,
	}, ui.Options{
		Demo:       *demo,
		ConfigPath: path,
	})

	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithFPS(60), tea.WithMouseCellMotion())
	finalModel, err := program.Run()
	if err != nil {
		cleanup()
		fatal(err)
	}
	if restarter, ok := finalModel.(interface{ RestartRequested() bool }); ok && restarter.RestartRequested() {
		cleanup()
		executable, execErr := os.Executable()
		if execErr != nil {
			fatal(execErr)
		}
		if execErr := syscall.Exec(executable, os.Args, os.Environ()); execErr != nil {
			fatal(fmt.Errorf("restart after update: %w", execErr))
		}
	}
}

func checkTools(tools ...string) error {
	var missing []string
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			missing = append(missing, tool)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("missing %s — install with: %s", strings.Join(missing, ", "), platform.InstallHint(missing...))
}

func fatal(err error) {
	style := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ff6b8a"))
	fmt.Fprintln(os.Stderr, style.Render("lofi-hiphopper: ")+err.Error())
	os.Exit(1)
}
