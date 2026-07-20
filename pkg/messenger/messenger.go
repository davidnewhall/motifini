// Package messenger disambiguates messenger protocols. Currently only supports Telegram.
package messenger

import (
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"

	"github.com/davidnewhall/motifini/pkg/chat"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golift.io/subscribe"
)

// Supported messenger APIs.
const (
	APITelegram = "telegram"
)

// Messenger is all the data needed to initialize this library.
type Messenger struct {
	Chat     *chat.Chat
	telebot  *tgbotapi.BotAPI
	Telegram *TelegramConfig
	Subs     *subscribe.Subscribe
	Info     *log.Logger
	Debug    *log.Logger
	Error    *log.Logger
	TempDir  string
	stopall  chan struct{}
}

// ErrNillConfigItem is returned when a required Messenger field is missing.
var ErrNillConfigItem = errors.New("a required configuration item was not provided")

// New provides a messenger handler.
func New(m *Messenger) error {
	if m.stopall != nil {
		return fmt.Errorf("%w: already running", ErrNillConfigItem)
	}

	if m.Telegram == nil {
		return fmt.Errorf("%w: telegram is nil, need at least one", ErrNillConfigItem)
	}

	if m.Chat == nil {
		return fmt.Errorf("%w: chat is nil", ErrNillConfigItem)
	}

	if m.Subs == nil {
		return fmt.Errorf("%w: subscribe is nil", ErrNillConfigItem)
	}

	if m.Info == nil {
		m.Info = log.New(io.Discard, "", 0)
	}

	if m.Debug == nil {
		m.Debug = log.New(io.Discard, "", 0)
	}

	if m.Error == nil {
		m.Error = log.New(io.Discard, "", 0)
	}

	if m.TempDir == "" {
		m.TempDir = "/tmp/"
	}

	return m.Start()
}

// Start connects configured messengers and begins background receivers.
func (m *Messenger) Start() error {
	m.stopall = make(chan struct{})

	if m.Telegram != nil {
		if err := m.connectTelegram(); err != nil {
			return err
		}

		go m.startTelegram()
	}

	return nil
}

// Stop closes the telegram routine.
func (m *Messenger) Stop() {
	close(m.stopall)
}

// SendFileOrMsg will send a notification to any subscriber provided using any supported messenger.
// This method is used by event handlers to notify subscribers.
func (m *Messenger) SendFileOrMsg(id, msg, path string, subs []*subscribe.Subscriber) {
	for _, sub := range subs {
		switch sub.API {
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
		b[i] = l[rand.Intn(len(l))] //nolint:gosec // not security critical.
	}

	return string(b)
}
