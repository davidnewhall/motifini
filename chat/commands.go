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
var NonAdminCommands = commandMap{
	Kind: "User",
	Map: map[string]chatCommands{
		"cams": chatCommands{
			Description: "Displays all available cameras by name.",
			Run:         cmdCams,
			Save:        false,
		},
		"events": chatCommands{
			Description: "Displays all available events.",
			Run:         cmdEvents,
			Save:        false,
		},
		"pics": chatCommands{
			Usage:       "[camera]",
			Description: "Sends pictures from all cameras, or from [camera].",
			Run:         cmdPics,
			Save:        false,
		},
		"sub": chatCommands{
			Usage:       "<camera|event>",
			Description: "Enables notifications from <camera> or <event>.",
			Run:         cmdSub,
			Save:        true,
		},
		"subs": chatCommands{
			Description: "Shows your subscriptions.",
			Run:         cmdSubs,
			Save:        false,
		},
		"unsub": chatCommands{
			Usage:       "<camera|event|*>",
			Description: "Stops notifications from <camera>, or <event> or everything if '*' is passed.",
			Run:         cmdUnsub,
			Save:        true,
		},
		"stop": chatCommands{
			Usage:       "[minutes] [camera]",
			Description: "Stops all motion notifications for 10 minutes or [minutes] on all cameras or [camera].",
			Run:         cmdStop,
			Save:        true,
		},
	},
}

func cmdCams(c *CommandHandle) (string, []string, error) {
	msg := "There are " + strconv.Itoa(len(Spy.Cameras.All())) + " cameras:\n"
	for _, cam := range Spy.Cameras.All() {
		msg += fmt.Sprintf("%v: %v\n", cam.Number, cam.Name)
	}
	return msg, nil, nil
}

func cmdEvents(c *CommandHandle) (string, []string, error) {
	events := Subs.Events.Names()
	msg := "There are " + strconv.Itoa(len(events)) + " events:\n"
	for i, event := range events {
		description, _ := Subs.Events.RuleGetS(event, "description")
		msg += strconv.Itoa(i) + ": " + event + " - " + description + "\n"
	}
	return msg, nil, nil
}

func cmdPics(c *CommandHandle) (string, []string, error) {
	msg := ""
	if len(c.Text) > 1 {
		name := strings.Join(c.Text[1:], " ")
		cam := Spy.Cameras.ByName(name)
		if cam == nil {
			return "Unknown Camera: " + name, nil, ErrorBadUsage
		}
		path := fmt.Sprintf("%vchat_command_%v_%v.jpg", TempDir, c.ID, cam.Name)
		if err := cam.SaveJPEG(&securityspy.VidOps{}, path); err != nil {
			log.Printf("[ERROR] [%v] cam.SaveJPEG: %v", c.ID, err)
			msg = "Error Getting '" + name + "' Picture: " + err.Error()
		}
		return msg, []string{path}, nil
	}
	var paths []string
	for _, cam := range Spy.Cameras.All() {
		path := fmt.Sprintf("%vchat_command_%v_%v.jpg", TempDir, c.ID, cam.Name)
		if err := cam.SaveJPEG(&securityspy.VidOps{}, path); err != nil {
			log.Printf("[ERROR] [%v] cam.SaveJPEG: %v", c.ID, err)
			msg += "Error Getting '" + cam.Name + "' Picture: " + err.Error() + "\n"
			continue
		}
		paths = append(paths, path)
	}
	// Give the file system time to sync
	time.Sleep(200 * time.Millisecond)
	return msg, paths, nil
}

func cmdSub(c *CommandHandle) (string, []string, error) {
	kind := "event"
	if len(c.Text) < 2 {
		return "", nil, ErrorBadUsage
	}
	event := strings.Join(c.Text[1:], " ")
	if !Subs.Events.Exists(event) {
		kind = "camera"
		if cam := Spy.Cameras.ByName(event); cam == nil {
			return "Event or Camera not found: " + event, nil, ErrorBadUsage
		}
	}
	msg := "You've been subscribed to " + kind + ": " + event
	if err := c.Sub.Subscribe(event); err != nil {
		msg = "You're already subscribed to: " + kind + ": " + event
	}
	msg += "\nYou have " + strconv.Itoa(c.Sub.Events.Len()) + " event subscriptions."
	return msg, nil, nil
}

func cmdSubs(c *CommandHandle) (string, []string, error) {
	if c.Sub.Admin && len(c.Text) > 1 {
		// admin asking for subs for someone else. handled by iMessageAdminSubs()
		return "", nil, nil
	}
	msg := "Your Subscriptions:"
	for i, event := range c.Sub.Events.Names() {
		msg += "\n" + strconv.Itoa(i) + ": " + event
		if c.Sub.Events.IsPaused(event) {
			msg += " (paused)"
		}
	}
	if msg += "\n"; c.Sub.Events.Len() == 0 {
		msg += "(none)\n"
	}
	return msg, nil, nil
}

func cmdUnsub(c *CommandHandle) (string, []string, error) {
	if len(c.Text) < 2 {
		return "", nil, ErrorBadUsage
	}
	event := strings.Join(c.Text[1:], " ")
	msg := "You've been unsubscribed from: " + event
	if event == "*" {
		for _, e := range c.Sub.Events.Names() {
			c.Sub.Events.Remove(e)
		}
		return "You've been unsubscribed all events.", nil, nil
	}
	if !c.Sub.Events.Exists(event) {
		msg = "You're not subscribed to: " + event
	}
	c.Sub.Events.Remove(event)
	msg += "\nYou have " + strconv.Itoa(c.Sub.Events.Len()) + " event subscriptions."
	return msg, nil, nil
}

func cmdStop(c *CommandHandle) (string, []string, error) {
	if len(c.Text) < 2 {
		return "", nil, ErrorBadUsage
	}
	dur, err := strconv.Atoi(c.Text[1])
	if err != nil {
		return "Unable to parse into a number: " + c.Text[1], nil, ErrorBadUsage
	}

	// Pause a single event.
	if len(c.Text) > 2 {
		event := strings.Join(c.Text[2:], " ")
		msg := "Notifications from '" + event + "' paused for at least " + c.Text[1] + " minutes."
		if dur == 0 {
			msg = "Notifications from '" + event + " are no longer paused."
		}
		if err := c.Sub.Events.Pause(event, time.Duration(dur)*time.Minute); err != nil {
			msg = "You're not subscribed to: " + event
		}
		return msg, nil, nil
	}

	// Pause Everything.
	for _, event := range c.Sub.Events.Names() {
		_ = c.Sub.Events.Pause(event, time.Duration(dur)*time.Minute)
	}
	msg := "Notifications paused for at least " + c.Text[1] + " minutes."
	if dur == 0 {
		msg = "Notifications are no longer paused."
	}
	return msg, nil, nil
}
