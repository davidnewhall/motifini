// Package messenger disambiguates messenger protocols. Currently only supports
// imessage but more are coming/possible.
package messenger

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"

	"github.com/davidnewhall/motifini/pkg/chat"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golift.io/imessage"
	"golift.io/subscribe"
)

const (
	// APIiMessage is just an identifier for an imessage contact type.
	APIiMessage = "imessage"
	APITelegram = "telegram"
)

// Messenger is all the data needed to initialize this library.
type Messenger struct {
	Chat     *chat.Chat
	imsg     *imessage.Messages
	telebot  *tgbotapi.BotAPI
	Telegram *TelegramConfig
	Conf     *imessage.Config
	Subs     *subscribe.Subscribe
	Info     *log.Logger
	Debug    *log.Logger
	Error    *log.Logger
	TempDir  string
	stopall  chan struct{}
}

var ErrNillConfigItem = fmt.Errorf("a required configuration item was not provided")

// New provides a messenger handler.
func New(m *Messenger) error {
	if m.stopall != nil {
		return fmt.Errorf("%w: already running", ErrNillConfigItem)
	}

	if m.Conf == nil && m.Telegram == nil {
		return fmt.Errorf("%w: imessage and telegram are nil, need at least one", ErrNillConfigItem)
	}

	if m.Chat == nil {
		return fmt.Errorf("%w: chat is nil", ErrNillConfigItem)
	}

	if m.Subs == nil {
		return fmt.Errorf("%w: subscribe is nil", ErrNillConfigItem)
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

	return m.Start()
}

func (m *Messenger) Start() error {
	m.stopall = make(chan struct{})

	if m.Telegram != nil {
		if err := m.connectTelegram(); err != nil {
			return err
		}

		go m.startTelegram()
	}

	if m.imsg != nil {
		return m.startiMessage()
	}

	return nil
}

// Stop closes the iMessage and telegram routines.
func (m *Messenger) Stop() {
	defer close(m.stopall)

	if m.imsg != nil {
		m.imsg.Stop()
	}
}

// SendFileOrMsg will send a notification to any subscriber provided using any supported messenger.
// This method is used by event handlers to notify subscribers.
func (m *Messenger) SendFileOrMsg(id, msg, path string, subs []*subscribe.Subscriber) {
	for _, sub := range subs {
		switch sub.API {
		case APIiMessage:
			if path != "" {
				m.SendiMessage(imessage.Outgoing{ID: id, To: sub.Contact, Text: path, File: true})
			}

			if msg != "" {
				m.SendiMessage(imessage.Outgoing{ID: id, To: sub.Contact, Text: msg})
			}
		case APITelegram:
			m.SendTelegram(id, msg, path, sub.ID)
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
		b[i] = l[rand.Intn(len(l))] // nolint:gosec
	}

	return string(b)
}
