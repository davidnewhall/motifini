package cli

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"golift.io/imessage"
	"golift.io/securityspy"
	"golift.io/subscribe"
)

// recvMessageHandler is a callback binding from the imessage library.
func (m *Motifini) recvMessageHandler(msg imessage.Incoming) {
	id := ReqID(4)
	text := strings.Fields(msg.Text)
	reply := imessage.Outgoing{To: msg.From, ID: id}

	requestor, err := m.Subs.GetSubscriber(msg.From, APIiMessage)
	if err != nil {
		// Every account we receive a message from gets logged as a subscriber with no subscriptions.
		requestor = m.Subs.CreateSub(msg.From, APIiMessage, len(m.Subs.GetAdmins()) == 0, false)
	}

	if !requestor.Ignored {
		switch strings.ToLower(text[0]) {
		case "cams":
			reply.Text = m.iMessageCams()
		case "events":
			reply.Text = m.iMessageEvents()
		case "pics":
			reply.Text = m.iMessagePics(msg.From, id, text)
		case "sub":
			reply.Text = m.iMessageSub(text, requestor)
		case "subs":
			reply.Text = m.iMessageSubs(text, requestor)
		case "unsub":
			reply.Text = m.iMessageUnsub(text, requestor)
		case "stop":
			reply.Text = m.iMessageStop(text, requestor)
		case "help":
			reply.Text = m.iMessageHelp()
		}
	}
	if requestor.Admin {
		reply.Text += m.handleAdminCmds(text)
	}
	if reply.Text != "" {
		m.Msgs.Send(reply)
	}
}

func (m *Motifini) handleAdminCmds(text []string) string {
	switch strings.ToLower(text[0]) {
	case "ignores":
		return m.iMessageAdminIgnores()
	case "ignore":
		return m.iMessageAdminIgnore(text)
	case "unignore":
		return m.iMessageAdminUnignore(text)
	case "admins":
		return m.iMessageAdminAdmins()
	case "admin":
		return m.iMessageAdminAdmin(text)
	case "unadmin":
		return m.iMessageAdminUnadmin(text)
	case "subs":
		return m.iMessageAdminSubs(text)
	case "help":
		return m.iMessageAdminHelp()
	default:
		return ""
	}
}

func (m *Motifini) iMessageHelp() string {
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

func (m *Motifini) iMessageAdminHelp() string {
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

func (m *Motifini) iMessageAdminAdmins() string {
	admins := m.Subs.GetAdmins()
	msg := "There are " + strconv.Itoa(len(admins)) + " admins:"
	for i, admin := range admins {
		msg += fmt.Sprintf("\n%v: (%v) %v (%v subscriptions)",
			strconv.Itoa(i+1), admin.API, admin.Contact, len(admin.Subscriptions()))
	}
	return msg
}

func (m *Motifini) iMessageAdminIgnores() string {
	ignores := m.Subs.GetIgnored()
	msg := "There are " + strconv.Itoa(len(ignores)) + " ignored subscribers:"
	for i, ignore := range ignores {
		msg += fmt.Sprintf("\n%v: (%v) %v (%v subscriptions)",
			strconv.Itoa(i+1), ignore.API, ignore.Contact, len(ignore.Subscriptions()))
	}
	return msg
}

func (m *Motifini) iMessageAdminSubs(text []string) string {
	if len(text) == 1 {
		subs := m.Subs.Subscribers
		msg := "There are " + strconv.Itoa(len(subs)) + " total subscribers:"
		for i, target := range subs {
			var x string
			if target.Ignored {
				x = ", ignored"
			} else if target.Admin {
				x = ", admin"
			}
			msg += fmt.Sprintf("\n%v: (%v) %v%v (%v subscriptions)",
				strconv.Itoa(i+1), target.API, target.Contact, x, len(target.Subscriptions()))
		}
		return msg
	}
	s, err := m.Subs.GetSubscriber(text[1], APIiMessage)
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

func (m *Motifini) iMessageAdminUnadmin(text []string) string {
	msg := "Usage: unadmin <contact> - Use 'admins' to see all admin subscribers."
	if len(text) != 2 {
		return msg
	}
	target, err := m.Subs.GetSubscriber(text[1], APIiMessage)
	if err != nil {
		msg = "Subscriber does not exist: " + text[1]
		return msg
	}
	target.Admin = false
	msg = "Subscriber '" + target.Contact + "' updated without admin privileges."
	return msg
}

func (m *Motifini) iMessageAdminAdmin(text []string) string {
	msg := "Usage: admin <contact> - Use 'subs' to see all (non-ignored) subscribers."
	if len(text) != 2 {
		return msg
	}
	target, err := m.Subs.GetSubscriber(text[1], APIiMessage)
	if err != nil {
		msg = "Subscriber does not exist: " + text[1]
		return msg
	}
	target.Admin = true
	msg = "Subscriber '" + target.Contact + "' updated with admin privileges."
	return msg
}

func (m *Motifini) iMessageAdminUnignore(text []string) string {
	msg := "Usage: unignore <contact> - Use 'ignores' to see all ignored subscribers."
	if len(text) != 2 {
		return msg
	}
	target, err := m.Subs.GetSubscriber(text[1], APIiMessage)
	if err != nil {
		msg = "Subscriber does not exist: " + text[1]
		return msg
	}
	target.Ignored = false
	msg = "Subscriber '" + target.Contact + "' no longer ignored."
	return msg
}

func (m *Motifini) iMessageAdminIgnore(text []string) string {
	msg := "Usage: ignore <contact> - Use 'subs' to see all (non-ignored) subscribers."
	if len(text) != 2 {
		return msg
	}
	target, err := m.Subs.GetSubscriber(text[1], APIiMessage)
	if err != nil {
		msg = "Subscriber does not exist: " + text[1]
		return msg
	}
	target.Ignored = true
	target.Admin = false
	msg = "Subscriber '" + target.Contact + "' ignored."
	return msg
}

func (m *Motifini) iMessageCams() string {
	msg := "There are " + strconv.Itoa(len(m.Spy.Cameras.All())) + " cameras:\n"
	for _, cam := range m.Spy.Cameras.All() {
		msg += fmt.Sprintf("%v: %v\n", cam.Number, cam.Name)
	}
	return msg
}

func (m *Motifini) iMessageEvents() string {
	events := m.Subs.GetEvents()
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

func (m *Motifini) iMessagePics(from string, id string, text []string) string {
	msg := ""
	if len(text) > 1 {
		name := strings.Join(text[1:], " ")
		cam := m.Spy.Cameras.ByName(name)
		if cam == nil {
			msg = "Unknown Camera: " + name
			return msg
		}
		path := m.Config.Global.TempDir + "imessage_relay_" + id + "_" + name + ".jpg"
		if err := cam.SaveJPEG(&securityspy.VidOps{}, path); err != nil {
			log.Printf("[ERROR] [%v] am.SaveJPEG: %v", id, err)
			msg = "Error Getting '" + name + "' Picture: " + err.Error()
		}
		m.Msgs.Send(imessage.Outgoing{ID: id, To: from, Text: path, File: true, Call: m.pictureCallback})
		return msg
	}
	for _, cam := range m.Spy.Cameras.All() {
		path := fmt.Sprintf("%vimessage_relay_%v_%v.jpg", m.Config.Global.TempDir, cam.Number, cam.Name)
		if err := cam.SaveJPEG(&securityspy.VidOps{}, path); err != nil {
			log.Printf("[ERROR] [%v] cam.SaveJPEG: %v", id, err)
			msg += "Error Getting '" + cam.Name + "' Picture: " + err.Error() + "\n"
			continue
		}
		// Give the file system time to sync
		time.Sleep(150 * time.Millisecond)
		m.Msgs.Send(imessage.Outgoing{ID: id, To: from, Text: path, File: true, Call: m.pictureCallback})
	}
	return msg
}

func (m *Motifini) iMessageSub(text []string, requestor *subscribe.Subscriber) string {
	kind := "event"
	msg := "Usage: sub <camera|event> - Use 'cams' and 'events' to see their names."
	if len(text) < 2 {
		return msg
	}
	event := strings.Join(text[1:], " ")
	if _, ok := m.Subs.GetEvents()[event]; !ok {
		kind = "camera"
		if cam := m.Spy.Cameras.ByName(event); cam == nil {
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

func (m *Motifini) iMessageSubs(text []string, requestor *subscribe.Subscriber) string {
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

func (m *Motifini) iMessageUnsub(text []string, requestor *subscribe.Subscriber) string {
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

func (m *Motifini) iMessageStop(text []string, requestor *subscribe.Subscriber) string {
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
