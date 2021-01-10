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
// Set all these variables before calling HandleCommand().
type Chat struct {
	Subs    *subscribe.Subscribe
	SSpy    *securityspy.Server
	TempDir string
	Cmds    []*Commands
}

// ErrorBadUsage is a standard error.
var ErrorBadUsage = fmt.Errorf("invalid command usage")

// Command is the configuration for a chat command handler.
type Command struct {
	AKA  []string
	Desc string
	Use  string
	Run  func(handle *Handler) (reply *Reply, err error)
	Save bool
}

type CmdLevel int

// Commands contains a list of related or grouped commands.
type Commands struct {
	Title string
	Level CmdLevel // not really used yet
	List  []*Command
}

const (
	LevelNone CmdLevel = iota
	LevelUser
	LevelMod
	LevelAdmin
	LevelOwner
)

// Handler contains the data to handle a command. ie. an incoming message.
type Handler struct {
	API  string
	ID   string
	Sub  *subscribe.Subscriber
	Text []string
	From string
}

// Reply is what the requestor gets in return. A message and/or some files.
type Reply struct {
	Reply string
	Files []string
	Found bool
}

// New just adds the basic commands to a Chat struct.
func New(c *Chat) *Chat {
	if c.TempDir == "" {
		c.TempDir = "/tmp"
	}

	defaults := []*Commands{c.nonAdminCommands(), c.adminCommands()}
	c.Cmds = append(defaults, c.Cmds...)

	return c
}

// HandleCommand builds responses and runs actions from incoming chat commands.
func (c *Chat) HandleCommand(h *Handler) *Reply {
	if c.Subs == nil || c.SSpy == nil || c.TempDir == "" ||
		h == nil || h.Sub == nil || h.Sub.Ignored || h.Text == nil {
		return &Reply{}
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

func (c *Chat) doHelp(h *Handler) *Reply {
	if len(h.Text) < twoItems {
		// Request general help.
		h.Text = append(h.Text, "")
	}

	// Request help for specific command.
	var (
		resp     = &Reply{}
		cmdFound bool
	)

	for i := range c.Cmds {
		if !h.Sub.Admin && c.Cmds[i].Level > LevelUser {
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

func (c *Chat) doCmd(h *Handler) (*Reply, bool) {
	var (
		resp        = &Reply{}
		save, found bool
	)

	for i := range c.Cmds {
		if !h.Sub.Admin && c.Cmds[i].Level > LevelUser {
			continue
		}

		r, s := c.Cmds[i].run(h)
		resp.Reply += r.Reply
		resp.Files = append(resp.Files, r.Files...)
		found = r.Found || found
		save = save || s
	}

	if !found && h.Sub.Admin {
		resp.Reply = "Command not found: " + h.Text[0]
	}

	return resp, save
}

func (c *Commands) run(h *Handler) (*Reply, bool) {
	cmdName := strings.ToLower(h.Text[0])
	cmd := c.GetCommand(cmdName)

	if cmd == nil || cmd.Run == nil {
		return &Reply{Found: false}, false
	}

	reply, err := cmd.Run(h)
	if reply == nil {
		reply = &Reply{Found: true}
	}

	reply.Found = true

	if err != nil {
		usage, _ := c.help(cmdName)
		reply.Reply = fmt.Sprintf("ERROR: %v\n%s\n%s\n", err, reply.Reply, usage)
	}

	return reply, cmd.Save && err == nil
}

func (c *Commands) help(cmdName string) (string, bool) {
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
func (c *Commands) GetCommand(command string) *Command {
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
