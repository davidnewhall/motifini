// Package messenger disambiguates messenger protocols. Currently only supports Telegram.
package messenger

import (
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"

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
func New(msgCfg *Messenger) error {
	if msgCfg.stopall != nil {
		return fmt.Errorf("%w: already running", ErrNillConfigItem)
	}

	if msgCfg.Telegram == nil {
		return fmt.Errorf("%w: telegram is nil, need at least one", ErrNillConfigItem)
	}

	if msgCfg.Chat == nil {
		return fmt.Errorf("%w: chat is nil", ErrNillConfigItem)
	}

	if msgCfg.Subs == nil {
		return fmt.Errorf("%w: subscribe is nil", ErrNillConfigItem)
	}

	if msgCfg.Info == nil {
		msgCfg.Info = log.New(io.Discard, "", 0)
	}

	if msgCfg.Debug == nil {
		msgCfg.Debug = log.New(io.Discard, "", 0)
	}

	if msgCfg.Error == nil {
		msgCfg.Error = log.New(io.Discard, "", 0)
	}

	if msgCfg.TempDir == "" {
		msgCfg.TempDir = "/tmp/"
	}

	return msgCfg.Start()
}

// Start connects configured messengers and begins background receivers.
func (m *Messenger) Start() error {
	m.stopall = make(chan struct{})

	if m.Telegram != nil {
		err := m.connectTelegram()
		if err != nil {
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
// When path is set, the file is removed after all subscribers have been attempted.
func (m *Messenger) SendFileOrMsg(reqID, msg, path string, subs []*subscribe.Subscriber) {
	if path != "" {
		defer os.Remove(path) // best-effort temp cleanup
	}

	for _, sub := range subs {
		switch sub.API {
		case APITelegram:
			m.SendTelegram(reqID, msg, path, sub.ID, sub.Contact)
		default:
			m.Error.Printf("[%v] Unknown Notification API '%v' for contact: %v", reqID, sub.API, sub.Contact)
		}
	}
}

// ReqID makes a random string to identify requests in the logs.
func ReqID(n int) string {
	letters := []rune("abcdefghjkmnopqrstuvwxyzABCDEFGHJKMNPQRTUVWXYZ23456789")
	buf := make([]rune, n)

	for i := range buf {
		buf[i] = letters[rand.Intn(len(letters))] //nolint:gosec // not security critical.
	}

	return string(buf)
}
