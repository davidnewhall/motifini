//go:build !darwin

package service

import (
	"errors"
	"fmt"
)

// ErrUnsupported is returned when service control is unavailable on this OS.
var ErrUnsupported = errors.New("service control is only supported on macOS at this time")

// Supported reports whether service control is available on this platform.
func Supported() bool { return false }

func run(action Action) error {
	return fmt.Errorf("%w (%s)", ErrUnsupported, action)
}
