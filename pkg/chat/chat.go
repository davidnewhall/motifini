// Package chat provides a chat-bot interface to subscribe, unscubscribe and receive
// notifications from events or SecuritySpy cameras.
package chat

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golift.io/securityspy/v2"
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

// ErrBadUsage is a standard error.
var ErrBadUsage = errors.New("invalid command usage")

// Command is the configuration for a chat command handler.
type Command struct {
	AKA  []string
	Desc string
	Use  string
	Run  func(handle *Handler) (reply *Reply, err error)
	Save bool
}

// CmdLevel is the authorization level required to run a command.
type CmdLevel int

// Commands contains a list of related or grouped commands.
type Commands struct {
	Title string
	Level CmdLevel // not really used yet
	List  []*Command
}

// Command access levels.
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
func New(chatCfg *Chat) *Chat {
	if chatCfg.TempDir == "" {
		chatCfg.TempDir = "/tmp"
	}

	defaults := make([]*Commands, 0, 2+len(chatCfg.Cmds)) //nolint:mnd // commands below....
	defaults = append(defaults, chatCfg.nonAdminCommands(), chatCfg.adminCommands())
	chatCfg.Cmds = append(defaults, chatCfg.Cmds...)

	return chatCfg
}

// HandleCommand builds responses and runs actions from incoming chat commands.
func (c *Chat) HandleCommand(handler *Handler) *Reply {
	if c.Subs == nil || c.SSpy == nil || c.TempDir == "" ||
		handler == nil || handler.Sub == nil || handler.Sub.Ignored || handler.Text == nil {
		return &Reply{}
	}

	if strings.EqualFold("help", strings.TrimPrefix(handler.Text[0], "/")) {
		return c.doHelp(handler)
	}

	// Run a command.
	resp, save := c.doCmd(handler)
	if save {
		_ = c.Subs.StateFileSave()
	}

	return resp
}

func (c *Chat) doHelp(handler *Handler) *Reply {
	if len(handler.Text) < twoItems {
		// Request general help.
		handler.Text = append(handler.Text, "")
	}

	// Request help for specific command.
	var (
		resp     = &Reply{}
		cmdFound bool
	)

	for i := range c.Cmds {
		if !handler.Sub.Admin && c.Cmds[i].Level > LevelUser {
			continue
		}

		reply, ok := c.Cmds[i].help(handler.Text[1])
		cmdFound = ok || cmdFound
		resp.Reply += reply
	}

	if !cmdFound {
		resp.Reply += "Command not found: " + handler.Text[1]
	}

	return resp
}

func (c *Chat) doCmd(handler *Handler) (*Reply, bool) {
	var (
		resp        = &Reply{}
		save, found bool
	)

	for i := range c.Cmds {
		if !handler.Sub.Admin && c.Cmds[i].Level > LevelUser {
			continue
		}

		r, s := c.Cmds[i].run(handler)
		resp.Reply += r.Reply
		resp.Files = append(resp.Files, r.Files...)
		found = r.Found || found
		save = save || s
	}

	if !found && handler.Sub.Admin {
		resp.Reply = "Command not found: " + handler.Text[0]
	}

	return resp, save
}

func (c *Chat) getSubscriber(contactID, api string) (*subscribe.Subscriber, error) {
	subscriber, err := c.Subs.GetSubscriber(contactID, api)
	if err == nil {
		return subscriber, nil
	}

	reqID, _ := strconv.ParseInt(contactID, 10, 64)

	subscriber, err = c.Subs.GetSubscriberByID(reqID, api)
	if err != nil {
		return subscriber, fmt.Errorf("missing subscriber: %w", err)
	}

	return subscriber, nil
}

func (c *Commands) run(handler *Handler) (*Reply, bool) {
	cmdName := strings.ToLower(strings.TrimPrefix(handler.Text[0], "/"))
	cmd := c.GetCommand(cmdName)

	if cmd == nil || cmd.Run == nil {
		return &Reply{Found: false}, false
	}

	reply, err := cmd.Run(handler)
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
			c.Title, "/"+cmd.AKA[0], cmd.Use, cmd.Desc, strings.Join(cmd.AKA, ", ")), true
	}

	var msg strings.Builder
	fmt.Fprintf(&msg, "\n* %s Commands *\n", c.Title)

	for _, cmd := range c.List {
		fmt.Fprintf(&msg, "/%s %s\n", cmd.AKA[0], cmd.Use)
	}

	msg.WriteString("- More Info: /help <cmd>\n")

	return msg.String(), true
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
