package messenger

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/davidnewhall/motifini/pkg/chat"
	"github.com/davidnewhall/motifini/pkg/export"
	"golift.io/imessage"
)

const (
	IDLength   = 4
	mebibyte   = 1024 * 1024
	uploadWait = 20 * time.Second
)

func (m *Messenger) startiMessage() error {
	var err error

	m.Conf.SQLPath = strings.Replace(m.Conf.SQLPath, "~", os.Getenv("HOME"), 1)
	m.Conf.ErrorLog = m.Error
	m.Conf.DebugLog = m.Debug

	m.imsg, err = imessage.Init(m.Conf)
	if err != nil {
		return fmt.Errorf("initializing imessage pkg: %w", err)
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
		ID:   ReqID(IDLength),
		Sub:  sub,
		Text: strings.Fields(msg.Text),
		From: msg.From,
	}

	m.Info.Printf("[%s] iMessage Received from %s (admin:%v, ignored:%v), size: %d, cmd: %s",
		h.ID, msg.From, sub.Admin, sub.Ignored, len(msg.Text), h.Text[0])

	resp := m.chat.HandleCommand(h)
	// Send the reply as files and/or text.
	if resp.Reply != "" {
		m.SendiMessage(imessage.Outgoing{To: msg.From, ID: h.ID, Text: resp.Reply})
	}

	for _, path := range resp.Files {
		m.SendiMessage(imessage.Outgoing{To: msg.From, ID: h.ID, Text: path, File: true})
	}
}

// SendiMessage is how we send out a message or file via iMessage.
// Use this wrapper so the internal counters are updated, and callbacks used.
func (m *Messenger) SendiMessage(msg imessage.Outgoing) {
	if msg.File {
		m.Info.Printf("[%s] iMessage sending file to %s, file: %s", msg.ID, msg.To, msg.Text)
		export.Map.Files.Add(1)

		if msg.Call == nil {
			msg.Call = m.fileCallback
		}
	} else {
		m.Info.Printf("[%s] iMessage sending msg to %s, size: %d", msg.ID, msg.To, len(msg.Text))
		export.Map.Sent.Add(1)

		if msg.Call == nil {
			msg.Call = m.msgCallback
		}
	}

	m.imsg.Send(msg)
}

// msgCallback is used as a generic callback function for messages. It just writes logs.
func (m *Messenger) msgCallback(msg *imessage.Response) {
	if msg.Errs != nil {
		export.Map.Errors.Add(1)
		m.Error.Printf("[%v] m.Msgs.Send '%v': sent: %v, %d errs: %v", msg.ID, msg.To, msg.Sent, len(msg.Errs), msg.Errs)
	}

	if !msg.Sent {
		return
	}

	m.Info.Printf("[%v] iMessage Reply SENT to %s, size: %d", msg.ID, msg.To, len(msg.Text))
}

// fileCallback runs in a go routine after a video or picture iMessage is processed.
func (m *Messenger) fileCallback(msg *imessage.Response) {
	if msg.Errs != nil {
		export.Map.Errors.Add(1)
		m.Error.Printf("[%v] m.Msgs.Send '%v': sent: %v, %d errs: %v", msg.ID, msg.To, msg.Sent, len(msg.Errs), msg.Errs)
	}

	if msg.Sent {
		var size int64

		if fi, err := os.Stat(msg.Text); err == nil {
			size = fi.Size()
		}

		m.Info.Printf("[%v] iMessage File '%v' (%.2fMb) SENT to: %v", msg.ID, msg.Text, float32(size)/mebibyte, msg.To)
	}

	if !strings.HasPrefix(msg.Text, m.TempDir) {
		// Only delete files in tempdir.
		return
	}

	// Might take a while to upload.
	time.Sleep(uploadWait)

	if err := os.Remove(msg.Text); err != nil && !os.IsNotExist(err) {
		m.Error.Printf("[%v] Remove(path): %v", msg.ID, err)
		return
	}

	m.Info.Printf("[%v] Deleted: %v", msg.ID, msg.Text)
}
