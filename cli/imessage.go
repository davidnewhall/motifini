package cli

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/davidnewhall/motifini/chat"
	"golift.io/imessage"
	"golift.io/subscribe"
)

func (m *Motifini) startiMessage() error {
	var err error
	m.Imsg, err = imessage.Init(&imessage.Config{
		SQLPath:   strings.Replace(m.Config.Imessage.DBPath, "~", os.Getenv("HOME"), 1),
		QueueSize: m.Config.Imessage.QueueSize,
		ClearMsgs: m.Config.Imessage.ClearMessages,
		Retries:   m.Config.Imessage.Retries,
		Interval:  m.Config.Imessage.Interval,
		ErrorLog:  &Log{Affix: "[ERROR] "},
		DebugLog:  &Log{Affix: "[DEBUG] ", Muted: !m.Flags.Debug},
	})
	if err != nil {
		return err
	}
	// Listen to all incoming imessages, pass them to our handler.
	m.Imsg.IncomingCall(".*", m.recviMessageHandler)
	return m.Imsg.Start()
}

// recviMessageHandler is a callback binding from the imessage library.
func (m *Motifini) recviMessageHandler(msg imessage.Incoming) {
	sub, err := m.Subs.GetSubscriber(msg.From, APIiMessage)
	if err != nil {
		// Every account we receive a message from gets logged as a subscriber with no subscriptions.
		sub = m.Subs.CreateSub(msg.From, APIiMessage, len(m.Subs.GetAdmins()) == 0, false)
	}
	// Pass the message off to the chat command handler routines.
	h := &chat.CommandHandler{
		API:  APIiMessage,
		ID:   ReqID(4),
		Sub:  sub,
		Text: strings.Fields(msg.Text),
		From: msg.From,
	}
	resp := m.Chat.HandleCommand(h)
	// Send the reply as files and/or text.
	if resp.Reply != "" {
		m.Imsg.Send(imessage.Outgoing{To: msg.From, ID: h.ID, Text: resp.Reply})
	}
	for _, path := range resp.Files {
		m.Imsg.Send(imessage.Outgoing{To: msg.From, ID: h.ID, Text: path, File: true, Call: m.fileCallback})
	}
}

func (m *Motifini) sendFileOrMsg(id, msg, path string, subs []*subscribe.Subscriber) {
	for _, sub := range subs {
		switch sub.API {
		case APIiMessage:
			if path != "" {
				m.Imsg.Send(imessage.Outgoing{ID: id, To: sub.Contact, Text: path, File: true, Call: m.fileCallback})
			}
			if msg != "" {
				m.Imsg.Send(imessage.Outgoing{ID: id, To: sub.Contact, Text: msg})
			}
		default:
			log.Printf("[%v] Unknown Notification API '%v' for contact: %v", id, sub.API, sub.Contact)
		}
	}
}

// fileCallback runs in a go routine after a video or picture iMessage is processed.
func (m *Motifini) fileCallback(msg *imessage.Response) {
	var size int64
	if fi, errStat := os.Stat(msg.Text); errStat == nil {
		size = fi.Size()
	}
	if msg.Errs != nil {
		m.exports.errors.Add(1)
		log.Printf("[ERROR] [%v] m.Msgs.Send '%v': %v", msg.ID, msg.To, msg.Errs)
	} else {
		m.exports.videos.Add(1)
		log.Printf("[REPLY] [%v] File '%v' (%.2fMb) sent to: %v", msg.ID, msg.Text, float32(size)/1024/1024, msg.To)
	}
	// Might take a while to upload.
	time.Sleep(30 * time.Second)
	if err := os.Remove(msg.Text); err != nil && !os.IsNotExist(err) {
		log.Printf("[ERROR] [%v] Remove(path): %v", msg.ID, err)
		return
	}
	m.Debug.Printf("[%v] Deleted: %v", msg.ID, msg.Text)
}
