//go:build darwin

package service

import (
	"errors"
	"testing"
)

func TestSupported(t *testing.T) {
	t.Parallel()

	if !Supported() {
		t.Fatal("expected Supported on darwin")
	}
}

func TestRunUnknownAction(t *testing.T) {
	t.Parallel()

	err := Run(Action("nope"))
	if !errors.Is(err, ErrUnknownAction) {
		t.Fatalf("got %v want %v", err, ErrUnknownAction)
	}
}
