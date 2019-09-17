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
	Aliases     []string
	Description string
	Usage       string
	Run         func(handle *CommandHandler) (reply *CommandReply, err error)
	Save        bool
}

// CommandMap contains a list of related or grouped commands.
type CommandMap struct {
	Title string
	Level int // not used yet
	List  []*Command
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
	Found bool
}

// HasAlias returns true if a command matches a specific alias.
// Use this to determine if a command should be run based in input text.
func (c *Command) HasAlias(alias string) bool {
	for _, a := range c.Aliases {
		if strings.EqualFold(a, alias) {
			return true
		}
	}
	return false
}

func (c *CommandMap) GetCommand(alias string) *Command {
	for i, cmd := range c.List {
		if cmd.HasAlias(alias) {
			return c.List[i]
		}
	}
	return nil
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
	var cmdFound bool
	for i := range c.Cmds {
		if !h.Sub.Admin && c.Cmds[i].Level > 2 {
			continue
		}
		reply, ok := c.Cmds[i].help(h.Text[1])
		cmdFound = ok || cmdFound
		resp.Reply += reply
	}
	if !cmdFound {
		resp.Reply += "Command not found: " + h.Text[1]
	}
	return resp
}

func (c *Chat) doCmd(h *CommandHandler) (*CommandReply, bool) {
	resp := &CommandReply{}
	var save bool
	var found bool
	for i := range c.Cmds {
		if !h.Sub.Admin && c.Cmds[i].Level > 2 {
			continue
		}
		r, s := c.Cmds[i].run(h)
		if r == nil {
			continue
		}
		found = r.Found || found
		resp.Reply += r.Reply
		resp.Files = append(resp.Files, r.Files...)
		save = save || s
	}
	if !found && h.Sub.Admin {
		resp.Reply = "Command not found: " + h.Text[0]
	}
	return resp, save
}

func (c *CommandMap) run(h *CommandHandler) (*CommandReply, bool) {
	name := strings.ToLower(h.Text[0])
	Cmd := c.GetCommand(name)
	if Cmd == nil || Cmd.Run == nil {
		return &CommandReply{Found: false}, false
	}
	reply, err := Cmd.Run(h)
	if reply == nil {
		reply = &CommandReply{Found: true}
	}
	reply.Found = true
	if err != nil {
		reply.Reply = fmt.Sprintf("ERROR: %v\n%s Usage: %s %s\n%s\nDescription: %s\n",
			err, c.Title, name, Cmd.Usage, reply.Reply, Cmd.Description)
	}
	return reply, Cmd.Save && err == nil
}

func (c *CommandMap) help(cmdName string) (string, bool) {
	if cmdName != "" {
		Cmd := c.GetCommand(cmdName)
		if Cmd == nil {
			return "", false
		}
		return fmt.Sprintf("* %s Usage: %s %s\nDescription: %s\nAliases: %s\n",
			c.Title, cmdName, Cmd.Usage, Cmd.Description, strings.Join(Cmd.Aliases, ", ")), true
	}
	msg := "\n* " + c.Title + " Commands *\n"
	for _, Cmd := range c.List {
		msg += Cmd.Aliases[0] + " " + Cmd.Usage + "\n"
	}
	msg += "- More Info: help <cmd>\n"
	return msg, true
}
