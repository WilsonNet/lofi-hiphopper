//go:build !darwin && !linux

package detect

import "fmt"

// Unsupported is the probe for platforms without an audio backend.
type Unsupported struct{}

// New returns a probe that always errors on unsupported platforms.
func New() *Unsupported { return &Unsupported{} }

// Processes reports that system audio detection needs macOS or Linux.
func (u *Unsupported) Processes() ([]Process, error) {
	return nil, fmt.Errorf("system audio detection needs macOS or Linux")
}
