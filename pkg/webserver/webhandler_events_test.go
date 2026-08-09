package webserver

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidnewhall/motifini/pkg/chat"
	"github.com/davidnewhall/motifini/pkg/export"
	"github.com/davidnewhall/motifini/pkg/messenger"
	"github.com/gorilla/mux"
	"golift.io/subscribe"
)

// TestMain initializes the expvar export map that finishReq/finishReqJSON increment.
func TestMain(m *testing.M) {
	export.Init("motifini_webserver_test")
	os.Exit(m.Run())
}

func testConfig(t *testing.T) (*Config, string) {
	t.Helper()

	stateFile := filepath.Join(t.TempDir(), "subscribers.json")

	subs, err := subscribe.GetDB(stateFile)
	if err != nil {
		t.Fatalf("subscribe.GetDB: %v", err)
	}

	return &Config{
		Subs:    subs,
		Msgs:    &messenger.Messenger{},
		TempDir: t.TempDir(),
		Info:    log.New(io.Discard, "", 0),
		Debug:   log.New(io.Discard, "", 0),
		Error:   log.New(io.Discard, "", 0),
	}, stateFile
}

func doRequest(handler http.HandlerFunc, method, target, body, contentType string,
	vars map[string]string,
) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	req := httptest.NewRequestWithContext(context.Background(), method, target, reader)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	rec := httptest.NewRecorder()
	handler(rec, mux.SetURLVars(req, vars))

	return rec
}

func TestEventUpsertFormAndJSON(t *testing.T) {
	t.Parallel()

	cfg, _ := testConfig(t)
	vars := map[string]string{"event": "garage_opened"}

	rec := doRequest(cfg.eventUpsertHandler, "PUT",
		"/api/v1.0/event/garage_opened?description=Big+garage+door", "", "", vars)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "registered") {
		t.Fatalf("register: code=%d body=%q", rec.Code, rec.Body.String())
	}

	desc, _ := cfg.Subs.Events.RuleGetS("garage_opened", "description")
	source, _ := cfg.Subs.Events.RuleGetS("garage_opened", "source")
	if desc != "Big garage door" || source != chat.EventSourceHA {
		t.Fatalf("registered rules: desc=%q source=%q", desc, source)
	}

	// JSON body update keeps the source and replaces the description.
	rec = doRequest(cfg.eventUpsertHandler, "PUT",
		"/api/v1.0/event/garage_opened", `{"description":"Garage door opened"}`,
		"application/json", vars)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "updated") {
		t.Fatalf("update: code=%d body=%q", rec.Code, rec.Body.String())
	}

	desc, _ = cfg.Subs.Events.RuleGetS("garage_opened", "description")
	source, _ = cfg.Subs.Events.RuleGetS("garage_opened", "source")
	if desc != "Garage door opened" || source != chat.EventSourceHA {
		t.Fatalf("updated rules: desc=%q source=%q", desc, source)
	}
}

func TestEventUpsertRejects(t *testing.T) {
	t.Parallel()

	cfg, _ := testConfig(t)

	rec := doRequest(cfg.eventUpsertHandler, "PUT",
		"/api/v1.0/event/__cam:Office", "", "", map[string]string{"event": "__cam:Office"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("reserved key: code=%d want 400", rec.Code)
	}

	rec = doRequest(cfg.eventUpsertHandler, "PUT",
		"/api/v1.0/event/foo", `{bad json`, "application/json", map[string]string{"event": "foo"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad json: code=%d want 400", rec.Code)
	}

	if cfg.Subs.Events.Exists("foo") || cfg.Subs.Events.Exists("__cam:Office") {
		t.Fatal("rejected upserts must not create catalog entries")
	}
}

// TestEventUpsertSavesEveryTime: a repeat PUT with an identical payload must
// still rewrite the state file, so a retry recovers from an earlier save failure.
func TestEventUpsertSavesEveryTime(t *testing.T) {
	t.Parallel()

	cfg, stateFile := testConfig(t)
	vars := map[string]string{"event": "garage_opened"}

	rec := doRequest(cfg.eventUpsertHandler, "PUT", "/api/v1.0/event/garage_opened", "", "", vars)
	if rec.Code != http.StatusOK {
		t.Fatalf("first PUT: code=%d", rec.Code)
	}

	// Simulate a lost write: the file disappears after the first PUT.
	err := os.Remove(stateFile)
	if err != nil {
		t.Fatalf("remove state file: %v", err)
	}

	rec = doRequest(cfg.eventUpsertHandler, "PUT", "/api/v1.0/event/garage_opened", "", "", vars)
	if rec.Code != http.StatusOK {
		t.Fatalf("repeat PUT: code=%d", rec.Code)
	}

	_, err = os.Stat(stateFile)
	if err != nil {
		t.Fatalf("repeat PUT did not rewrite the state file: %v", err)
	}
}

// TestUpsertSaveFailureRollsBackNewEvent: a PUT whose save fails must not leave
// the new event in memory. A later notify would see it as registered, skip its
// own save and answer 200 for an event that disappears on restart.
func TestUpsertSaveFailureRollsBackNewEvent(t *testing.T) {
	t.Parallel()

	cfg, stateFile := testConfig(t)
	vars := map[string]string{"event": "garage_opened"}

	// Make the state file unwritable: its parent directory is gone.
	err := os.RemoveAll(filepath.Dir(stateFile))
	if err != nil {
		t.Fatalf("remove temp dir: %v", err)
	}

	rec := doRequest(cfg.eventUpsertHandler, "PUT", "/api/v1.0/event/garage_opened", "", "", vars)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("PUT with failed save: code=%d want 500", rec.Code)
	}

	if cfg.Subs.Events.Exists("garage_opened") {
		t.Fatal("a new event must be rolled back when the save fails")
	}

	// The follow-up notify has to retry the registration rather than trust the
	// catalog, so it reports the persistence failure too.
	rec = doRequest(cfg.eventsHandler, "POST", "/api/v1.0/event/notify/garage_opened?msg=hi", "", "",
		map[string]string{"cmd": "notify", "event": "garage_opened"})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("notify after rolled-back PUT: code=%d want 500", rec.Code)
	}
}

// TestUpsertSaveFailureKeepsExistingEvent: a failed *update* keeps the event.
// Only creations roll back, and the next PUT saves again.
func TestUpsertSaveFailureKeepsExistingEvent(t *testing.T) {
	t.Parallel()

	cfg, stateFile := testConfig(t)
	vars := map[string]string{"event": "garage_opened"}

	rec := doRequest(cfg.eventUpsertHandler, "PUT", "/api/v1.0/event/garage_opened", "", "", vars)
	if rec.Code != http.StatusOK {
		t.Fatalf("first PUT: code=%d", rec.Code)
	}

	err := os.RemoveAll(filepath.Dir(stateFile))
	if err != nil {
		t.Fatalf("remove temp dir: %v", err)
	}

	rec = doRequest(cfg.eventUpsertHandler, "PUT",
		"/api/v1.0/event/garage_opened?description=New+text", "", "", vars)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("update with failed save: code=%d want 500", rec.Code)
	}

	if !cfg.Subs.Events.Exists("garage_opened") {
		t.Fatal("a failed update must not remove an already-registered event")
	}
}

// TestNotifySaveFailureRollsBack: when the state save fails after
// auto-registration, the notify must fail (500) and the event must be rolled
// back so a later notify retries the registration.
func TestNotifySaveFailureRollsBack(t *testing.T) {
	t.Parallel()

	cfg, stateFile := testConfig(t)

	// Make the state file unwritable: its parent directory is gone.
	err := os.RemoveAll(filepath.Dir(stateFile))
	if err != nil {
		t.Fatalf("remove temp dir: %v", err)
	}

	rec := doRequest(cfg.eventsHandler, "POST", "/api/v1.0/event/notify/new_event?msg=hi", "", "",
		map[string]string{"cmd": "notify", "event": "new_event"})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("notify with failed save: code=%d want 500", rec.Code)
	}

	if cfg.Subs.Events.Exists("new_event") {
		t.Fatal("event must be rolled back when the save fails")
	}
}

func TestEventsList(t *testing.T) {
	t.Parallel()

	cfg, _ := testConfig(t)
	cfg.upsertCatalogEvent("garage_opened", "Big garage door")
	chat.EnsureBuiltInEvents(cfg.Subs)
	chat.EnsureCameraSettings(cfg.Subs, "Office")

	sub := cfg.Subs.CreateSubWithID(1234, "someone", messenger.APITelegram, false, false)

	err := sub.Subscribe("garage_opened")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	rec := doRequest(cfg.eventsListHandler, "GET", "/api/v1.0/events", "", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: code=%d body=%q", rec.Code, rec.Body.String())
	}

	var events []eventListEntry

	err = json.Unmarshal(rec.Body.Bytes(), &events)
	if err != nil {
		t.Fatalf("list json: %v", err)
	}

	byName := make(map[string]eventListEntry, len(events))
	for _, entry := range events {
		byName[entry.Event] = entry
		if strings.HasPrefix(entry.Event, "__cam:") {
			t.Fatalf("camera settings key leaked into event list: %s", entry.Event)
		}
	}

	haEvent, ok := byName["garage_opened"]
	if !ok {
		t.Fatalf("garage_opened missing from list: %v", events)
	}

	if haEvent.Description != "Big garage door" || haEvent.Source != chat.EventSourceHA ||
		haEvent.Subscribers != 1 {
		t.Fatalf("garage_opened entry: %+v", haEvent)
	}

	if sysEvent, ok := byName[chat.EventStarted]; !ok || sysEvent.Source != "" {
		t.Fatalf("built-in entry: ok=%v %+v", ok, sysEvent)
	}
}

func TestNotifyValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target string
		code   int
	}{
		{"empty", "/api/v1.0/event/notify/foo", http.StatusBadRequest},
		{"msg only", "/api/v1.0/event/notify/foo?msg=hi", http.StatusOK},
		{"explicit none", "/api/v1.0/event/notify/foo?msg=hi&media=none", http.StatusOK},
		{"none no msg", "/api/v1.0/event/notify/foo?media=none", http.StatusBadRequest},
		{"photo no camera", "/api/v1.0/event/notify/foo?media=photo", http.StatusBadRequest},
		{"video no camera", "/api/v1.0/event/notify/foo?media=video", http.StatusBadRequest},
		{"bad media", "/api/v1.0/event/notify/foo?msg=hi&media=gif", http.StatusBadRequest},
		// No SecuritySpy configured: any media request is a 503.
		{"camera default photo", "/api/v1.0/event/notify/foo?camera=Garage", http.StatusServiceUnavailable},
		{"photo camera", "/api/v1.0/event/notify/foo?camera=Garage&media=photo", http.StatusServiceUnavailable},
		{"video camera", "/api/v1.0/event/notify/foo?camera=3&media=video&msg=hi", http.StatusServiceUnavailable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg, _ := testConfig(t)
			rec := doRequest(cfg.eventsHandler, "POST", test.target, "", "",
				map[string]string{"cmd": "notify", "event": "foo"})

			if rec.Code != test.code {
				t.Fatalf("%s: code=%d want %d body=%q", test.target, rec.Code, test.code, rec.Body.String())
			}
		})
	}
}

func TestNotifyAutoUpsert(t *testing.T) {
	t.Parallel()

	cfg, _ := testConfig(t)

	rec := doRequest(cfg.eventsHandler, "POST",
		"/api/v1.0/event/notify/power_restored?msg=House+power+is+back&description=Power+restored", "", "",
		map[string]string{"cmd": "notify", "event": "power_restored"})
	if rec.Code != http.StatusOK {
		t.Fatalf("notify: code=%d body=%q", rec.Code, rec.Body.String())
	}

	desc, _ := cfg.Subs.Events.RuleGetS("power_restored", "description")
	source, _ := cfg.Subs.Events.RuleGetS("power_restored", "source")
	if desc != "Power restored" || source != chat.EventSourceHA {
		t.Fatalf("auto-registered rules: desc=%q source=%q", desc, source)
	}

	// A later notify must not clobber the registered description.
	rec = doRequest(cfg.eventsHandler, "POST",
		"/api/v1.0/event/notify/power_restored?msg=again&description=Changed", "", "",
		map[string]string{"cmd": "notify", "event": "power_restored"})
	if rec.Code != http.StatusOK {
		t.Fatalf("notify again: code=%d body=%q", rec.Code, rec.Body.String())
	}

	desc, _ = cfg.Subs.Events.RuleGetS("power_restored", "description")
	if desc != "Power restored" {
		t.Fatalf("description clobbered: %q", desc)
	}

	// Reserved camera settings keys are never auto-registered.
	rec = doRequest(cfg.eventsHandler, "POST",
		"/api/v1.0/event/notify/__cam:Office?msg=hi", "", "",
		map[string]string{"cmd": "notify", "event": "__cam:Office"})
	if rec.Code != http.StatusOK {
		t.Fatalf("notify cam key: code=%d body=%q", rec.Code, rec.Body.String())
	}

	if cfg.Subs.Events.Exists("__cam:Office") {
		t.Fatal("notify must not register camera settings keys")
	}
}

func TestEventRemove(t *testing.T) {
	t.Parallel()

	cfg, stateFile := testConfig(t)
	cfg.upsertCatalogEvent("garage_opened", "Big garage door")

	sub := cfg.Subs.CreateSubWithID(1234, "someone", messenger.APITelegram, false, false)

	err := sub.Subscribe("garage_opened")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	rec := doRequest(cfg.eventsHandler, "POST", "/api/v1.0/event/remove/garage_opened", "", "",
		map[string]string{"cmd": "remove", "event": "garage_opened"})
	if rec.Code != http.StatusOK {
		t.Fatalf("remove: code=%d body=%q", rec.Code, rec.Body.String())
	}

	if cfg.Subs.Events.Exists("garage_opened") {
		t.Fatal("catalog event still present after remove")
	}

	if sub.Events.Exists("garage_opened") {
		t.Fatal("subscriber subscription still present after remove")
	}

	// The state file on disk must reflect the removal.
	buf, err := os.ReadFile(stateFile) //nolint:gosec // test reads the state file it just wrote
	if err != nil {
		t.Fatalf("read state file: %v", err)
	}

	if strings.Contains(string(buf), "garage_opened") {
		t.Fatal("state file still contains removed event")
	}
}
