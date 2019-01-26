package main

import (
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/golift/imessage"
	"github.com/golift/subscribe"
)

// /api/v1.0/recv/imessage/msg/{from}
func (c *Config) recvMessageHandler(msg imessage.Incoming) {
	id := ReqID(4)
	text := strings.Fields(msg.Text)
	reply := imessage.Outgoing{To: msg.From, ID: id}

	requestor, err := c.subs.GetSubscriber(msg.From, APIiMessage)
	if err != nil {
		// Every account we receive a message from gets logged as a subscriber with no subscriptions.
		requestor = c.subs.CreateSub(msg.From, APIiMessage, len(c.subs.GetAdmins()) == 0, false)
	}

	if !requestor.Ignored {
		switch strings.ToLower(text[0]) {
		case "cams":
			reply.Text = c.iMessageCams()
		case "events":
			reply.Text = c.iMessageEvents(text)
		case "pics":
			reply.Text = c.iMessagePics(msg.From, id, text)
		case "sub":
			reply.Text = c.iMessageSub(text, requestor)
		case "subs":
			reply.Text = c.iMessageSubs(text, requestor)
		case "unsub":
			reply.Text = c.iMessageUnsub(text, requestor)
		case "stop":
			reply.Text = c.iMessageStop(text, requestor)
		case "help":
			reply.Text = c.iMessageHelp()
		}
	}
	if requestor.Admin {
		switch strings.ToLower(text[0]) {
		case "ignores":
			reply.Text = c.iMessageAdminIgnores()
		case "ignore":
			reply.Text = c.iMessageAdminIgnore(text)
		case "unignore":
			reply.Text = c.iMessageAdminUnignore(text)
		case "admins":
			reply.Text = c.iMessageAdminAdmins()
		case "admin":
			reply.Text = c.iMessageAdminAdmin(text)
		case "unadmin":
			reply.Text = c.iMessageAdminUnadmin(text)
		case "subs":
			reply.Text += c.iMessageAdminSubs(text)
		case "help":
			reply.Text += c.iMessageAdminHelp()
		}
		if reply.Text != "" {
			c.msgs.Send(reply)
		}
	}
}

func (c *Config) iMessageHelp() string {
	msg := `- iMessageSpy Help -
Available User Commands:
cams - Displays all available cameras by name.
events - Displays all available events.
pics [camera] - Sends pictures from all cameras, or from [camera].
sub <camera|event> - Enables notifications from <camera> or <event>.
unsub <camera|event|*> - Stops notifications from <camera>, lor <event> or everything if '*' is passed.
stop [minutes] [camera] - Stops all motion notifications for 10 minutes or [minutes] on all cameras or [camera]
subs - Shows your subscriptions.`
	return msg
}

func (c *Config) iMessageAdminHelp() string {
	msg := "\n\n" + `Available Admin Commands:
subs [subscriber] - Shows subscribers, or details for [subscriber].
ignores - Lists all ignored handles.
ignore <handle> - Ignores <handle>; stops notifications.
unignore <handle> - Stop ignoring <handle>
admins - Lists all administrator handles.
admin <handle> - Makes <handle> an admin.
unadmin <handle> - Take away admin from <handle>`
	return msg
}

func (c *Config) iMessageAdminAdmins() string {
	admins := c.subs.GetAdmins()
	msg := "There are " + strconv.Itoa(len(admins)) + " admins:"
	for i, admin := range admins {
		msg += "\n" + strconv.Itoa(i+1) + ": (" + admin.API + ") " + admin.Contact + " (" + strconv.Itoa(len(admin.Subscriptions())) + " subscriptions)"
	}
	return msg
}

func (c *Config) iMessageAdminIgnores() string {
	ignores := c.subs.GetIgnored()
	msg := "There are " + strconv.Itoa(len(ignores)) + " ignored subscribers:"
	for i, ignore := range ignores {
		msg += "\n" + strconv.Itoa(i+1) + ": (" + ignore.API + ") " + ignore.Contact + " (" + strconv.Itoa(len(ignore.Subscriptions())) + " subscriptions)"
	}
	return msg
}

func (c *Config) iMessageAdminSubs(text []string) string {
	if len(text) == 1 {
		subs := c.subs.GetAllSubscribers()
		msg := "There are " + strconv.Itoa(len(subs)) + " total subscribers:"
		for i, target := range subs {
			var x string
			if target.Ignored {
				x = ", ignored"
			} else if target.Admin {
				x = ", admin"
			}
			msg += "\n" + strconv.Itoa(i+1) + ": (" + target.API + ") " + target.Contact + x + " (" + strconv.Itoa(len(target.Subscriptions())) + " subscriptions)"
		}
		return msg
	}
	s, err := c.subs.GetSubscriber(text[1], APIiMessage)
	if err != nil {
		return "Subscriber does not exist: " + text[1]
	}
	subs := s.Subscriptions()
	if len(subs) == 0 {
		return text[1] + " has no subscriptions."
	}
	var x string
	if s.Ignored {
		x = " (ignored)"
	} else if s.Admin {
		x = " (admin)"
	}
	msg := s.Contact + x + " has " + strconv.Itoa(len(subs)) + " subscriptions:"
	i := 0
	for event, resume := range subs {
		i++
		msg += "\n" + strconv.Itoa(i) + ": " + event
		if resume.After(time.Now()) {
			msg += " (paused " + time.Until(resume).Round(time.Second).String() + ")"
		}
	}
	return msg
}

func (c *Config) iMessageAdminUnadmin(text []string) string {
	msg := "Usage: unadmin <contact> - Use 'admins' to see all admin subscribers."
	if len(text) != 2 {
		return msg
	}
	target, err := c.subs.GetSubscriber(text[1], APIiMessage)
	if err != nil {
		msg = "Subscriber does not exist: " + text[1]
		return msg
	}
	target.Admin = false
	msg = "Subscriber '" + target.Contact + "' updated without admin privileges."
	return msg
}

func (c *Config) iMessageAdminAdmin(text []string) string {
	msg := "Usage: admin <contact> - Use 'subs' to see all (non-ignored) subscribers."
	if len(text) != 2 {
		return msg
	}
	target, err := c.subs.GetSubscriber(text[1], APIiMessage)
	if err != nil {
		msg = "Subscriber does not exist: " + text[1]
		return msg
	}
	target.Admin = true
	msg = "Subscriber '" + target.Contact + "' updated with admin privileges."
	return msg
}

func (c *Config) iMessageAdminUnignore(text []string) string {
	msg := "Usage: unignore <contact> - Use 'ignores' to see all ignored subscribers."
	if len(text) != 2 {
		return msg
	}
	target, err := c.subs.GetSubscriber(text[1], APIiMessage)
	if err != nil {
		msg = "Subscriber does not exist: " + text[1]
		return msg
	}
	target.Ignored = false
	msg = "Subscriber '" + target.Contact + "' no longer ignored."
	return msg
}

func (c *Config) iMessageAdminIgnore(text []string) string {
	msg := "Usage: ignore <contact> - Use 'subs' to see all (non-ignored) subscribers."
	if len(text) != 2 {
		return msg
	}
	target, err := c.subs.GetSubscriber(text[1], APIiMessage)
	if err != nil {
		msg = "Subscriber does not exist: " + text[1]
		return msg
	}
	target.Ignored = true
	target.Admin = false
	msg = "Subscriber '" + target.Contact + "' ignored."
	return msg
}

func (c *Config) iMessageCams() string {
	c.Lock()
	defer c.Unlock()
	msg := "There are " + strconv.Itoa(len(c.Cameras)) + " cameras:\n"
	for cam := range c.Cameras {
		msg += c.Cameras[cam].Number + ": " + cam + "\n"
	}
	return msg
}

func (c *Config) iMessageEvents(text []string) string {
	events := c.subs.GetEvents()
	msg := "There are " + strconv.Itoa(len(events)) + " events:\n"
	i := 0
	for eventName, event := range events {
		i++
		description := "no description"
		if d, ok := event["description"]; ok {
			description = d
		}
		msg += strconv.Itoa(i) + ": " + eventName + " - " + description + "\n"
	}
	return msg
}

func (c *Config) iMessagePics(from string, id string, text []string) string {
	c.Lock()
	defer c.Unlock()
	msg := ""
	if len(text) > 1 {
		cam := strings.Join(text[1:], " ")
		if _, ok := c.Cameras[cam]; !ok {
			msg = "Unknown Camera: " + cam
			return msg
		}
		path := c.TempDir + "imessage_relay_" + id + "_" + cam + ".jpg"
		if err := c.GetPicture(id, cam, path); err != nil {
			log.Printf("[ERROR] [%v] GetPicture: %v", id, err)
			msg = "Error Getting '" + cam + "' Picture: " + err[0].Error()
		}
		c.msgs.Send(imessage.Outgoing{ID: id, To: from, Text: path, File: true, Call: c.pictureCallback})
		return msg
	}
	for cam := range c.Cameras {
		path := c.TempDir + "imessage_relay_" + id + "_" + cam + ".jpg"
		if err := c.GetPicture(id, cam, path); err != nil {
			log.Printf("[ERROR] [%v] GetPicture: %v", id, err)
			msg += "Error Getting '" + cam + "' Picture: " + err[0].Error() + "\n"
			continue
		}
		// Give the file system time to sync
		time.Sleep(150 * time.Millisecond)
		c.msgs.Send(imessage.Outgoing{ID: id, To: from, Text: path, File: true, Call: c.pictureCallback})
	}
	return msg
}

func (c *Config) iMessageSub(text []string, requestor *subscribe.Subscriber) string {
	kind := "event"
	msg := "Usage: sub <camera|event> - Use 'cams' and 'events' to see their names."
	if len(text) < 2 {
		return msg
	}
	event := strings.Join(text[1:], " ")
	if _, ok := c.subs.GetEvents()[event]; !ok {
		c.Lock()
		defer c.Unlock()
		kind = "camera"
		if _, ok := c.Cameras[event]; !ok {
			msg = "Event or Camera not found: " + event + "\n" + msg
			return msg
		}
	}
	msg = "You've been subscribed to " + kind + ": " + event
	if err := requestor.Subscribe(event); err != nil {
		msg = "You're already subscribed to: " + kind + ": " + event
	}
	msg += "\nYou have " + strconv.Itoa(len(requestor.Subscriptions())) + " event subscriptions."
	return msg
}

func (c *Config) iMessageSubs(text []string, requestor *subscribe.Subscriber) string {
	if requestor.Admin && len(text) > 1 {
		// admin asking for subs for someone else. handled by iMessageAdminSubs()
		return ""
	}
	msg := "Your Subscriptions:"
	i := 0
	for event, resume := range requestor.Subscriptions() {
		i++
		msg += "\n" + strconv.Itoa(i) + ": " + event
		if resume.After(time.Now()) {
			msg += " (paused " + time.Until(resume).Round(time.Second).String() + ")"
		}
	}
	if msg += "\n"; len(requestor.Subscriptions()) == 0 {
		msg += "(none)\n"
	}
	return msg
}

func (c *Config) iMessageUnsub(text []string, requestor *subscribe.Subscriber) string {
	msg := "Usage: unsub <camera|event|*> - Use 'subs' to see your subscriptions."
	if len(text) < 2 {
		return msg
	}
	event := strings.Join(text[1:], " ")
	msg = "You've been unsubscribed from: " + event
	if event == "*" {
		for e := range requestor.Subscriptions() {
			_ = requestor.UnSubscribe(e)
		}
		return "You've been unsubscribed all events."
	}
	if err := requestor.UnSubscribe(event); err != nil {
		msg = "You're not subscribed to: " + event
	}
	msg += "\nYou have " + strconv.Itoa(len(requestor.Subscriptions())) + " event subscriptions."
	return msg
}

func (c *Config) iMessageStop(text []string, requestor *subscribe.Subscriber) string {
	msg := "Usage: stop <minutes> [camera|event] - Use 'subs' to see your subscriptions."
	if len(text) < 2 {
		return msg
	}
	dur, err := strconv.Atoi(text[1])
	if err != nil {
		msg = "Unable to parse into a number: " + text[1] + "\n" + msg
		return msg
	}
	if len(text) > 2 {
		event := strings.Join(text[2:], " ")
		msg = "Notifications from '" + event + "' paused for at least " + text[1] + " minutes."
		if dur == 0 {
			msg = "Notifications from '" + event + " are no longer paused."
		}
		if err := requestor.Pause(event, time.Duration(dur)*time.Minute); err != nil {
			msg = "You're not subscribed to: " + event
		}
		return msg
	}
	for event := range requestor.Subscriptions() {
		_ = requestor.Pause(event, time.Duration(dur)*time.Minute)
	}
	msg = "Notifications paused for at least " + text[1] + " minutes."
	if dur == 0 {
		msg = "Notifications are no longer paused."
	}
	return msg
}
