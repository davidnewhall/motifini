package chat

import (
	"fmt"
	"strings"

	"golift.io/subscribe"
)

// EventSourceHA is the catalog "source" rule value for events registered
// through the HTTP API, which is how Home Assistant files them.
const EventSourceHA = "ha"

// MaxEventNameLen caps event names so they fit in a Telegram callback_data
// payload (64 bytes) with the 4-byte e:s: / s:e: subscribe prefixes.
const MaxEventNameLen = 60

// IsHAEvent reports whether a catalog event was registered by Home Assistant.
func IsHAEvent(events *subscribe.Events, name string) bool {
	if events == nil {
		return false
	}

	source, ok := events.RuleGetS(name, "source")

	return ok && source == EventSourceHA
}

// CatalogEventsBySource splits subscribable catalog events into Home Assistant
// and system (built-in) groups. Each group stays sorted; camera clip settings
// keys are excluded via CatalogEventNames.
func CatalogEventsBySource(events *subscribe.Events) ([]string, []string) {
	var haEvents, sysEvents []string

	for _, name := range CatalogEventNames(events) {
		if IsHAEvent(events, name) {
			haEvents = append(haEvents, name)
		} else {
			sysEvents = append(sysEvents, name)
		}
	}

	return haEvents, sysEvents
}

// EventMenuNames returns catalog event names in menu order: Home Assistant
// events first, then system events.
func EventMenuNames(events *subscribe.Events) []string {
	haEvents, sysEvents := CatalogEventsBySource(events)

	return append(haEvents, sysEvents...)
}

// eventSectionRows builds event menu rows with Home Assistant / System section
// headers. dataPrefix is the subscribe callback prefix (e:s: or s:e:); buttons
// carry the event name rather than an index, so a catalog change while a menu
// is open can never subscribe someone to the wrong event. Events whose names
// cannot fit in a callback payload are returned as skipped for the menu text.
func (c *Chat) eventSectionRows(dataPrefix string) ([][]Button, []string) {
	haEvents, sysEvents := CatalogEventsBySource(c.Subs.Events)
	rows := make([][]Button, 0, len(haEvents)+len(sysEvents)+2)

	var skipped []string

	if len(haEvents) > 0 {
		rows = append(rows, []Button{{Label: "— Home Assistant —", Data: cbEvtsHdr}})

		for _, name := range haEvents {
			if btn, ok := c.eventMenuButton(name, dataPrefix); ok {
				rows = append(rows, []Button{btn})
			} else {
				skipped = append(skipped, name)
			}
		}
	}

	if len(sysEvents) > 0 {
		rows = append(rows, []Button{{Label: "— System —", Data: cbEvtsHdr}})

		for _, name := range sysEvents {
			if btn, ok := c.eventMenuButton(name, dataPrefix); ok {
				rows = append(rows, []Button{btn})
			} else {
				skipped = append(skipped, name)
			}
		}
	}

	return rows, skipped
}

// eventMenuButton builds a subscribe button for one catalog event, with the
// description appended when one is registered. ok is false when the event
// name cannot fit in a Telegram callback payload.
func (c *Chat) eventMenuButton(name, dataPrefix string) (Button, bool) {
	if len(name) > MaxEventNameLen {
		return Button{}, false
	}

	label := name
	if desc, _ := c.Subs.Events.RuleGetS(name, "description"); desc != "" {
		label = name + " — " + desc
	}

	// Truncate on rune boundaries so multi-byte descriptions never produce
	// invalid UTF-8 that Telegram would reject or garble.
	const maxLabelRunes = 64
	if runes := []rune(label); len(runes) > maxLabelRunes {
		label = string(runes[:maxLabelRunes-1]) + "…"
	}

	return Button{Label: label, Data: dataPrefix + name}, true
}

// skippedEventsNote mentions events hidden from a menu for over-long names.
func skippedEventsNote(skipped []string) string {
	if len(skipped) == 0 {
		return ""
	}

	return fmt.Sprintf("\n\n(%d hidden — name too long for a button: %s)",
		len(skipped), strings.Join(skipped, ", "))
}
