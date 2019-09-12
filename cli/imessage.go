package cli

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golift.io/imessage"
	"golift.io/subscribe"
)

func (m *Motifini) startiMessage() error {
	var err error
	m.Msgs, err = imessage.Init(&imessage.Config{
		SQLPath:   strings.Replace(m.Config.Imessage.DBPath, "~", os.Getenv("HOME"), 1),
		QueueSize: m.Config.Imessage.QueueSize,
		ClearMsgs: m.Config.Imessage.ClearMessages,
		Retries:   m.Config.Imessage.Retries,
		Interval:  m.Config.Imessage.Interval,
		ErrorLog:  &Log{Affix: "[ERROR] "},
		DebugLog:  &Log{Affix: "[DEBUG] ", Muted: !m.Flags.Debug},
	})
	if err != nil {
		return errors.Wrap(err, "initializing imessage")
	}
	// Listen to all incoming imessages, pass them to our handler.
	m.Msgs.IncomingCall(".*", m.recvMessageHandler)
	return errors.Wrap(m.Msgs.Start(), "starting imessage")
}

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
			defer m.save()
			reply.Text = m.iMessageSub(text, requestor)
		case "subs":
			reply.Text = m.iMessageSubs(text, requestor)
		case "unsub":
			defer m.save()
			reply.Text = m.iMessageUnsub(text, requestor)
		case "stop":
			defer m.save()
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
		defer m.save()
		return m.iMessageAdminIgnore(text)
	case "unignore":
		defer m.save()
		return m.iMessageAdminUnignore(text)
	case "admins":
		return m.iMessageAdminAdmins()
	case "admin":
		defer m.save()
		return m.iMessageAdminAdmin(text)
	case "unadmin":
		defer m.save()
		return m.iMessageAdminUnadmin(text)
	case "subs":
		return m.iMessageAdminSubs(text)
	case "help":
		return m.iMessageAdminHelp()
	default:
		return ""
	}
}

func (m *Motifini) sendFileOrMsg(id, msg, path string, subs []*subscribe.Subscriber) {
	for _, sub := range subs {
		switch sub.API {
		case APIiMessage:
			if path != "" {
				m.Msgs.Send(imessage.Outgoing{ID: id, To: sub.Contact, Text: path, File: true, Call: m.pictureCallback})
			}
			if msg != "" {
				m.Msgs.Send(imessage.Outgoing{ID: id, To: sub.Contact, Text: msg})
			}
		default:
			log.Printf("[%v] Unknown Notification API '%v' for contact: %v", id, sub.API, sub.Contact)
		}
	}
}

// videoCallback runs in a go routine after a video iMessage is processed.
func (m *Motifini) videoCallback(msg *imessage.Response) {
	var size int64
	if fi, errStat := os.Stat(msg.Text); errStat == nil {
		size = fi.Size()
	}
	if msg.Errs != nil {
		m.exports.errors.Add(1)
		log.Printf("[ERROR] [%v] msgs.Msgs.Send '%v': %v", msg.ID, msg.To, msg.Errs)
	} else {
		m.exports.videos.Add(1)
		log.Printf("[REPLY] [%v] Video '%v' (%.2fMb) sent to: %v", msg.ID, msg.Text, float32(size)/1024/1024, msg.To)
	}
	// Might take a while to upload.
	time.Sleep(20 * time.Minute)
	if err := os.Remove(msg.Text); err != nil && !os.IsNotExist(err) {
		log.Printf("[ERROR] [%v] Remove(path): %v", msg.ID, err)
		return
	}
	m.Debug.Printf("[%v] Deleted: %v", msg.ID, msg.Text)
}

// pictureCallback runs in a go routine after an iMessage is processed.
// Possibly more than once...
func (m *Motifini) pictureCallback(msg *imessage.Response) {
	if msg.Errs != nil {
		m.exports.errors.Add(1)
		log.Printf("[ERROR] [%v] msgs.Msgs.Send '%v': %v", msg.ID, msg.To, msg.Errs)
	} else {
		m.exports.pics.Add(1)
		log.Printf("[REPLY] [%v] Picture '%v' sent to: %v", msg.ID, msg.Text, msg.To)
	}
	// Might take a while to upload.
	time.Sleep(5 * time.Second)
	if err := os.Remove(msg.Text); err != nil && !os.IsNotExist(err) {
		log.Printf("[ERROR] [%v] Remove(path): %v", msg.ID, err)
	} else if err == nil {
		m.Debug.Printf("[%v] Deleted: %v", msg.ID, msg.Text)
	}
}
