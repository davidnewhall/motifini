// Package service controls the Motifini background service (LaunchAgent on macOS).
package service

import (
	"errors"
	"fmt"
)

// Action is a service control operation.
type Action string

// Supported service control actions.
const (
	Start   Action = "start"
	Stop    Action = "stop"
	Restart Action = "restart"
)

// Label is the LaunchAgent / service identifier.
const Label = "io.golift.motifini"

// ErrUnknownAction is returned when Run is given an unsupported action.
var ErrUnknownAction = errors.New("unknown service action")

// Run executes a service control action for the current platform.
func Run(action Action) error {
	switch action {
	case Start, Stop, Restart:
		return run(action)
	default:
		return fmt.Errorf("%w: %q", ErrUnknownAction, action)
	}
}
