package webserver

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/davidnewhall/motifini/pkg/chat"
	"github.com/davidnewhall/motifini/pkg/messenger"
)

// eventRaceRounds is how many times each concurrent worker loops.
const eventRaceRounds = 15

// TestEventAPIConcurrentRegistration hammers the register, notify, list and
// remove endpoints from many goroutines at once, the way a busy Home Assistant
// does. Two requests can both find an event missing, and a rollback from one
// must never delete an event the other already registered and saved.
func TestEventAPIConcurrentRegistration(t *testing.T) {
	t.Parallel()

	cfg, _ := testConfig(t)

	var wait sync.WaitGroup

	for worker := range 6 {
		wait.Go(func() { registerShared(t, cfg) })
		wait.Go(func() { churnOwned(t, cfg, "owned_"+strconv.Itoa(worker)) })
	}

	wait.Wait()

	// Every shared event survived: no rollback dropped a registration that
	// another request had already saved.
	for round := range eventRaceRounds {
		event := "shared_" + strconv.Itoa(round)
		if !cfg.Subs.Events.Exists(event) {
			t.Errorf("event %s went missing after concurrent registration", event)
		}
	}
}

// registerShared races PUT and notify to create the same set of events.
func registerShared(t *testing.T, cfg *Config) {
	t.Helper()

	for round := range eventRaceRounds {
		event := "shared_" + strconv.Itoa(round)

		rec := doRequest(cfg.eventUpsertHandler, "PUT",
			"/api/v1.0/event/"+event, "", "", map[string]string{"event": event})
		if rec.Code != http.StatusOK {
			t.Errorf("PUT %s: code=%d body=%q", event, rec.Code, rec.Body.String())

			return
		}

		rec = doRequest(cfg.eventsHandler, "POST", "/api/v1.0/event/notify/"+event+"?msg=hi", "", "",
			map[string]string{"cmd": "notify", "event": event})
		if rec.Code != http.StatusOK {
			t.Errorf("notify %s: code=%d body=%q", event, rec.Code, rec.Body.String())

			return
		}
	}
}

// churnOwned registers, lists and removes one worker's own event, so removals
// and listings run against the catalog while other workers register into it.
func churnOwned(t *testing.T, cfg *Config, event string) {
	t.Helper()

	for range eventRaceRounds {
		rec := doRequest(cfg.eventUpsertHandler, "PUT",
			"/api/v1.0/event/"+event, "", "", map[string]string{"event": event})
		if rec.Code != http.StatusOK {
			t.Errorf("PUT %s: code=%d", event, rec.Code)

			return
		}

		rec = doRequest(cfg.eventsListHandler, "GET", "/api/v1.0/events", "", "", nil)
		if rec.Code != http.StatusOK {
			t.Errorf("list: code=%d", rec.Code)

			return
		}

		rec = doRequest(cfg.eventsHandler, "POST", "/api/v1.0/event/remove/"+event, "", "",
			map[string]string{"cmd": "remove", "event": event})
		if rec.Code != http.StatusOK {
			t.Errorf("remove %s: code=%d", event, rec.Code)

			return
		}
	}
}

// TestNotifyConcurrentSaveFailureAllFail: several notifies register the same
// unknown event at once while the state file is unwritable. Every one must
// report the failure. Unserialized, a request could find the event already in
// memory from a peer whose save failed, skip registration and answer 200 for an
// event that was never persisted — and then that peer's rollback deletes it.
func TestNotifyConcurrentSaveFailureAllFail(t *testing.T) {
	t.Parallel()

	cfg, stateFile := testConfig(t)

	err := os.RemoveAll(filepath.Dir(stateFile))
	if err != nil {
		t.Fatalf("remove temp dir: %v", err)
	}

	var wait sync.WaitGroup

	for range 8 {
		wait.Go(func() {
			rec := doRequest(cfg.eventsHandler, "POST", "/api/v1.0/event/notify/new_event?msg=hi", "", "",
				map[string]string{"cmd": "notify", "event": "new_event"})
			if rec.Code != http.StatusInternalServerError {
				t.Errorf("notify with failed save: code=%d want 500, body=%q", rec.Code, rec.Body.String())
			}
		})
	}

	wait.Wait()

	if cfg.Subs.Events.Exists("new_event") {
		t.Fatal("event must be rolled back when every save fails")
	}
}

// TestRecipientAllowedDuringAuthChanges reads the subscriber auth flags from an
// HTTP request goroutine while a Telegram handler flips them.
func TestRecipientAllowedDuringAuthChanges(t *testing.T) {
	t.Parallel()

	cfg, _ := testConfig(t)
	cfg.AllowSubscribers = true

	sub := cfg.Subs.CreateSubWithID(1234, "someone", messenger.APITelegram, false, false)

	const rounds = 200

	var wait sync.WaitGroup

	wait.Go(func() {
		for round := range rounds {
			chat.SetSubAuthed(sub, round%2 == 0)
			chat.SetSubIgnored(sub, round%3 == 0)
		}
	})

	wait.Go(func() {
		for range rounds {
			_ = cfg.recipientAllowed("1234")
		}
	})

	wait.Wait()
}
