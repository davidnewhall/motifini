//go:build darwin

package service

import (
	"errors"
	"slices"
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

func TestLaunchctlArgs(t *testing.T) {
	t.Parallel()

	const (
		domain  = "gui/501"
		service = "gui/501/io.golift.motifini"
		plist   = "/Users/test/Library/LaunchAgents/io.golift.motifini.plist"
	)

	cases := []struct {
		action Action
		want   []string
	}{
		{Start, []string{"bootstrap", domain, plist}},
		{Stop, []string{"bootout", service}},
		{Restart, []string{"kickstart", "-k", service}},
	}

	for _, tc := range cases {
		got := launchctlArgs(tc.action, domain, service, plist)
		if !slices.Equal(got, tc.want) {
			t.Fatalf("%s: got %v want %v", tc.action, got, tc.want)
		}
	}
}
