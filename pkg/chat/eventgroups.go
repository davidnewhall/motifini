package chat

import (
	"fmt"

	"golift.io/subscribe"
)

// EventSourceHA is the catalog "source" rule value for events registered
// through the HTTP API, which is how Home Assistant files them.
const EventSourceHA = "ha"

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
// events first, then system events. Subscribe callbacks carry an index into
// this list, so every menu and every index resolver must use this same order.
func EventMenuNames(events *subscribe.Events) []string {
	haEvents, sysEvents := CatalogEventsBySource(events)

	return append(haEvents, sysEvents...)
}

// eventSectionRows builds event menu rows with Home Assistant / System section
// headers. dataPrefix is the subscribe callback prefix (e:s: or s:e:); button
// indices follow EventMenuNames order so subWizardSubscribeEvt can resolve them.
func (c *Chat) eventSectionRows(dataPrefix string) [][]Button {
	haEvents, sysEvents := CatalogEventsBySource(c.Subs.Events)
	rows := make([][]Button, 0, len(haEvents)+len(sysEvents)+2)
	idx := 0

	if len(haEvents) > 0 {
		rows = append(rows, []Button{{Label: "— Home Assistant —", Data: cbEvtsHdr}})

		for _, name := range haEvents {
			rows = append(rows, []Button{c.eventMenuButton(name, idx, dataPrefix)})
			idx++
		}
	}

	if len(sysEvents) > 0 {
		rows = append(rows, []Button{{Label: "— System —", Data: cbEvtsHdr}})

		for _, name := range sysEvents {
			rows = append(rows, []Button{c.eventMenuButton(name, idx, dataPrefix)})
			idx++
		}
	}

	return rows
}

// eventMenuButton builds a subscribe button for one catalog event, with the
// description appended when one is registered. idx is the EventMenuNames index.
func (c *Chat) eventMenuButton(name string, idx int, dataPrefix string) Button {
	label := name
	if desc, _ := c.Subs.Events.RuleGetS(name, "description"); desc != "" {
		label = name + " — " + desc
		if len(label) > 64 {
			label = label[:61] + "…"
		}
	}

	return Button{Label: label, Data: fmt.Sprintf("%s%d", dataPrefix, idx)}
}
