package chat

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"golift.io/securityspy"
)

// NonAdminCommands contains all the non-admin commands.
func (c *Chat) NonAdminCommands() *CommandMap {
	return &CommandMap{
		Kind:  "User",
		Level: 1,
		Map: map[string]Command{
			"cams": Command{
				Description: "Displays all available cameras by name.",
				Run:         c.cmdCams,
				Save:        false,
			},
			"events": Command{
				Description: "Displays all available events.",
				Run:         c.cmdEvents,
				Save:        false,
			},
			"pics": Command{
				Usage:       "[camera]",
				Description: "Sends pictures from all cameras, or from [camera].",
				Run:         c.cmdPics,
				Save:        false,
			},
			"sub": Command{
				Usage:       "<camera|event>",
				Description: "Enables notifications from <camera> or <event>.",
				Run:         c.cmdSub,
				Save:        true,
			},
			"subs": Command{
				Description: "Shows your subscriptions.",
				Run:         c.cmdSubs,
				Save:        false,
			},
			"unsub": Command{
				Usage:       "<camera|event|*>",
				Description: "Stops notifications from <camera>, or <event> or everything if '*' is passed.",
				Run:         c.cmdUnsub,
				Save:        true,
			},
			"stop": Command{
				Usage:       "[minutes] [camera]",
				Description: "Stops all motion notifications for 10 minutes or [minutes] on all cameras or [camera].",
				Run:         c.cmdStop,
				Save:        true,
			},
		},
	}
}

func (c *Chat) cmdCams(h *CommandHandle) (string, []string, error) {
	msg := "There are " + strconv.Itoa(len(c.Spy.Cameras.All())) + " cameras:\n"
	for _, cam := range c.Spy.Cameras.All() {
		msg += fmt.Sprintf("%v: %v\n", cam.Number, cam.Name)
	}
	return msg, nil, nil
}

func (c *Chat) cmdEvents(h *CommandHandle) (string, []string, error) {
	events := c.Subs.Events.Names()
	msg := "There are " + strconv.Itoa(len(events)) + " events:\n"
	for i, event := range events {
		description, _ := c.Subs.Events.RuleGetS(event, "description")
		msg += strconv.Itoa(i) + ": " + event + " - " + description + "\n"
	}
	return msg, nil, nil
}

func (c *Chat) cmdPics(h *CommandHandle) (string, []string, error) {
	msg := ""
	if len(h.Text) > 1 {
		name := strings.Join(h.Text[1:], " ")
		cam := c.Spy.Cameras.ByName(name)
		if cam == nil {
			return "Unknown Camera: " + name, nil, ErrorBadUsage
		}
		path := fmt.Sprintf("%vchat_command_%v_%v.jpg", c.TempDir, h.ID, cam.Name)
		if err := cam.SaveJPEG(&securityspy.VidOps{}, path); err != nil {
			log.Printf("[ERROR] [%v] cam.SaveJPEG: %v", h.ID, err)
			msg = "Error Getting '" + name + "' Picture: " + err.Error()
		}
		return msg, []string{path}, nil
	}
	paths := []string{}
	for _, cam := range c.Spy.Cameras.All() {
		path := fmt.Sprintf("%vchat_command_%v_%v.jpg", c.TempDir, h.ID, cam.Name)
		if err := cam.SaveJPEG(&securityspy.VidOps{}, path); err != nil {
			log.Printf("[ERROR] [%v] cam.SaveJPEG: %v", h.ID, err)
			msg += "Error Getting '" + cam.Name + "' Picture: " + err.Error() + "\n"
			continue
		}
		paths = append(paths, path)
	}
	// Give the file system time to sync
	time.Sleep(200 * time.Millisecond)
	return msg, paths, nil
}

func (c *Chat) cmdSub(h *CommandHandle) (string, []string, error) {
	kind := "event"
	if len(h.Text) < 2 {
		return "", nil, ErrorBadUsage
	}
	event := strings.Join(h.Text[1:], " ")
	if !c.Subs.Events.Exists(event) {
		kind = "camera"
		if cam := c.Spy.Cameras.ByName(event); cam == nil {
			return "Event or Camera not found: " + event, nil, ErrorBadUsage
		}
	}
	msg := "You've been subscribed to " + kind + ": " + event
	if err := h.Sub.Subscribe(event); err != nil {
		msg = "You're already subscribed to: " + kind + ": " + event
	}
	msg += "\nYou have " + strconv.Itoa(h.Sub.Events.Len()) + " event subscriptions."
	return msg, nil, nil
}

func (c *Chat) cmdSubs(h *CommandHandle) (string, []string, error) {
	if h.Sub.Admin && len(h.Text) > 1 {
		// admin asking for subs for someone else. handled by iMessageAdminSubs()
		return "", nil, nil
	}
	msg := "Your Subscriptions:"
	for i, event := range h.Sub.Events.Names() {
		msg += "\n" + strconv.Itoa(i) + ": " + event
		if h.Sub.Events.IsPaused(event) {
			msg += " (paused)"
		}
	}
	if msg += "\n"; h.Sub.Events.Len() == 0 {
		msg += "(none)\n"
	}
	return msg, nil, nil
}

func (c *Chat) cmdUnsub(h *CommandHandle) (string, []string, error) {
	if len(h.Text) < 2 {
		return "", nil, ErrorBadUsage
	}
	event := strings.Join(h.Text[1:], " ")
	msg := "You've been unsubscribed from: " + event
	if event == "*" {
		for _, e := range h.Sub.Events.Names() {
			h.Sub.Events.Remove(e)
		}
		return "You've been unsubscribed all events.", nil, nil
	}
	if !h.Sub.Events.Exists(event) {
		msg = "You're not subscribed to: " + event
	}
	h.Sub.Events.Remove(event)
	msg += "\nYou have " + strconv.Itoa(h.Sub.Events.Len()) + " event subscriptions."
	return msg, nil, nil
}

func (c *Chat) cmdStop(h *CommandHandle) (string, []string, error) {
	if len(h.Text) < 2 {
		return "", nil, ErrorBadUsage
	}
	dur, err := strconv.Atoi(h.Text[1])
	if err != nil {
		return "Unable to parse into a number: " + h.Text[1], nil, ErrorBadUsage
	}

	// Pause a single event.
	if len(h.Text) > 2 {
		event := strings.Join(h.Text[2:], " ")
		msg := "Notifications from '" + event + "' paused for at least " + h.Text[1] + " minutes."
		if dur == 0 {
			msg = "Notifications from '" + event + " are no longer paused."
		}
		if err := h.Sub.Events.Pause(event, time.Duration(dur)*time.Minute); err != nil {
			msg = "You're not subscribed to: " + event
		}
		return msg, nil, nil
	}

	// Pause Everything.
	for _, event := range h.Sub.Events.Names() {
		_ = h.Sub.Events.Pause(event, time.Duration(dur)*time.Minute)
	}
	msg := "Notifications paused for at least " + h.Text[1] + " minutes."
	if dur == 0 {
		msg = "Notifications are no longer paused."
	}
	return msg, nil, nil
}
