package messenger

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidnewhall/motifini/pkg/chat"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramConfig struct {
	Token string `toml:"token"`
	Debug bool   `toml:"debug"`
	Pass  string `toml:"password"`
}

func (m *Messenger) connectTelegram() (err error) {
	m.telebot, err = tgbotapi.NewBotAPI(m.Telegram.Token)
	if err != nil {
		return fmt.Errorf("token failed: %w", err)
	}

	m.Info.Printf("Authorized on account %s", m.telebot.Self.UserName)
	m.telebot.Debug = m.Telegram.Debug

	return nil
}

func (m *Messenger) startTelegram() {
	m.Info.Println("Connecting to Telegram!")

	u := tgbotapi.NewUpdate(0)
	u.Timeout = int(time.Minute.Seconds())

	updates := m.telebot.GetUpdatesChan(u)
	defer m.telebot.StopReceivingUpdates()

	for {
		select {
		case update := <-updates:
			if update.Message != nil { // If we got a message
				m.recvTelegramHandler(update)
			}
		case <-m.stopall:
			return
		}
	}
}

func (m *Messenger) recvTelegramHandler(update tgbotapi.Update) {
	m.Debug.Printf("Telegram [%d,%s] %s", update.Message.Chat.ID, update.Message.From.UserName, update.Message.Text)

	sub, err := m.Subs.GetSubscriberByID(update.Message.Chat.ID, APITelegram)
	if err != nil {
		// Every account we receive a message from gets logged as a subscriber with no subscriptions.
		sub = m.Subs.CreateSubWithID(update.Message.Chat.ID, update.Message.From.UserName,
			APITelegram, len(m.Subs.GetAdmins()) == 0, false)
		sub.Meta = map[string]interface{}{"hasAuth": false}
	}

	sub.Meta["user"] = update.Message.From

	//nolint:wsl
	if strings.TrimPrefix(update.Message.Text, "/") == "id "+m.Telegram.Pass {
		sub.Meta["hasAuth"] = true
		sub = m.Subs.CreateSubWithID(update.Message.Chat.ID, update.Message.From.UserName,
			APITelegram, sub.Admin, false)
		m.SendTelegram("none", "You are now authenticated.", "", update.Message.Chat.ID)
		m.Info.Printf("Telegram Received from %d:%s (admin:%v, ignored:%v), 'id' command, authenticated.",
			update.Message.Chat.ID, update.Message.From.UserName, sub.Admin, sub.Ignored)
		return
	} else if a, _ := sub.Meta["hasAuth"].(bool); !a {
		m.Info.Printf("Telegram Received from %d:%s (admin:%v, ignored:%v), NOT authenticated (ignored), rcvd: %s",
			update.Message.Chat.ID, update.Message.From.UserName, sub.Admin, sub.Ignored, update.Message.Text)
		return
	}

	// Pass the message off to the chat command handler routines.
	handler := &chat.Handler{
		API:  APITelegram,
		ID:   ReqID(IDLength),
		Sub:  sub,
		Text: strings.Fields(update.Message.Text),
		From: update.Message.From.UserName,
	}

	m.Info.Printf("[%s] Telegram Received from %d:%s (admin:%v, ignored:%v), size: %d, cmd: %s",
		handler.ID, update.Message.Chat.ID, update.Message.From.UserName, sub.Admin, sub.Ignored,
		len(update.Message.Text), handler.Text[0])
	m.replyTelegramHandler(update, handler)
}

func (m *Messenger) replyTelegramHandler(update tgbotapi.Update, handler *chat.Handler) {
	resp := m.Chat.HandleCommand(handler)
	// Send the reply as files and/or text.
	if resp.Reply != "" && len(resp.Files) == 0 {
		reply := tgbotapi.NewMessage(update.Message.Chat.ID, resp.Reply)
		if _, err := m.telebot.Send(reply); err != nil {
			m.Error.Printf("[%s] Error Sending Telegram message: %v", handler.ID, err)
		}
	}

	for i, path := range resp.Files {
		if i > 0 && resp.Reply != "" {
			resp.Reply = "" // only send the caption on first file.
		}

		err := m.SendTelegramFile(path, resp.Reply, update.Message.Chat.ID)
		if err != nil {
			m.Error.Printf("[%s] Error Sending Telegram message: %v", handler.ID, err)
		}
	}
}

func (m *Messenger) SendTelegram(reqID string, msg, path string, telegramID int64) {
	if m.telebot == nil {
		return
	}

	if path != "" {
		if err := m.SendTelegramFile(path, msg, telegramID); err != nil {
			m.Error.Printf("[%s] Error Sending Telegram file: %v", reqID, err)
		}
	} else if msg != "" {
		if _, err := m.telebot.Send(tgbotapi.NewMessage(telegramID, msg)); err != nil {
			m.Error.Printf("[%s] Error Sending Telegram message: %v", reqID, err)
		}
	}
}

func (m *Messenger) SendTelegramFile(path, caption string, id int64) (err error) {
	if m.telebot == nil {
		return nil
	}

	switch x := filepath.Ext(path); x {
	case ".gif", ".jpg", ".jpeg", ".png":
		m.Info.Printf("Telegram: Sending Photo (%s) to %d", path, id)
		photo := tgbotapi.NewPhoto(id, tgbotapi.FilePath(path))
		photo.AllowSendingWithoutReply = true
		photo.DisableNotification = false
		photo.Caption = caption
		_, err = m.telebot.Send(photo)
	case ".mov", ".m4v", ".mp4":
		m.Info.Printf("Telegram: Sending Video (%s) to %d", path, id)
		video := tgbotapi.NewVideo(id, tgbotapi.FilePath(path))
		video.SupportsStreaming = true
		video.Caption = caption
		_, err = m.telebot.Send(video)
	case ".wav", ".mp3":
		m.Info.Printf("Telegram: Sending Audio (%s) to %d", path, id)
		audio := tgbotapi.NewAudio(id, tgbotapi.FilePath(path))
		audio.Caption = caption
		_, err = m.telebot.Send(audio)
	default:
		m.Info.Printf("Telegram: Sending Document (%s|%s) to %d", path, x, id)
		doc := tgbotapi.NewDocument(id, tgbotapi.FilePath(path))
		doc.Caption = caption
		_, err = m.telebot.Send(doc)
	}

	if err != nil {
		return fmt.Errorf("sending telegram: %w", err)
	}

	return nil
}
