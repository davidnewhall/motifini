package main

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/davidnewhall/motifini/messages"
	"github.com/davidnewhall/motifini/subscribe"
	"github.com/gorilla/mux"
)

// /api/v1.0/recv/imessage/msg/{from}
func (c *Config) recvMessageHandler(w http.ResponseWriter, r *http.Request) {
	c.export.httpVisits.Add(1)
	vars := mux.Vars(r)
	id, code, reply := ReqID(4), 500, "ERR0R\n"
	text := strings.Fields(r.FormValue("msg"))
	msg := messages.Msg{To: vars["from"], ID: id}
	if vars["from"] == "" || len(text) == 0 {
		c.finishReq(w, r, id, code, reply, msg, strings.Join(text, " "))
		return
	}
	sub, err := c.subs.GetSubscriber(vars["from"], APIiMessage)
	if err != nil {
		// Every account we receive a message from gets logged as a subscriber with no subscriptions.
		sub = c.subs.CreateSub(vars["from"], APIiMessage, len(c.subs.GetAdmins()) == 0, false)
	}

	if !sub.IsIgnored() {
		code = 200
		switch strings.ToLower(text[0]) {
		case "cams":
			reply, msg.Text = c.iMessageCams(id, sub)
		case "events":
			reply, msg.Text = c.iMessageEvents(id, text, sub)
		case "pics":
			code, reply, msg.Text = c.iMessagePics(vars["from"], id, text, sub)
		case "sub":
			reply, msg.Text = c.iMessageSub(id, text, sub)
		case "subs":
			reply, msg.Text = c.iMessageSubs(id, text, sub)
		case "unsub":
			reply, msg.Text = c.iMessageUnsub(id, text, sub)
		case "stop":
			reply, msg.Text = c.iMessageStop(id, text, sub)
		case "help":
			reply, msg.Text = c.iMessageHelp(id, sub)
		default:
			code = 404
		}
	}
	if sub.IsAdmin() {
		code = 200
		switch strings.ToLower(text[0]) {
		case "ignores":
			reply, msg.Text = c.iMessageAdminIgnores(id, sub)
		case "ignore":
			reply, msg.Text = c.iMessageAdminIgnore(id, text, sub)
		case "unignore":
			reply, msg.Text = c.iMessageAdminUnignore(id, text, sub)
		case "admins":
			reply, msg.Text = c.iMessageAdminAdmins(id, sub)
		case "admin":
			reply, msg.Text = c.iMessageAdminAdmin(id, text, sub)
		case "unadmin":
			reply, msg.Text = c.iMessageAdminUnadmin(id, text, sub)
		case "subs":
			msg.Text += c.iMessageAdminSubs(id, text, sub)
		case "help":
			msg.Text += c.iMessageAdminHelp(id, sub)
		default:
			code = 404
		}
	}
	c.finishReq(w, r, id, code, reply, msg, strings.Join(text, " "))
}

func (c *Config) iMessageHelp(id string, sub subscribe.SubInterface) (string, string) {
	reply := "REQ ID: " + id + ", msg: got help\n"
	msg := `- iMessageSpy Help -
Available User Commands:
cams - Displays all available cameras by name.
events - Displays all available events.
pics [camera] - Sends pictures from all cameras, or from [camera].
sub <camera|event> - Enables notifications from <camera> or <event>.
unsub <camera|event|*> - Stops notifications from <camera>, lor <event> or everything if '*' is passed.
stop [minutes] [camera] - Stops all motion notifications for 10 minutes or [minutes] on all cameras or [camera]
subs - Shows your subscriptions.`
	return reply, msg
}

func (c *Config) iMessageAdminHelp(id string, sub subscribe.SubInterface) string {
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

func (c *Config) iMessageAdminAdmins(id string, sub subscribe.SubInterface) (string, string) {
	reply := "REQ ID: " + id + ", msg: got admins\n"
	admins := c.subs.GetAdmins()
	msg := "There are " + strconv.Itoa(len(admins)) + " admins:"
	for i, admin := range admins {
		msg += "\n" + strconv.Itoa(i+1) + ": (" + admin.GetAPI() + ") " + admin.GetContact() + " (" + strconv.Itoa(len(admin.Subscriptions())) + " subscriptions)"
	}
	return reply, msg
}

func (c *Config) iMessageAdminIgnores(id string, sub subscribe.SubInterface) (string, string) {
	ignores := c.subs.GetIgnored()
	msg := "There are " + strconv.Itoa(len(ignores)) + " ignored subscribers:"
	for i, ignore := range ignores {
		msg += "\n" + strconv.Itoa(i+1) + ": (" + ignore.GetAPI() + ") " + ignore.GetContact() + " (" + strconv.Itoa(len(ignore.Subscriptions())) + " subscriptions)"
	}
	return "REQ ID: " + id + ", msg: got admins\n", msg
}

func (c *Config) iMessageAdminSubs(id string, text []string, sub subscribe.SubInterface) string {
	if len(text) == 1 {
		subs := c.subs.GetAllSubscribers()
		msg := "There are " + strconv.Itoa(len(subs)) + " total subscribers:"
		for i, target := range subs {
			var x string
			if target.IsIgnored() {
				x = ", ignored"
			} else if target.IsAdmin() {
				x = ", admin"
			}
			msg += "\n" + strconv.Itoa(i+1) + ": (" + target.GetAPI() + ") " + target.GetContact() + x + " (" + strconv.Itoa(len(target.Subscriptions())) + " subscriptions)"
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
	if sub.IsIgnored() {
		x = " (ignored)"
	} else if sub.IsAdmin() {
		x = " (admin)"
	}
	msg := sub.GetContact() + x + " has " + strconv.Itoa(len(subs)) + " subscriptions:"
	i := 0
	for event, resume := range subs {
		i++
		msg += "\n" + strconv.Itoa(i) + ": " + event
		if resume.After(time.Now()) {
			msg += " (paused " + resume.Sub(time.Now()).Round(time.Second).String() + ")"
		}
	}
	return msg
}

func (c *Config) iMessageAdminUnadmin(id string, text []string, sub subscribe.SubInterface) (string, string) {
	reply := "REQ ID: " + id + ", msg: got unadmin\n"
	msg := "Usage: unadmin <contact> - Use 'admins' to see all admin subscribers."
	if len(text) != 2 {
		return reply, msg
	}
	target, err := c.subs.GetSubscriber(text[1], APIiMessage)
	if err != nil {
		msg = "Subscriber does not exist: " + text[1]
		return reply, msg
	}
	target.Unadmin()
	msg = "Subscriber '" + target.GetContact() + "' updated without admin privileges."
	return reply, msg
}

func (c *Config) iMessageAdminAdmin(id string, text []string, sub subscribe.SubInterface) (string, string) {
	reply := "REQ ID: " + id + ", msg: got admin\n"
	msg := "Usage: admin <contact> - Use 'subs' to see all (non-ignored) subscribers."
	if len(text) != 2 {
		return reply, msg
	}
	target, err := c.subs.GetSubscriber(text[1], APIiMessage)
	if err != nil {
		msg = "Subscriber does not exist: " + text[1]
		return reply, msg
	}
	target.MakeAdmin()
	msg = "Subscriber '" + target.GetContact() + "' updated with admin privileges."
	return reply, msg
}

func (c *Config) iMessageAdminUnignore(id string, text []string, sub subscribe.SubInterface) (string, string) {
	reply := "REQ ID: " + id + ", msg: got unignore\n"
	msg := "Usage: unignore <contact> - Use 'ignores' to see all ignored subscribers."
	if len(text) != 2 {
		return reply, msg
	}
	target, err := c.subs.GetSubscriber(text[1], APIiMessage)
	if err != nil {
		msg = "Subscriber does not exist: " + text[1]
		return reply, msg
	}
	target.Ignore()
	msg = "Subscriber '" + target.GetContact() + "' no longer ignored."
	return reply, msg
}

func (c *Config) iMessageAdminIgnore(id string, text []string, sub subscribe.SubInterface) (string, string) {
	reply := "REQ ID: " + id + ", msg: got ignore\n"
	msg := "Usage: ignore <contact> - Use 'subs' to see all (non-ignored) subscribers."
	if len(text) != 2 {
		return reply, msg
	}
	target, err := c.subs.GetSubscriber(text[1], APIiMessage)
	if err != nil {
		msg = "Subscriber does not exist: " + text[1]
		return reply, msg
	}
	target.Ignore()
	target.Unadmin()
	msg = "Subscriber '" + target.GetContact() + "' ignored."
	return reply, msg
}

func (c *Config) iMessageCams(id string, sub subscribe.SubInterface) (string, string) {
	c.Lock()
	defer c.Unlock()
	reply := "REQ ID: " + id + ", msg: got cams\n"
	msg := "There are " + strconv.Itoa(len(c.Cameras)) + " cameras:\n"
	for cam := range c.Cameras {
		msg += c.Cameras[cam].Number + ": " + cam + "\n"
	}
	return reply, msg
}

func (c *Config) iMessageEvents(id string, text []string, sub subscribe.SubInterface) (string, string) {
	reply := "REQ ID: " + id + ", msg: got events\n"
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
	return reply, msg
}

func (c *Config) iMessagePics(from string, id string, text []string, sub subscribe.SubInterface) (int, string, string) {
	c.Lock()
	defer c.Unlock()
	code, reply, msg := 200, "REQ ID: "+id+", msg: got pics\n", ""
	if len(text) > 1 {
		cam := strings.Join(text[1:], " ")
		if _, ok := c.Cameras[cam]; !ok {
			msg = "Unknown Camera: " + cam
			return code, reply, msg
		}
		path := c.TempDir + "imessage_relay_" + id + "_" + cam + ".jpg"
		if err := c.GetPicture(id, cam, path); err != nil {
			log.Printf("[ERROR] [%v] GetPicture: %v", id, err)
			code, reply = 500, "ERROR: "+err.Error()
			msg = "Error Getting '" + cam + "' Picture: " + err.Error()
		}
		c.msgs.Send(messages.Msg{ID: id, To: from, Text: path, File: true, Call: c.pictureCallback})
		return code, reply, msg
	}
	for cam := range c.Cameras {
		path := c.TempDir + "imessage_relay_" + id + "_" + cam + ".jpg"
		if err := c.GetPicture(id, cam, path); err != nil {
			log.Printf("[ERROR] [%v] GetPicture: %v", id, err)
			msg += "Error Getting '" + cam + "' Picture: " + err.Error() + "\n"
			continue
		}
		// Give the file system time to sync
		time.Sleep(150 * time.Millisecond)
		c.msgs.Send(messages.Msg{ID: id, To: from, Text: path, File: true, Call: c.pictureCallback})
	}
	return code, reply, msg
}

func (c *Config) iMessageSub(id string, text []string, sub subscribe.SubInterface) (string, string) {
	reply := "REQ ID: " + id + ", msg: got sub\n"
	kind := "event"
	msg := "Usage: sub <camera|event> - Use 'cams' and 'events' to see their names."
	if len(text) < 2 {
		return reply, msg
	}
	event := strings.Join(text[1:], " ")
	if _, ok := c.subs.GetEvents()[event]; !ok {
		c.Lock()
		defer c.Unlock()
		kind = "camera"
		if _, ok := c.Cameras[event]; !ok {
			msg = "Event or Camera not found: " + event + "\n" + msg
			return reply, msg
		}
	}
	msg = "You've been subscribed to " + kind + ": " + event
	if err := sub.Subscribe(event); err != nil {
		msg = "You're already subscribed to: " + kind + ": " + event
	}
	msg += "\nYou have " + strconv.Itoa(len(sub.Subscriptions())) + " event subscriptions."
	return reply, msg
}

func (c *Config) iMessageSubs(id string, text []string, sub subscribe.SubInterface) (string, string) {
	reply := "REQ ID: " + id + ", msg: got subs\n"
	if sub.IsAdmin() && len(text) > 1 {
		// admin asking for subs for someone else.
		return reply, ""
	}
	msg := "Your Subscriptions:"
	i := 0
	for event, resume := range sub.Subscriptions() {
		i++
		msg += "\n" + strconv.Itoa(i) + ": " + event
		if resume.After(time.Now()) {
			msg += " (paused " + resume.Sub(time.Now()).Round(time.Second).String() + ")"
		}
	}
	if msg += "\n"; len(sub.Subscriptions()) == 0 {
		msg += "(none)\n"
	}
	return reply, msg
}

func (c *Config) iMessageUnsub(id string, text []string, sub subscribe.SubInterface) (string, string) {
	reply := "REQ ID: " + id + ", msg: got unsub\n"
	msg := "Usage: unsub <camera|event|*> - Use 'subs' to see your subscriptions."
	if len(text) < 2 {
		return reply, msg
	}
	event := strings.Join(text[1:], " ")
	msg = "You've been unsubscribed from: " + event
	if event == "*" {
		for e := range sub.Subscriptions() {
			_ = sub.UnSubscribe(e)
		}
		return reply, "You've been unsubscribed all events."
	}
	if err := sub.UnSubscribe(event); err != nil {
		msg = "You're not subscribed to: " + event
	}
	msg += "\nYou have " + strconv.Itoa(len(sub.Subscriptions())) + " event subscriptions."
	return reply, msg
}

func (c *Config) iMessageStop(id string, text []string, sub subscribe.SubInterface) (string, string) {
	reply := "REQ ID: " + id + ", msg: got stop\n"
	msg := "Usage: stop <minutes> [camera|event] - Use 'subs' to see your subscriptions."
	if len(text) < 2 {
		return reply, msg
	}
	dur, err := strconv.Atoi(text[1])
	if err != nil {
		msg = "Unable to parse into a number: " + text[1] + "\n" + msg
		return reply, msg
	}
	if len(text) > 2 {
		event := strings.Join(text[2:], " ")
		msg = "Notifications from '" + event + "' paused for at least " + text[1] + " minutes."
		if dur == 0 {
			msg = "Notifications from '" + event + " are no longer paused."
		}
		if err := sub.Pause(event, time.Duration(dur)*time.Minute); err != nil {
			msg = "You're not subscribed to: " + event
		}
		return reply, msg
	}
	for event := range sub.Subscriptions() {
		_ = sub.Pause(event, time.Duration(dur)*time.Minute)
	}
	msg = "Notifications paused for at least " + text[1] + " minutes."
	if dur == 0 {
		msg = "Notifications are no longer paused."
	}
	return reply, msg
}
