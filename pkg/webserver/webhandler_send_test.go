package webserver

import (
	"testing"

	"github.com/davidnewhall/motifini/pkg/messenger"
)

func TestRecipientAllowed(t *testing.T) {
	t.Parallel()

	cfg, _ := testConfig(t)
	cfg.AllowedTo = []string{"999"}

	authed := cfg.Subs.CreateSubWithID(1234, "authed", messenger.APITelegram, false, false)
	authed.Meta = map[string]any{"hasAuth": true}

	unauthed := cfg.Subs.CreateSubWithID(5678, "unauthed", messenger.APITelegram, false, false)
	unauthed.Meta = map[string]any{"hasAuth": false}

	ignored := cfg.Subs.CreateSubWithID(4321, "ignored", messenger.APITelegram, false, true)
	ignored.Meta = map[string]any{"hasAuth": true}

	tests := []struct {
		name             string
		id               string
		allowSubscribers bool
		want             bool
	}{
		{"allowed_to list", "999", false, true},
		{"allowed_to list wins without flag", "999", true, true},
		{"unknown id, flag off", "1234", false, false},
		{"authed subscriber, flag on", "1234", true, true},
		{"unauthed subscriber, flag on", "5678", true, false},
		{"ignored subscriber, flag on", "4321", true, false},
		{"unknown id, flag on", "777", true, false},
		{"non-numeric id, flag on", "notanid", true, false},
		{"empty id, flag on", "", true, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg.AllowSubscribers = test.allowSubscribers
			if got := cfg.recipientAllowed(test.id); got != test.want {
				t.Fatalf("recipientAllowed(%q, flag=%v): got %v want %v",
					test.id, test.allowSubscribers, got, test.want)
			}
		})
	}
}
