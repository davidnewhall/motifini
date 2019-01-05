package messages

import (
	"bytes"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// Msg struct
type Msg struct {
	ID   string
	To   string
	Text string
	File bool
	Call func(id, to, text string, err error)
}

// Messages is the interface into this module.
type Messages interface {
	RunAppleScript(id string, scripts []string, retry int) error
	Send(msg Msg)
	ClearMessages()
}

// Config is our input data, data store, and interface to methods.
type Config struct {
	ClearMsgs bool
	QueueSize int
	Debug     bool
	msgChan   chan Msg
}

// Init reads incoming messages destined for iMessage buddies.
// The messages are queued in a channel and sent 1 at a time with a small
// delay between. Each message may have a callback attached that is kicked
// off in a go routine after the message is sent.
func Init(c *Config) Messages {
	c.msgChan = make(chan Msg, c.QueueSize)
	go c.watchMsgChan()
	return c
}

// watchMsgChan keeps an eye out for incoming messages; then processes them.
func (c *Config) watchMsgChan() {
	newMsg := true
	clearTicker := time.NewTicker(2 * time.Minute).C
	for {
		select {
		case msg := <-c.msgChan:
			newMsg = true
			err := c.sendiMessage(msg)
			if msg.Call != nil {
				go msg.Call(msg.ID, msg.To, msg.Text, err)
			}
			// Give iMessage time to do its thing.
			time.Sleep(300 * time.Millisecond)
		case <-clearTicker:
			if c.ClearMsgs && newMsg {
				newMsg = false
				log.Println("Clearing Messages.app Conversations")
				c.ClearMessages()
				time.Sleep(300 * time.Millisecond)
			}
		}
	}
}

// Send a message.
func (c *Config) Send(msg Msg) {
	c.msgChan <- msg
}

func (c *Config) sendiMessage(m Msg) error {
	arg := []string{`tell application "Messages" to send "` + m.Text + `" to buddy "` + m.To +
		`" of (1st service whose service type = iMessage)`}
	if _, err := os.Stat(m.Text); err == nil && m.File {
		arg = []string{`tell application "Messages" to send (POSIX file ("` + m.Text + `")) to buddy "` + m.To +
			`" of (1st service whose service type = iMessage)`}
	}
	arg = append(arg, `tell application "Messages" to close every window`)
	if err := c.RunAppleScript(m.ID, arg, 3); err != nil {
		return errors.Wrapf(err, "(3/3) RunAppleScript")
	}
	return nil
}

// ClearMessages deletes all conversations in MESSAGES.APP
func (c *Config) ClearMessages() {
	arg := `tell application "Messages"
  activate
	try
		repeat (count of (get every chat)) times
			tell application "System Events" to tell process "Messages" to keystroke return
			delete item 1 of (get every chat)
			tell application "System Events" to tell process "Messages" to keystroke return
		end repeat
	end try
	close every window
end tell
`
	if err := c.RunAppleScript("wipe", []string{arg}, 2); err != nil {
		log.Printf("[ERROR] [wipe] (2/2) runAppleScript: %v", err)
	}
	time.Sleep(75 * time.Millisecond)
}

// RunAppleScript runs a script.
func (c *Config) RunAppleScript(id string, scripts []string, retry int) error {
	arg := []string{"/usr/bin/osascript"}
	for _, s := range scripts {
		arg = append(arg, "-e", s)
	}
	if c.Debug {
		log.Printf("[DEBUG] [%v] AppleScript Command: %v", id, strings.Join(arg, " "))
	}
	for i := 1; i <= retry; i++ {
		var out bytes.Buffer
		cmd := exec.Command(arg[0], arg[1:]...)
		cmd.Stdout = &out
		cmd.Stderr = &out
		if err := cmd.Run(); err == nil {
			if i > 1 {
				log.Printf("[RETRY] [%v] (%v/%v) cmd.Run: Previous error now successful!", id, i, retry)
			}
			break
		} else if i >= retry {
			return errors.Wrapf(errors.New(out.String()), "cmd.Run: %v", err.Error())
		} else {
			log.Printf("[ERROR] [%v] (%v/%v) cmd.Run: %v: %v", id, i, retry, err, out.String())
		}
		time.Sleep(750 * time.Millisecond)
	}
	return nil
}
