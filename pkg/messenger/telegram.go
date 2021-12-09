package messenger

import (
	"fmt"
	"os"
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
				m.recvTelegramHandler(*update.Message)
			}
		case <-m.stopall:
			return
		}
	}
}

func (m *Messenger) recvTelegramHandler(msg tgbotapi.Message) {
	m.Debug.Printf("Telegram [%d,%s] %s", msg.Chat.ID, msg.From.UserName, msg.Text)

	sub, err := m.Subs.GetSubscriberByID(msg.Chat.ID, APITelegram)
	if err != nil {
		// Every account we receive a message from gets logged as a subscriber with no subscriptions.
		sub = m.Subs.CreateSubWithID(msg.Chat.ID, msg.From.UserName,
			APITelegram, len(m.Subs.GetAdmins()) == 0, false)
		sub.Meta = map[string]interface{}{"hasAuth": false}
	}

	if sub.Meta == nil {
		sub.Meta = map[string]interface{}{"hasAuth": false, "user": msg.From}
	} else {
		sub.Meta["user"] = msg.From
	}

	//nolint:wsl
	if strings.TrimPrefix(msg.Text, "/") == "id "+m.Telegram.Pass {
		sub.Meta["hasAuth"] = true
		sub = m.Subs.CreateSubWithID(msg.Chat.ID, msg.From.UserName,
			APITelegram, sub.Admin, false)
		m.SendTelegram("none", "You are now authenticated.", "", msg.Chat.ID)
		m.Info.Printf("Telegram Received from %d:%s (admin:%v, ignored:%v), 'id' command, authenticated.",
			msg.Chat.ID, msg.From.UserName, sub.Admin, sub.Ignored)
		return
	} else if a, _ := sub.Meta["hasAuth"].(bool); !a {
		m.Info.Printf("Telegram Received from %d:%s (admin:%v, ignored:%v), NOT authenticated (ignored), rcvd: %s",
			msg.Chat.ID, msg.From.UserName, sub.Admin, sub.Ignored, msg.Text)
		return
	}

	// Pass the message off to the chat command handler routines.
	handler := &chat.Handler{
		API:  APITelegram,
		ID:   ReqID(IDLength),
		Sub:  sub,
		Text: strings.Fields(msg.Text),
		From: msg.From.UserName,
	}

	m.Info.Printf("[%s] Telegram Received from %d:%s (admin:%v, ignored:%v), size: %d, cmd: %s",
		handler.ID, msg.Chat.ID, msg.From.UserName, sub.Admin, sub.Ignored,
		len(msg.Text), handler.Text[0])
	m.replyTelegramHandler(msg, handler)
}

func (m *Messenger) replyTelegramHandler(msg tgbotapi.Message, handler *chat.Handler) {
	resp := m.Chat.HandleCommand(handler)
	// Send the reply as files and/or text.
	if resp.Reply != "" && len(resp.Files) == 0 {
		reply := tgbotapi.NewMessage(msg.Chat.ID, resp.Reply)
		if _, err := m.telebot.Send(reply); err != nil {
			m.Error.Printf("[%s] Error Sending Telegram message: %v", handler.ID, err)
		}
	}

	for i, path := range resp.Files {
		if i > 0 && resp.Reply != "" {
			resp.Reply = "" // only send the caption on first file.
		}

		err := m.SendTelegramFile(handler.ID, path, resp.Reply, msg.Chat.ID)
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
		if err := m.SendTelegramFile(reqID, path, msg, telegramID); err != nil {
			m.Error.Printf("[%s] Error Sending Telegram file: %v", reqID, err)
		}
	} else if msg != "" {
		if _, err := m.telebot.Send(tgbotapi.NewMessage(telegramID, msg)); err != nil {
			m.Error.Printf("[%s] Error Sending Telegram message: %v", reqID, err)
		}
	}
}

func (m *Messenger) SendTelegramFile(reqID, path, caption string, id int64) (err error) {
	if m.telebot == nil {
		return nil
	}

	f, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	// TODO: this can't stay here in case other things need the file.
	defer os.Remove(path)

	switch x := filepath.Ext(path); x {
	case ".gif", ".jpg", ".jpeg", ".png":
		m.Info.Printf("[%s] Telegram: Sending Photo (%s, %.2fMb) to %d", reqID, path, float64(f.Size())/mebibyte, id)
		photo := tgbotapi.NewPhoto(id, tgbotapi.FilePath(path))
		photo.AllowSendingWithoutReply = true
		photo.DisableNotification = false
		photo.Caption = caption
		_, err = m.telebot.Send(photo)
	case ".mov", ".m4v", ".mp4":
		m.Info.Printf("[%s] Telegram: Sending Video (%s, %.2fMb) to %d", reqID, path, float64(f.Size())/mebibyte, id)
		video := tgbotapi.NewVideo(id, tgbotapi.FilePath(path))
		video.SupportsStreaming = true
		video.Caption = caption
		_, err = m.telebot.Send(video)
	case ".wav", ".mp3":
		m.Info.Printf("[%s] Telegram: Sending Audio (%s, %.2fMb) to %d", reqID, path, float64(f.Size())/mebibyte, id)
		audio := tgbotapi.NewAudio(id, tgbotapi.FilePath(path))
		audio.Caption = caption
		_, err = m.telebot.Send(audio)
	default:
		m.Info.Printf("[%s] Telegram: Sending Document (%s, %.2fMb) to %d", reqID, path, float64(f.Size())/mebibyte, id)
		doc := tgbotapi.NewDocument(id, tgbotapi.FilePath(path))
		doc.Caption = caption
		_, err = m.telebot.Send(doc)
	}

	if err != nil {
		return fmt.Errorf("sending telegram: %w", err)
	}

	return nil
}
