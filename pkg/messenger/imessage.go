package messenger

import (
	"os"
	"strings"
	"time"

	"github.com/davidnewhall/motifini/pkg/chat"
	"github.com/davidnewhall/motifini/pkg/export"
	"golift.io/imessage"
)

func (m *Messenger) startiMessage() error {
	var err error
	m.Conf.SQLPath = strings.Replace(m.Conf.SQLPath, "~", os.Getenv("HOME"), 1)
	m.Conf.ErrorLog = m.Error
	m.Conf.DebugLog = m.Debug
	m.imsg, err = imessage.Init(m.Conf)
	if err != nil {
		return err
	}
	// Listen to all incoming imessages, pass them to our handler.
	m.imsg.IncomingCall(".*", m.recviMessageHandler)
	return m.imsg.Start()
}

// recviMessageHandler is a callback binding from the imessage library.
func (m *Messenger) recviMessageHandler(msg imessage.Incoming) {
	export.Map.Recv.Add(1)
	sub, err := m.Subs.GetSubscriber(msg.From, APIiMessage)
	if err != nil {
		// Every account we receive a message from gets logged as a subscriber with no subscriptions.
		sub = m.Subs.CreateSub(msg.From, APIiMessage, len(m.Subs.GetAdmins()) == 0, false)
	}
	// Pass the message off to the chat command handler routines.
	h := &chat.Handler{
		API:  APIiMessage,
		ID:   ReqID(4),
		Sub:  sub,
		Text: strings.Fields(msg.Text),
		From: msg.From,
	}
	m.Info.Printf("[%s] iMessage Received from %s (admin:%v, ignored:%v), size: %d, cmd: %s",
		h.ID, msg.From, sub.Admin, sub.Ignored, len(msg.Text), h.Text[0])
	resp := m.chat.HandleCommand(h)
	// Send the reply as files and/or text.
	if resp.Reply != "" {
		m.Info.Printf("[%s] iMessage Reply to %s, size: %d", h.ID, msg.From, len(resp.Reply))
		m.SendiMessage(imessage.Outgoing{To: msg.From, ID: h.ID, Text: resp.Reply})
	}
	for _, path := range resp.Files {
		m.SendiMessage(imessage.Outgoing{To: msg.From, ID: h.ID, Text: path, File: true, Call: m.FileCallback})
	}
}

func (m *Messenger) SendiMessage(msg imessage.Outgoing) {
	if msg.File {
		export.Map.Files.Add(1)
	} else {
		export.Map.Sent.Add(1)
	}
	m.imsg.Send(msg)
}

// FileCallback runs in a go routine after a video or picture iMessage is processed.
func (m *Messenger) FileCallback(msg *imessage.Response) {
	var size int64
	if fi, err := os.Stat(msg.Text); err == nil {
		size = fi.Size()
	}
	if msg.Errs != nil {
		export.Map.Errors.Add(1)
		m.Error.Printf("[%v] m.Msgs.Send '%v': %v", msg.ID, msg.To, msg.Errs)
	} else {
		m.Info.Printf("[%v] iMessage File '%v' (%.2fMb) sent to: %v", msg.ID, msg.Text, float32(size)/1024/1024, msg.To)
	}
	// Might take a while to upload.
	time.Sleep(20 * time.Second)
	if err := os.Remove(msg.Text); err != nil && !os.IsNotExist(err) {
		m.Error.Printf("[%v] Remove(path): %v", msg.ID, err)
		return
	}
	m.Debug.Printf("[%v] Deleted: %v", msg.ID, msg.Text)
}
