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
	AKA  []string
	Desc string
	Use  string
	Run  func(handle *CommandHandler) (reply *CommandReply, err error)
	Save bool
}

// CommandMap contains a list of related or grouped commands.
type CommandMap struct {
	Title string
	Level int // not really used yet
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
	cmd := c.GetCommand(name)
	if cmd == nil || cmd.Run == nil {
		return &CommandReply{Found: false}, false
	}
	reply, err := cmd.Run(h)
	if reply == nil {
		reply = &CommandReply{Found: true}
	}
	reply.Found = true
	if err != nil {
		usage, _ := c.help(name)
		reply.Reply = fmt.Sprintf("ERROR: %v\n%s\n%s\n", err, reply.Reply, usage)
	}
	return reply, cmd.Save && err == nil
}

func (c *CommandMap) help(cmdName string) (string, bool) {
	if cmdName != "" {
		cmd := c.GetCommand(cmdName)
		if cmd == nil {
			return "", false
		}
		return fmt.Sprintf("* %s Usage: %s %s\nDetail: %s\nAlias: %s\n",
			c.Title, cmd.AKA[0], cmd.Use, cmd.Desc, strings.Join(cmd.AKA, ", ")), true
	}
	msg := "\n* " + c.Title + " Commands *\n"
	for _, cmd := range c.List {
		msg += cmd.AKA[0] + " " + cmd.Use + "\n"
	}
	msg += "- More Info: help <cmd>\n"
	return msg, true
}

// GetCommand returns a command for an alias. Or nil. Check for nil.
func (c *CommandMap) GetCommand(command string) *Command {
	for i, cmd := range c.List {
		if cmd.HasCmdAlias(command) {
			return c.List[i]
		}
	}
	return nil
}

// HasCmdAlias returns true if a command matches a specific alias.
// Use this to determine if a command should be run based in input text.
func (c *Command) HasCmdAlias(alias string) bool {
	for _, a := range c.AKA {
		if strings.EqualFold(a, alias) {
			return true
		}
	}
	return false
}
