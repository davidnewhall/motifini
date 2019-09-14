// Package chat provides a chat-bot interface to subscribe, unscubscribe and receive
// notifications from events or SecuritySpy cameras.
package chat

import (
	"errors"
	"fmt"
	"strings"

	"golift.io/securityspy"
	"golift.io/subscribe"
)

/* Do not include message-provider-specific code in chat_* files. */

// If any of these are blank, the library doesn't work.
// Set all these variables before calling HandleCommand
var (
	Subs    *subscribe.Subscribe
	Spy     *securityspy.Server
	TempDir string
)

// ErrorBadUsage is a standard error
var ErrorBadUsage = errors.New("invalid command usage")

type chatCommands struct {
	Description string
	Usage       string
	Run         func(handle *CommandHandle) (string, []string, error)
	Save        bool
}

type commandMap struct {
	Kind string
	Map  map[string]chatCommands
}

// CommandHandle contains the data to handle a command.
type CommandHandle struct {
	API  string
	Cmds commandMap
	ID   string
	Sub  *subscribe.Subscriber
	Text []string
	From string
}

// CommandReply is what the requestor gets in return.
type CommandReply struct {
	Reply string
	Files []string
}

// HandleCommand builds responses and runs actions from incoming chat commands.
func HandleCommand(c *CommandHandle) *CommandReply {
	if Subs == nil || Spy == nil || TempDir == "" {
		return &CommandReply{}
	}
	if strings.EqualFold("help", c.Text[0]) {
		if len(c.Text) < 2 {
			return &CommandReply{Reply: c.Cmds.Help(c.Cmds.Kind, "")}
		}
		return &CommandReply{Reply: c.Cmds.Help(c.Cmds.Kind, c.Text[1])}
	}
	for name, Cmd := range c.Cmds.Map {
		if !strings.EqualFold(name, c.Text[0]) {
			continue
		}
		reply, files, err := Cmd.Run(c)
		if err != nil {
			return &CommandReply{Reply: fmt.Sprintf("ERROR: %v\n%s Usage: %s %s\n%s\nDescription: %s\n",
				err, c.Cmds.Kind, name, Cmd.Usage, reply, Cmd.Description)}
		}
		if Cmd.Save {
			_ = Subs.StateFileSave()
		}
		return &CommandReply{Reply: reply, Files: files}
	}
	return &CommandReply{}
}

func (c *commandMap) Help(kind string, cmdName string) string {
	if cmdName != "" {
		Cmd, ok := c.Map[cmdName]
		if !ok {
			return ""
		}
		return fmt.Sprintf("%s Usage: %s %s\n%s Description: %s\n",
			c.Kind, cmdName, Cmd.Usage, c.Kind, Cmd.Description)
	}
	msg := "\n=== " + kind + " Commands ===\n"
	for name, Cmd := range c.Map {
		msg += name + " " + Cmd.Usage
	}
	msg += "- Use 'help <cmd>' for more.\n"
	return msg
}
