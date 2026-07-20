// Package chat provides a chat-bot interface to subscribe, unscubscribe and receive
// notifications from events or SecuritySpy cameras.
package chat

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golift.io/securityspy/v2"
	"golift.io/subscribe"
)

// DefaultRepeatDelay is used when a subscription has no explicit "delay" rule.
const DefaultRepeatDelay = time.Minute

// MaxPauseMinutes is the longest pause/stop duration accepted (24 hours).
const MaxPauseMinutes = 1440

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
	// Callback is set for inline-keyboard presses (Telegram callback_data).
	Callback string
	// SendFile, when set, delivers each captured file immediately (progressive Telegram sends).
	// Path ownership transfers to the callback (it should delete the file when done).
	SendFile func(path, caption string) error
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
	if !c.commandReady(handler) {
		return &Reply{}
	}

	if reply := c.applyPendingRename(handler); reply != nil {
		return reply
	}

	if strings.EqualFold("help", commandName(handler.Text[0])) {
		return c.doHelp(handler)
	}

	resp, save := c.doCmd(handler)
	if save {
		_ = c.Subs.StateFileSave()
	}

	return resp
}

func (c *Chat) commandReady(handler *Handler) bool {
	return c.Subs != nil && c.SSpy != nil && c.TempDir != "" &&
		handler != nil && handler.Sub != nil && !handler.Sub.Ignored && handler.Text != nil
}

// applyPendingRename returns a reply when a rename was consumed.
// nil means fall through (no pending rename, or slash command cancelled it).
func (c *Chat) applyPendingRename(handler *Handler) *Reply {
	reply, handled, save := c.consumePendingRename(handler)
	if !handled {
		return nil
	}

	if save {
		_ = c.Subs.StateFileSave()
	}

	return reply
}

// HandleCallback routes inline-keyboard presses (messenger-agnostic callback_data).
func (c *Chat) HandleCallback(handler *Handler) *Reply {
	if c.Subs == nil || c.SSpy == nil || handler == nil || handler.Sub == nil || handler.Sub.Ignored {
		return &Reply{}
	}

	resp, save := c.handleWizardCallback(handler)
	if save {
		_ = c.Subs.StateFileSave()
	}

	return resp
}

func (c *Chat) doHelp(handler *Handler) *Reply {
	if len(handler.Text) < twoItems {
		root := c.helpWizardRootFor(handler)
		root.Edit = false

		return root
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

		cmdReply, cmdSave := c.Cmds[i].run(handler)
		mergeCmdReply(resp, cmdReply)
		found = cmdReply.Found || found
		save = save || cmdSave
	}

	if !found && handler.Sub.Admin {
		resp.Reply = "Command not found: " + handler.Text[0]
	}

	return resp, save
}

// mergeCmdReply folds one command result into the accumulated reply.
// Multiple command groups can match the same alias (e.g. user+admin /subs).
func mergeCmdReply(resp, part *Reply) {
	if part.Reply != "" {
		if resp.Reply != "" {
			resp.Reply = strings.TrimRight(resp.Reply, "\n") + "\n\n"
		}

		resp.Reply += part.Reply
	}

	resp.Files = append(resp.Files, part.Files...)
	if len(part.Keyboard) > 0 {
		resp.Keyboard = part.Keyboard
	}

	resp.Edit = resp.Edit || part.Edit
	if part.Toast != "" {
		resp.Toast = part.Toast
	}
}

func commandName(raw string) string {
	name := strings.ToLower(strings.TrimPrefix(raw, "/"))
	if at := strings.IndexByte(name, '@'); at >= 0 {
		name = name[:at]
	}

	return name
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
	cmdName := commandName(handler.Text[0])
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
