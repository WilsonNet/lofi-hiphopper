//go:build !darwin

package detect

import "fmt"

// CoreAudio is unavailable outside macOS.
type CoreAudio struct{}

// New returns a probe that always errors on unsupported platforms.
func New() *CoreAudio { return &CoreAudio{} }

// Processes reports that system audio detection needs macOS.
func (c *CoreAudio) Processes() ([]Process, error) {
	return nil, fmt.Errorf("system audio detection is macOS-only")
}
