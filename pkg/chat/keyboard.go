package chat

// Button is a messenger-agnostic inline keyboard button.
type Button struct {
	Label string
	Data  string // callback payload (Telegram: ≤64 bytes)
}

// Reply is what the requestor gets in return.
type Reply struct {
	Reply    string
	Files    []string
	Found    bool
	Keyboard [][]Button // optional inline keyboard rows
	// Edit replaces an existing message (callback menus) instead of sending a new one.
	Edit bool
	// Toast is a short popup for Telegram answerCallbackQuery.
	Toast string
}
