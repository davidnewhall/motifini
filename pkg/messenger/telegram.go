package messenger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidnewhall/motifini/pkg/chat"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golift.io/subscribe"
)

const (
	// IDLength is the length of generated request IDs for Telegram logs.
	IDLength              = 4
	mebibyte              = 1024 * 1024
	uploadWait            = 20 * time.Second
	telegramCaptionMaxLen = 1024
)

// TelegramConfig is the Telegram bot settings from the config file.
type TelegramConfig struct {
	Token string `toml:"token"`
	Debug bool   `toml:"debug"`
	Pass  string `toml:"password"`
}

func (m *Messenger) connectTelegram() error {
	var err error

	m.telebot, err = tgbotapi.NewBotAPI(m.Telegram.Token)
	if err != nil {
		return fmt.Errorf("token failed: %w", err)
	}

	m.Info.Printf("Authorized on account %s", m.telebot.Self.UserName)
	m.telebot.Debug = m.Telegram.Debug
	m.registerTelegramCommands()

	return nil
}

func (m *Messenger) registerTelegramCommands() {
	cmds := []tgbotapi.BotCommand{
		{Command: "help", Description: "Menu of everything"},
		{Command: "sub", Description: "Subscribe (tap menu)"},
		{Command: "unsub", Description: "Unsubscribe (tap menu)"},
		{Command: "subs", Description: "Manage your subscriptions"},
		{Command: "users", Description: "Admin: manage subscribers"},
		{Command: "cams", Description: "Cameras — snapshot or video"},
		{Command: "pics", Description: "Snapshot (tap a camera)"},
		{Command: "vid", Description: "Video clip (tap a camera)"},
		{Command: "stop", Description: "Pause alerts (tap menu)"},
		{Command: "delay", Description: "Repeat delay (tap menu)"},
		{Command: "events", Description: "Events — tap to subscribe"},
	}

	_, err := m.telebot.Request(tgbotapi.NewSetMyCommands(cmds...))
	if err != nil {
		m.Error.Printf("Telegram setMyCommands: %v", err)
	}
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
			switch {
			case update.CallbackQuery != nil:
				m.recvTelegramCallback(update.CallbackQuery)
			case update.Message != nil:
				m.recvTelegramHandler(update.Message)
			}
		case <-m.stopall:
			return
		}
	}
}

func (m *Messenger) recvTelegramCallback(callback *tgbotapi.CallbackQuery) {
	if callback == nil || callback.From == nil || callback.Message == nil {
		return
	}

	displayName := telegramContactName(callback.From)
	m.Debug.Printf("Telegram callback [%d,%s] %s", callback.Message.Chat.ID, displayName, callback.Data)

	sub, err := m.Subs.GetSubscriberByID(callback.Message.Chat.ID, APITelegram)
	if err != nil {
		_, _ = m.telebot.Request(tgbotapi.NewCallback(callback.ID, "Not authenticated"))

		return
	}

	hasAuth := false
	if sub.Meta != nil {
		hasAuth, _ = sub.Meta["hasAuth"].(bool)
	}

	if !hasAuth {
		_, _ = m.telebot.Request(tgbotapi.NewCallback(callback.ID, "Not authenticated"))
		m.Info.Printf("Telegram callback from %d:%s NOT authenticated", callback.Message.Chat.ID, displayName)

		return
	}

	ensureSubscriberContact(sub, displayName)
	if sub.Meta != nil && displayName != "" {
		sub.Meta["displayName"] = displayName
	}

	// Never block the Telegram update loop on slow media fetches.
	go m.handleTelegramCallback(callback, sub, displayName)
}

func (m *Messenger) handleTelegramCallback(
	callback *tgbotapi.CallbackQuery, sub *subscribe.Subscriber, displayName string,
) {
	handler := &chat.Handler{
		API:      APITelegram,
		ID:       ReqID(IDLength),
		Sub:      sub,
		From:     displayName,
		Callback: callback.Data,
		Text:     []string{callback.Data},
	}

	m.Info.Printf("[%s] Telegram callback from %d:%s data=%s",
		handler.ID, callback.Message.Chat.ID, displayName, callback.Data)

	toast := "…"
	mediaAll := callback.Data == "p:a" || callback.Data == "v:a"
	mediaOne := strings.HasPrefix(callback.Data, "p:") || strings.HasPrefix(callback.Data, "v:") ||
		strings.HasPrefix(callback.Data, "c:p:") || strings.HasPrefix(callback.Data, "c:v:")
	if mediaAll || mediaOne {
		toast = "Working…"
	}
	_, _ = m.telebot.Request(tgbotapi.NewCallback(callback.ID, toast))

	if mediaAll {
		kind := "snapshots"
		if callback.Data == "v:a" {
			kind = "video clips"
		}
		edit := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID,
			"Fetching "+kind+" — they'll arrive one at a time as each camera finishes…")
		_, err := m.telebot.Send(edit)
		if err != nil {
			m.Error.Printf("[%s] status edit: %v", handler.ID, err)
		}

		handler.SendFile = func(path, caption string) error {
			err := m.SendTelegramFile(handler.ID, path, caption, callback.Message.Chat.ID, displayName)
			_ = os.Remove(path)

			return err
		}
	}

	resp := m.Chat.HandleCallback(handler)
	m.sendTelegramReply(callback.Message.Chat.ID, callback.Message.MessageID, "", handler.ID, displayName, resp)
}

func (m *Messenger) recvTelegramHandler(msg *tgbotapi.Message) {
	displayName := telegramContactName(msg.From)
	m.Debug.Printf("Telegram [%d,%s] %s", msg.Chat.ID, displayName, msg.Text)

	sub, err := m.Subs.GetSubscriberByID(msg.Chat.ID, APITelegram)
	if err != nil {
		// Every account we receive a message from gets logged as a subscriber with no subscriptions.
		sub = m.Subs.CreateSubWithID(msg.Chat.ID, displayName,
			APITelegram, len(m.Subs.GetAdmins()) == 0, false)
		sub.Meta = map[string]any{"hasAuth": false}
	}

	ensureSubscriberContact(sub, displayName)

	if sub.Meta == nil {
		sub.Meta = map[string]any{"hasAuth": false}
	}
	sub.Meta["user"] = msg.From
	if displayName != "" {
		sub.Meta["displayName"] = displayName
	}

	if strings.TrimPrefix(msg.Text, "/") == "id "+m.Telegram.Pass {
		sub.Meta["hasAuth"] = true
		sub = m.Subs.CreateSubWithID(msg.Chat.ID, displayName,
			APITelegram, sub.Admin, false)
		ensureSubscriberContact(sub, displayName)
		m.SendTelegram("none", "You are now authenticated.", "", msg.Chat.ID, displayName)
		m.Info.Printf("Telegram Received from %d:%s (admin:%v, ignored:%v), 'id' command, authenticated.",
			msg.Chat.ID, displayName, sub.Admin, sub.Ignored)

		return
	} else if a, _ := sub.Meta["hasAuth"].(bool); !a {
		m.Info.Printf("Telegram Received from %d:%s (admin:%v, ignored:%v), NOT authenticated (ignored), rcvd: %s",
			msg.Chat.ID, displayName, sub.Admin, sub.Ignored, msg.Text)

		return
	}

	// Pass the message off to the chat command handler routines.
	handler := &chat.Handler{
		API:  APITelegram,
		ID:   ReqID(IDLength),
		Sub:  sub,
		Text: strings.Fields(msg.Text),
		From: displayName,
	}

	m.Info.Printf("[%s] Telegram Received from %d:%s (admin:%v, ignored:%v), size: %d, cmd: %s",
		handler.ID, msg.Chat.ID, displayName, sub.Admin, sub.Ignored,
		len(msg.Text), handler.Text[0])
	m.replyTelegramHandler(msg, handler)
}

// telegramContactName prefers @username; falls back to first/last name.
func telegramContactName(from *tgbotapi.User) string {
	if from == nil {
		return ""
	}

	if from.UserName != "" {
		return from.UserName
	}

	return strings.TrimSpace(from.FirstName + " " + from.LastName)
}

// ensureSubscriberContact keeps Contact filled from Telegram when blank.
// Never clears an existing Contact (admin /name or prior first/last stay).
func ensureSubscriberContact(sub *subscribe.Subscriber, displayName string) {
	if sub == nil || strings.TrimSpace(sub.Contact) != "" || strings.TrimSpace(displayName) == "" {
		return
	}

	sub.Contact = strings.TrimSpace(displayName)
}

func (m *Messenger) replyTelegramHandler(msg *tgbotapi.Message, handler *chat.Handler) {
	resp := m.Chat.HandleCommand(handler)
	m.sendTelegramReply(msg.Chat.ID, 0, "", handler.ID, handler.From, resp)
}

func (m *Messenger) sendTelegramReply(
	chatID int64, messageID int, callbackID, reqID, contact string, resp *chat.Reply,
) {
	if resp == nil {
		return
	}

	m.answerTelegramCallback(callbackID, reqID, resp.Toast)

	kb := telegramInlineKeyboard(resp.Keyboard)
	switch {
	case resp.Edit && messageID != 0:
		m.editTelegramMessage(chatID, messageID, reqID, contact, resp.Reply, kb)
	case resp.Reply != "" && len(resp.Files) == 0:
		m.sendTelegramText(chatID, reqID, contact, resp.Reply, kb)
	}

	m.sendTelegramReplyFiles(chatID, reqID, contact, resp)
}

func (m *Messenger) answerTelegramCallback(callbackID, reqID, toast string) {
	if callbackID == "" {
		return
	}

	if toast == "" {
		toast = " "
	}

	_, err := m.telebot.Request(tgbotapi.NewCallback(callbackID, toast))
	if err != nil {
		m.Error.Printf("[%s] Telegram answerCallback: %v", reqID, err)
	}
}

func (m *Messenger) editTelegramMessage(
	chatID int64, messageID int, reqID, contact, text string, kb *tgbotapi.InlineKeyboardMarkup,
) {
	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	if kb != nil {
		edit.ReplyMarkup = kb
	} else {
		empty := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}}
		edit.ReplyMarkup = &empty
	}

	_, err := m.telebot.Send(edit)
	if err != nil {
		m.Error.Printf("[%s] Error editing Telegram message to %d:%s: %v", reqID, chatID, contact, err)
	}
}

func (m *Messenger) sendTelegramText(
	chatID int64, reqID, contact, text string, kb *tgbotapi.InlineKeyboardMarkup,
) {
	msg := tgbotapi.NewMessage(chatID, text)
	if kb != nil {
		msg.ReplyMarkup = kb
	}

	_, err := m.telebot.Send(msg)
	if err != nil {
		m.Error.Printf("[%s] Error Sending Telegram message to %d:%s: %v", reqID, chatID, contact, err)
	}
}

func (m *Messenger) sendTelegramReplyFiles(chatID int64, reqID, contact string, resp *chat.Reply) {
	for i, path := range resp.Files {
		caption := resp.Reply
		if i > 0 {
			caption = ""
		}

		err := m.SendTelegramFile(reqID, path, caption, chatID, contact)
		if err != nil {
			m.Error.Printf("[%s] Error Sending Telegram file to %d:%s: %v", reqID, chatID, contact, err)
		}

		_ = os.Remove(path) // command-generated temps
	}
}

func telegramInlineKeyboard(rows [][]chat.Button) *tgbotapi.InlineKeyboardMarkup {
	if len(rows) == 0 {
		return nil
	}

	out := make([][]tgbotapi.InlineKeyboardButton, 0, len(rows))
	for _, row := range rows {
		tgRow := make([]tgbotapi.InlineKeyboardButton, 0, len(row))
		for _, btn := range row {
			tgRow = append(tgRow, tgbotapi.NewInlineKeyboardButtonData(btn.Label, btn.Data))
		}
		out = append(out, tgRow)
	}

	kb := tgbotapi.NewInlineKeyboardMarkup(out...)

	return &kb
}

// SendTelegram sends a text message or file to a Telegram chat ID.
func (m *Messenger) SendTelegram(reqID, msg, path string, telegramID int64, contact string) {
	if m.telebot == nil {
		return
	}

	if contact == "" {
		contact = "?"
	}

	if path != "" {
		err := m.SendTelegramFile(reqID, path, msg, telegramID, contact)
		if err != nil {
			m.Error.Printf("[%s] Error Sending Telegram file to %d:%s: %v", reqID, telegramID, contact, err)
		}
	} else if msg != "" {
		_, err := m.telebot.Send(tgbotapi.NewMessage(telegramID, msg))
		if err != nil {
			m.Error.Printf("[%s] Error Sending Telegram message to %d:%s: %v", reqID, telegramID, contact, err)
		}
	}
}

// SendTelegramFile uploads a local file to Telegram.
// Callers own cleanup of path (so the same file can be sent to multiple chats).
func (m *Messenger) SendTelegramFile(reqID, path, caption string, telegramID int64, contact string) error {
	if m.telebot == nil {
		return nil
	}

	if contact == "" {
		contact = "?"
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	caption = trimTelegramCaption(caption)
	dest := fmt.Sprintf("%d:%s", telegramID, contact)

	switch ext := filepath.Ext(path); ext {
	case ".gif", ".jpg", ".jpeg", ".png":
		m.Info.Printf("[%s] Telegram: Sending Photo (%s, %.2fMb) to %s",
			reqID, path, float64(fileInfo.Size())/mebibyte, dest)
		photo := tgbotapi.NewPhoto(telegramID, tgbotapi.FilePath(path))
		photo.AllowSendingWithoutReply = true
		photo.DisableNotification = false
		photo.Caption = caption
		_, err = m.telebot.Send(photo)
	case ".mov", ".m4v", ".mp4":
		m.Info.Printf("[%s] Telegram: Sending Video (%s, %.2fMb) to %s",
			reqID, path, float64(fileInfo.Size())/mebibyte, dest)
		video := tgbotapi.NewVideo(telegramID, tgbotapi.FilePath(path))
		video.SupportsStreaming = true
		video.Caption = caption
		started := time.Now()
		_, err = m.telebot.Send(video)
		if err == nil {
			m.Info.Printf("[%s] Telegram: Sent Video to %s in %s", reqID, dest, time.Since(started).Round(time.Millisecond))
		}
	case ".wav", ".mp3":
		m.Info.Printf("[%s] Telegram: Sending Audio (%s, %.2fMb) to %s",
			reqID, path, float64(fileInfo.Size())/mebibyte, dest)
		audio := tgbotapi.NewAudio(telegramID, tgbotapi.FilePath(path))
		audio.Caption = caption
		_, err = m.telebot.Send(audio)
	default:
		m.Info.Printf("[%s] Telegram: Sending Document (%s, %.2fMb) to %s",
			reqID, path, float64(fileInfo.Size())/mebibyte, dest)
		doc := tgbotapi.NewDocument(telegramID, tgbotapi.FilePath(path))
		doc.Caption = caption
		_, err = m.telebot.Send(doc)
	}

	if err != nil {
		return fmt.Errorf("sending telegram: %w", err)
	}

	return nil
}

func trimTelegramCaption(caption string) string {
	runes := []rune(caption)
	if len(runes) <= telegramCaptionMaxLen {
		return caption
	}

	return string(runes[:telegramCaptionMaxLen])
}
