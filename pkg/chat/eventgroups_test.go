package chat

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"golift.io/subscribe"
)

func testEventCatalog(t *testing.T) *subscribe.Subscribe {
	t.Helper()

	events := &subscribe.Events{Map: make(map[string]*subscribe.Rules)}
	data := &subscribe.Subscribe{Events: events}

	for _, name := range []string{"Motifini Started", "Camera Offline"} {
		err := events.New(name, &subscribe.Rules{
			S: map[string]string{"description": "built-in " + name},
		})
		if err != nil {
			t.Fatalf("seed built-in %s: %v", name, err)
		}
	}

	for _, name := range []string{"garage_opened", "driveway_motion"} {
		err := events.New(name, &subscribe.Rules{
			S: map[string]string{"description": "HA " + name, "source": EventSourceHA},
		})
		if err != nil {
			t.Fatalf("seed HA %s: %v", name, err)
		}
	}

	// Reserved camera settings keys must never leak into event menus.
	EnsureCameraSettings(data, "Office")

	return data
}

func TestCatalogEventsBySource(t *testing.T) {
	t.Parallel()

	data := testEventCatalog(t)
	haEvents, sysEvents := CatalogEventsBySource(data.Events)

	if !slices.Equal(haEvents, []string{"driveway_motion", "garage_opened"}) {
		t.Fatalf("ha events: got %v", haEvents)
	}

	if !slices.Equal(sysEvents, []string{"Camera Offline", "Motifini Started"}) {
		t.Fatalf("system events: got %v", sysEvents)
	}
}

func TestEventMenuNamesHAFirst(t *testing.T) {
	t.Parallel()

	data := testEventCatalog(t)
	want := []string{"driveway_motion", "garage_opened", "Camera Offline", "Motifini Started"}

	if got := EventMenuNames(data.Events); !slices.Equal(got, want) {
		t.Fatalf("menu order: got %v want %v", got, want)
	}
}

func TestIsHAEvent(t *testing.T) {
	t.Parallel()

	data := testEventCatalog(t)

	if !IsHAEvent(data.Events, "garage_opened") {
		t.Fatal("garage_opened should be HA-sourced")
	}

	if IsHAEvent(data.Events, "Motifini Started") {
		t.Fatal("built-in should not be HA-sourced")
	}

	if IsHAEvent(data.Events, "missing") || IsHAEvent(nil, "garage_opened") {
		t.Fatal("unknown event or nil events must not be HA-sourced")
	}
}

func TestEventSectionRows(t *testing.T) {
	t.Parallel()

	data := testEventCatalog(t)
	c := &Chat{Subs: data}

	rows, skipped := c.eventSectionRows("e:s:", nil)

	// 2 headers + 4 events.
	if len(rows) != 6 {
		t.Fatalf("rows: got %d want 6: %v", len(rows), rows)
	}

	if len(skipped) != 0 {
		t.Fatalf("skipped: got %v want none", skipped)
	}

	if rows[0][0].Data != cbEvtsHdr || rows[3][0].Data != cbEvtsHdr {
		t.Fatalf("headers: got %q and %q", rows[0][0].Data, rows[3][0].Data)
	}

	// Buttons carry event names (stable across catalog changes), not indices.
	wantData := []string{
		"e:s:driveway_motion", "e:s:garage_opened", "e:s:Camera Offline", "e:s:Motifini Started",
	}
	gotData := []string{rows[1][0].Data, rows[2][0].Data, rows[4][0].Data, rows[5][0].Data}

	if !slices.Equal(gotData, wantData) {
		t.Fatalf("button data: got %v want %v", gotData, wantData)
	}

	if rows[1][0].Label != "HA driveway_motion" {
		t.Fatalf("label with description: got %q", rows[1][0].Label)
	}
}

func TestEventSectionRowsOmitsSubscribed(t *testing.T) {
	t.Parallel()

	data := testEventCatalog(t)
	sub := &subscribe.Subscriber{
		ID: 1, API: "telegram", Contact: "Alice",
		Events: &subscribe.Events{Map: make(map[string]*subscribe.Rules)},
	}
	err := sub.Subscribe("driveway_motion")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	chat := &Chat{Subs: data}
	rows, skipped := chat.eventSectionRows("e:s:", sub)

	if len(skipped) != 0 {
		t.Fatalf("skipped: got %v want none", skipped)
	}

	gotData := make([]string, 0, len(rows))
	for _, row := range rows {
		gotData = append(gotData, row[0].Data)
	}

	wantData := []string{
		cbEvtsHdr, "e:s:garage_opened",
		cbEvtsHdr, "e:s:Camera Offline", "e:s:Motifini Started",
	}
	if !slices.Equal(gotData, wantData) {
		t.Fatalf("button data: got %v want %v", gotData, wantData)
	}

	// Without a subscriber, the subscribed event remains visible.
	allRows, _ := chat.eventSectionRows("e:s:", nil)
	allData := make([]string, 0, len(allRows))
	for _, row := range allRows {
		allData = append(allData, row[0].Data)
	}
	if !slices.Contains(allData, "e:s:driveway_motion") {
		t.Fatalf("nil sub must not filter: got %v", allData)
	}
}

func TestEventSectionRowsOmitsEmptySection(t *testing.T) {
	t.Parallel()

	data := testEventCatalog(t)
	sub := &subscribe.Subscriber{
		ID: 1, API: "telegram", Contact: "Alice",
		Events: &subscribe.Events{Map: make(map[string]*subscribe.Rules)},
	}
	for _, name := range []string{"driveway_motion", "garage_opened"} {
		err := sub.Subscribe(name)
		if err != nil {
			t.Fatalf("subscribe %s: %v", name, err)
		}
	}

	chat := &Chat{Subs: data}
	rows, _ := chat.eventSectionRows("s:e:", sub)

	gotData := make([]string, 0, len(rows))
	for _, row := range rows {
		gotData = append(gotData, row[0].Data)
	}

	wantData := []string{
		cbEvtsHdr, "s:e:Camera Offline", "s:e:Motifini Started",
	}
	if !slices.Equal(gotData, wantData) {
		t.Fatalf("button data: got %v want %v", gotData, wantData)
	}

	for _, row := range rows {
		if row[0].Label == "— Home Assistant —" {
			t.Fatal("empty HA section header should be omitted")
		}
	}
}

func TestEventSectionRowsAllSubscribed(t *testing.T) {
	t.Parallel()

	data := testEventCatalog(t)
	sub := &subscribe.Subscriber{
		ID: 1, API: "telegram", Contact: "Alice",
		Events: &subscribe.Events{Map: make(map[string]*subscribe.Rules)},
	}
	for _, name := range EventMenuNames(data.Events) {
		err := sub.Subscribe(name)
		if err != nil {
			t.Fatalf("subscribe %s: %v", name, err)
		}
	}

	chat := &Chat{Subs: data}
	rows, skipped := chat.eventSectionRows("e:s:", sub)
	if len(rows) != 0 || len(skipped) != 0 {
		t.Fatalf("want empty menu, got rows=%v skipped=%v", rows, skipped)
	}

	reply := chat.subWizardEvents(&Handler{Sub: sub})
	if !strings.Contains(reply.Reply, "subscribed to everything") {
		t.Fatalf("empty-menu note: got %q", reply.Reply)
	}
}

func TestUnsubWizardStillListsSubscribedEvents(t *testing.T) {
	t.Parallel()

	data := testEventCatalog(t)
	sub := &subscribe.Subscriber{
		ID: 1, API: "telegram", Contact: "Alice",
		Events: &subscribe.Events{Map: make(map[string]*subscribe.Rules)},
	}
	err := sub.Subscribe("garage_opened")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	chat := &Chat{Subs: data}
	reply := chat.unsubWizardRoot(&Handler{Sub: sub})

	found := false
	for _, row := range reply.Keyboard {
		for _, btn := range row {
			if strings.Contains(btn.Data, "garage_opened") || strings.Contains(btn.Label, "garage") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("unsubscribe menu must still list garage_opened: %+v", reply.Keyboard)
	}
}

func TestEventSectionRowsSkipsLongNames(t *testing.T) {
	t.Parallel()

	data := testEventCatalog(t)

	longName := strings.Repeat("a", MaxEventNameLen+1)

	err := data.Events.New(longName, &subscribe.Rules{
		S: map[string]string{"source": EventSourceHA},
	})
	if err != nil {
		t.Fatalf("seed long event: %v", err)
	}

	c := &Chat{Subs: data}
	rows, skipped := c.eventSectionRows("e:s:", nil)

	if !slices.Equal(skipped, []string{longName}) {
		t.Fatalf("skipped: got %v want [%q]", skipped, longName)
	}

	// 2 headers + 4 events; the long one is not a button.
	if len(rows) != 6 {
		t.Fatalf("rows: got %d want 6", len(rows))
	}

	if note := skippedEventsNote(skipped); !strings.Contains(note, longName) {
		t.Fatalf("note should name the hidden event: %q", note)
	}
}

func TestEventMenuButtonUsesDescriptionOrName(t *testing.T) {
	t.Parallel()

	events := &subscribe.Events{Map: make(map[string]*subscribe.Rules)}
	data := &subscribe.Subscribe{Events: events}

	err := events.New("gate_opened", &subscribe.Rules{
		S: map[string]string{"description": "Gate Open"},
	})
	if err != nil {
		t.Fatalf("seed described: %v", err)
	}

	err = events.New("no_desc_event", &subscribe.Rules{S: map[string]string{}})
	if err != nil {
		t.Fatalf("seed bare: %v", err)
	}

	chat := &Chat{Subs: data}

	btn, built := chat.eventMenuButton("gate_opened", "e:s:")
	if !built {
		t.Fatal("described button should build")
	}
	if btn.Label != "Gate Open" {
		t.Fatalf("pretty label: got %q want %q", btn.Label, "Gate Open")
	}
	if btn.Data != "e:s:gate_opened" {
		t.Fatalf("callback must use event id: got %q", btn.Data)
	}

	btn, built = chat.eventMenuButton("no_desc_event", "e:s:")
	if !built {
		t.Fatal("fallback button should build")
	}
	if btn.Label != "no_desc_event" {
		t.Fatalf("fallback label: got %q want event name", btn.Label)
	}
	if btn.Data != "e:s:no_desc_event" {
		t.Fatalf("callback must use event id: got %q", btn.Data)
	}
}

func TestEventMenuButtonTruncatesOnRunes(t *testing.T) {
	t.Parallel()

	events := &subscribe.Events{Map: make(map[string]*subscribe.Rules)}
	data := &subscribe.Subscribe{Events: events}

	err := events.New("gate_open", &subscribe.Rules{
		S: map[string]string{"description": strings.Repeat("é", 100)},
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	c := &Chat{Subs: data}
	btn, ok := c.eventMenuButton("gate_open", "e:s:")

	if !ok {
		t.Fatal("button should build")
	}

	runes := []rune(btn.Label)
	if len(runes) != 64 || !strings.HasSuffix(btn.Label, "…") {
		t.Fatalf("label: got %d runes, suffix %q", len(runes), btn.Label[len(btn.Label)-3:])
	}

	if !utf8.ValidString(btn.Label) {
		t.Fatal("label is not valid UTF-8")
	}
}

// TestSubscribeEvtEventRemovedMidFlight: the HTTP remove endpoint can drop an
// event between the menu button's name lookup and the subscribe. The wizard has
// to undo its own subscription instead of leaving one nobody can see.
func TestSubscribeEvtEventRemovedMidFlight(t *testing.T) {
	t.Parallel()

	data := testEventCatalog(t)
	sub := &subscribe.Subscriber{
		ID: 1, API: "telegram", Contact: "Alice",
		Events: &subscribe.Events{Map: make(map[string]*subscribe.Rules)},
	}
	data.Subscribers = []*subscribe.Subscriber{sub}
	c := &Chat{Subs: data}

	// Stand in for the concurrent remove: the name resolves, then it is gone.
	data.EventRemove("garage_opened")

	reply, save := c.subWizardSubscribeEvt(&Handler{Sub: sub}, "garage_opened")
	if save {
		t.Fatal("a vanished event must not be saved as a subscription")
	}

	if !strings.Contains(reply.Reply, "gone") {
		t.Fatalf("reply: %q", reply.Reply)
	}

	if sub.Events.Len() != 0 {
		t.Fatalf("subscriptions: got %d want 0", sub.Events.Len())
	}
}
