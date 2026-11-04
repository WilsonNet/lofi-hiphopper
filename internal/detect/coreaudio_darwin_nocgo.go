//go:build darwin && !cgo

package detect

import "fmt"

// CoreAudio requires cgo for the per-process HAL API.
type CoreAudio struct{}

// New returns a probe that explains the cgo requirement.
func New() *CoreAudio { return &CoreAudio{} }

// Processes reports that CGO_ENABLED=1 is required.
func (c *CoreAudio) Processes() ([]Process, error) {
	return nil, fmt.Errorf("audio detection needs cgo; rebuild with CGO_ENABLED=1")
}
