// Package chat provides a chat-bot interface to subscribe, unscubscribe and receive
// notifications from events or SecuritySpy cameras.
package chat

import (
	"fmt"
	"strings"

	"golift.io/securityspy"
	"golift.io/subscribe"
)

/* Do not include message-provider-specific code in chat_* files. */

// Chat is the input data to initialize the library.
// If any of these are blank, the library doesn't work.
// Set all these variables before calling HandleCommand
type Chat struct {
	Subs    *subscribe.Subscribe
	SSpy    *securityspy.Server
	TempDir string
	Cmds    []*CommandMap
}

// ErrorBadUsage is a standard error
var ErrorBadUsage = fmt.Errorf("invalid command usage")

// Command is the configuration for a chat command handler.
type Command struct {
	Description string
	Usage       string
	Run         func(handle *CommandHandler) (reply string, files []string, err error)
	Save        bool
}

// CommandMap contains a list of related or grouped commands.
type CommandMap struct {
	Title string
	Level int // not used yet
	Map   map[string]Command
}

// CommandHandler contains the data to handle a command. ie. an incoming message.
type CommandHandler struct {
	API  string
	ID   string
	Sub  *subscribe.Subscriber
	Text []string
	From string
}

// CommandReply is what the requestor gets in return. A message and/or some files.
type CommandReply struct {
	Reply string
	Files []string
}

// New just adds the basic commands to a Chat struct.
func New(c *Chat) *Chat {
	if c.TempDir == "" {
		c.TempDir = "/tmp"
	}
	defaults := []*CommandMap{c.NonAdminCommands(), c.AdminCommands()}
	c.Cmds = append(defaults, c.Cmds...)
	return c
}

// HandleCommand builds responses and runs actions from incoming chat commands.
func (c *Chat) HandleCommand(h *CommandHandler) *CommandReply {
	if c.Subs == nil || c.SSpy == nil || c.TempDir == "" || h.Sub.Ignored {
		return &CommandReply{}
	}

	if strings.EqualFold("help", h.Text[0]) {
		return c.doHelp(h)
	}

	// Run a command.
	resp, save := c.doCmd(h)
	if save {
		_ = c.Subs.StateFileSave()
	}
	return resp
}

func (c *Chat) doHelp(h *CommandHandler) *CommandReply {
	if len(h.Text) < 2 {
		// Request general help.
		h.Text = append(h.Text, "")
	}
	// Request help for specific command.
	resp := &CommandReply{}
	for i := range c.Cmds {
		if !h.Sub.Admin && c.Cmds[i].Level > 2 {
			continue
		}
		resp.Reply += c.Cmds[i].help(h.Text[1])
	}
	return resp
}

func (c *Chat) doCmd(h *CommandHandler) (*CommandReply, bool) {
	resp := &CommandReply{}
	var save bool
	for i := range c.Cmds {
		if !h.Sub.Admin && c.Cmds[i].Level > 2 {
			continue
		}
		r, s := c.Cmds[i].run(h)
		resp.Reply += r.Reply
		resp.Files = append(resp.Files, r.Files...)
		save = save || s
	}
	return resp, save
}

func (c *CommandMap) run(h *CommandHandler) (*CommandReply, bool) {
	name := strings.ToLower(h.Text[0])
	Cmd, ok := c.Map[name]
	if !ok || Cmd.Run == nil {
		return &CommandReply{}, false
	}
	reply, files, err := Cmd.Run(h)
	if err != nil {
		return &CommandReply{Reply: fmt.Sprintf("ERROR: %v\n%s Usage: %s %s\n%s\nDescription: %s\n",
			err, c.Title, name, Cmd.Usage, reply, Cmd.Description)}, false
	}
	return &CommandReply{Reply: reply, Files: files}, Cmd.Save
}

func (c *CommandMap) help(cmdName string) string {
	if cmdName != "" {
		Cmd, ok := c.Map[cmdName]
		if !ok {
			return ""
		}
		return fmt.Sprintf("%s Usage: %s %s\n%s Description: %s\n",
			c.Title, cmdName, Cmd.Usage, c.Title, Cmd.Description)
	}
	msg := "\n* " + c.Title + " Commands *\n"
	for name, Cmd := range c.Map {
		msg += name + " " + Cmd.Usage + "\n"
	}
	msg += "- More Info: help <cmd>\n"
	return msg
}
