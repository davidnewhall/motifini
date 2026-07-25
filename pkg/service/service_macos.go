//go:build darwin

package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

const launchctlTimeout = 30 * time.Second

// Supported reports whether service control is available on this platform.
func Supported() bool { return true }

func run(action Action) error {
	uid := strconv.Itoa(os.Getuid())
	domain := "gui/" + uid
	service := domain + "/" + Label

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home directory: %w", err)
	}

	plist := filepath.Join(home, "Library", "LaunchAgents", Label+".plist")
	args := launchctlArgs(action, domain, service, plist)

	ctx, cancel := context.WithTimeout(context.Background(), launchctlTimeout)
	defer cancel()

	//nolint:gosec // fixed launchctl binary; args are uid/plist paths we build
	cmd := exec.CommandContext(ctx, "launchctl", args...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := string(out)
		if msg == "" {
			return fmt.Errorf("launchctl %s: %w", action, err)
		}

		return fmt.Errorf("launchctl %s: %w: %s", action, err, msg)
	}

	if len(out) > 0 {
		fmt.Print(string(out)) //nolint:forbidigo // surface launchctl stdout
	}

	fmt.Printf("%s %s\n", Label, action) //nolint:forbidigo // cli status

	return nil
}

func launchctlArgs(action Action, domain, service, plist string) []string {
	switch action {
	case Start:
		return []string{"bootstrap", domain, plist}
	case Stop:
		return []string{"bootout", service}
	default: // Restart (validated by Run)
		return []string{"kickstart", "-k", service}
	}
}
