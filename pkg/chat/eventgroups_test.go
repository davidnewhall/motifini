package chat

import (
	"slices"
	"testing"

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

	rows := c.eventSectionRows("e:s:")

	// 2 headers + 4 events.
	if len(rows) != 6 {
		t.Fatalf("rows: got %d want 6: %v", len(rows), rows)
	}

	if rows[0][0].Data != cbEvtsHdr || rows[3][0].Data != cbEvtsHdr {
		t.Fatalf("headers: got %q and %q", rows[0][0].Data, rows[3][0].Data)
	}

	// Indices must follow EventMenuNames order so e:s:{idx} resolves correctly.
	wantData := []string{"e:s:0", "e:s:1", "e:s:2", "e:s:3"}
	gotData := []string{rows[1][0].Data, rows[2][0].Data, rows[4][0].Data, rows[5][0].Data}

	if !slices.Equal(gotData, wantData) {
		t.Fatalf("button data: got %v want %v", gotData, wantData)
	}

	if rows[1][0].Label != "driveway_motion — HA driveway_motion" {
		t.Fatalf("label with description: got %q", rows[1][0].Label)
	}
}
