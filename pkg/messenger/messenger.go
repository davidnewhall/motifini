// Package messenger disambiguates messenger protocols. Currently only supports
// imessage but more are coming/possible.
package messenger

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/davidnewhall/motifini/pkg/chat"
	"github.com/davidnewhall/motifini/pkg/export"
	"golift.io/imessage"
	"golift.io/securityspy"
	"golift.io/subscribe"
)

const (
	// APIiMessage is just an identifier for an imessage contact type.
	APIiMessage = "imessage"
)

type Messenger struct {
	chat    *chat.Chat
	imsg    *imessage.Messages
	Conf    *imessage.Config
	Subs    *subscribe.Subscribe
	SSpy    *securityspy.Server
	Info    *log.Logger
	Debug   *log.Logger
	Error   *log.Logger
	TempDir string
}

func New(m *Messenger) error {
	if m.Conf == nil {
		return fmt.Errorf("imessage config is nil")
	}
	if m.SSpy == nil {
		return fmt.Errorf("securityspy is nil")
	}
	if m.Subs == nil {
		return fmt.Errorf("subscribe is nil")
	}
	if m.Info == nil {
		m.Info = log.New(ioutil.Discard, "", 0)
	}
	if m.Debug == nil {
		m.Debug = log.New(ioutil.Discard, "", 0)
	}
	if m.Error == nil {
		m.Error = log.New(ioutil.Discard, "", 0)
	}
	if m.TempDir == "" {
		m.TempDir = "/tmp/"
	}
	m.chat = chat.New(&chat.Chat{TempDir: m.TempDir, Subs: m.Subs, SSpy: m.SSpy})
	m.Info.Println("Watching iMessage Database:", m.Conf.SQLPath)
	return m.startiMessage()
}

func (m *Messenger) SendFileOrMsg(id, msg, path string, subs []*subscribe.Subscriber) {
	for _, sub := range subs {
		switch sub.API {
		case APIiMessage:
			if path != "" {
				m.SendiMessage(imessage.Outgoing{ID: id, To: sub.Contact, Text: path, File: true, Call: m.FileCallback})
			}
			if msg != "" {
				m.SendiMessage(imessage.Outgoing{ID: id, To: sub.Contact, Text: msg})
			}
		default:
			m.Error.Printf("[%v] Unknown Notification API '%v' for contact: %v", id, sub.API, sub.Contact)
		}
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
