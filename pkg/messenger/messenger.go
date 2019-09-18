// Package messenger disambiguates messenger protocols. Currently only supports
// imessage but more are coming/possible.
package messenger

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"

	"github.com/davidnewhall/motifini/pkg/chat"
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

// SendFileOrMsg will send a notification to any subscriber provided using any supported messenger.
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

// ReqID makes a random string to identify requests in the logs.
func ReqID(n int) string {
	l := []rune("abcdefghjkmnopqrstuvwxyzABCDEFGHJKMNPQRTUVWXYZ23456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = l[rand.Intn(len(l))]
	}
	return string(b)
}
