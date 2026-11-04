package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/wilsonpsn/lofi-hiphopper/internal/audio"
	"github.com/wilsonpsn/lofi-hiphopper/internal/config"
	"github.com/wilsonpsn/lofi-hiphopper/internal/detect"
	"github.com/wilsonpsn/lofi-hiphopper/internal/media"
	"github.com/wilsonpsn/lofi-hiphopper/internal/player"
	"github.com/wilsonpsn/lofi-hiphopper/internal/ui"
)

const version = "0.1.0"

func main() {
	var (
		configPath = flag.String("config", "", "config file (default ~/.config/lofi-hiphopper/config.yaml)")
		demo       = flag.Bool("demo", false, "run the visualizer with simulated audio")
		showVer    = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *showVer {
		fmt.Println("lofi-hiphopper " + version)
		return
	}

	path := *configPath
	if strings.TrimSpace(path) == "" {
		path = config.Path()
	}
	cfg, created, err := config.LoadOrCreate(path)
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
	}, ui.Options{
		Demo:          *demo,
		ConfigPath:    path,
		CreatedConfig: created,
	})

	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithFPS(60))
	if _, err := program.Run(); err != nil {
		cleanup()
		fatal(err)
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
	return fmt.Errorf("missing %s — install with: brew install %s", strings.Join(missing, ", "), strings.Join(missing, " "))
}

func fatal(err error) {
	style := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ff6b8a"))
	fmt.Fprintln(os.Stderr, style.Render("lofi-hiphopper: ")+err.Error())
	os.Exit(1)
}
