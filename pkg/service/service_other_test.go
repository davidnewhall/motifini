//go:build !darwin

package service

import (
	"errors"
	"testing"
)

func TestSupported(t *testing.T) {
	t.Parallel()

	if Supported() {
		t.Fatal("expected Supported false off darwin")
	}
}

func TestRunUnsupported(t *testing.T) {
	t.Parallel()

	err := Run(Start)
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("got %v want %v", err, ErrUnsupported)
	}
}
