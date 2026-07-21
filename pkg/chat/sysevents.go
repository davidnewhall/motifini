package chat

import "golift.io/subscribe"

// Built-in non-camera system events users can subscribe to.
const (
	EventStarted       = "Motifini Started"
	EventStreamDown    = "Event Stream Down"
	EventStreamUp      = "Event Stream Up"
	EventCameraOffline = "Camera Offline"
	EventCameraOnline  = "Camera Online"
	EventSecSpyError   = "SecuritySpy Error"
)

// BuiltInEvent is a catalog entry for the /events subscribe menu.
type BuiltInEvent struct {
	Name string
	Desc string
}

// BuiltInEvents are registered at startup so they appear in the Event subscribe wizard.
func BuiltInEvents() []BuiltInEvent {
	return []BuiltInEvent{
		{
			Name: EventStarted,
			Desc: "Motifini finished starting (Telegram is ready)",
		},
		{
			Name: EventStreamDown,
			Desc: "Motifini lost the live link to SecuritySpy (no motion alerts until it reconnects)",
		},
		{
			Name: EventStreamUp,
			Desc: "Motifini reconnected to SecuritySpy's event stream",
		},
		{
			Name: EventCameraOffline,
			Desc: "Any camera dropped offline",
		},
		{
			Name: EventCameraOnline,
			Desc: "Any camera came back online",
		},
		{
			Name: EventSecSpyError,
			Desc: "SecuritySpy reported an ERROR on the event stream",
		},
	}
}

// EnsureBuiltInEvents registers system events in the global event catalog.
func EnsureBuiltInEvents(data *subscribe.Subscribe) {
	if data == nil || data.Events == nil {
		return
	}

	for _, event := range BuiltInEvents() {
		if data.Events.Exists(event.Name) {
			data.Events.RuleSetS(event.Name, "description", event.Desc)
			continue
		}

		_ = data.Events.New(event.Name, &subscribe.Rules{
			S: map[string]string{"description": event.Desc},
		})
	}
}
