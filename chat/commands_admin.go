package chat

import (
	"fmt"
	"strconv"
)

// AdminCommands contains all the built-in admin commands like 'ignore'
func (c *Chat) AdminCommands() *CommandMap {
	return &CommandMap{
		Title: "Admin",
		Level: 10,
		List: []*Command{
			{
				Aliases:     []string{"subs", "subscribers"},
				Usage:       "[subscriber]",
				Description: "Displays all subscribers.",
				Run:         c.cmdAdminSubs,
				Save:        false,
			},
			{
				Aliases:     []string{"ignores"},
				Description: "Displays all ignored subscribers.",
				Run:         c.cmdAdminIgnores,
				Save:        false,
			},
			{
				Aliases:     []string{"ignore"},
				Usage:       "<subscriber>",
				Description: "Ignores a subscriber.",
				Run:         c.cmdAdminIgnore,
				Save:        true,
			},
			{
				Aliases:     []string{"unignore"},
				Usage:       "<subscriber>",
				Description: "Removes a subscriber's ignore.",
				Run:         c.cmdAdminUnignore,
				Save:        true,
			},
			{
				Aliases:     []string{"admins"},
				Description: "Displays all administrative subscribers.",
				Run:         c.cmdAdminAdmins,
				Save:        false,
			},
			{
				Aliases:     []string{"admin"},
				Usage:       "<subscriber>",
				Description: "Gives a subscriber administrative access.",
				Run:         c.cmdAdminAdmin,
				Save:        true,
			},
			{
				Aliases:     []string{"unadmin", "unmasking", "inadmissible", "unassuming"},
				Usage:       "<subscriber>",
				Description: "Removes a subscriber's administrative access.",
				Run:         c.cmdAdminUnadmin,
				Save:        true,
			},
		},
	}
}

func (c *Chat) cmdAdminAdmins(h *CommandHandler) (*CommandReply, error) {
	admins := c.Subs.GetAdmins()
	msg := "There are " + strconv.Itoa(len(admins)) + " admins:"
	for i, admin := range admins {
		msg += fmt.Sprintf("\n%v: (%v) %v (%v subscriptions)",
			strconv.Itoa(i+1), admin.API, admin.Contact, admin.Events.Len())
	}
	return &CommandReply{Reply: msg}, nil
}

func (c *Chat) cmdAdminIgnores(h *CommandHandler) (*CommandReply, error) {
	ignores := c.Subs.GetIgnored()
	msg := "There are " + strconv.Itoa(len(ignores)) + " ignored subscribers:"
	for i, ignore := range ignores {
		msg += fmt.Sprintf("\n%v: (%v) %v (%v subscriptions)",
			strconv.Itoa(i+1), ignore.API, ignore.Contact, ignore.Events.Len())
	}
	return &CommandReply{Reply: msg}, nil
}

func (c *Chat) cmdAdminSubs(h *CommandHandler) (*CommandReply, error) {
	if len(h.Text) == 1 {
		subs := c.Subs.Subscribers
		msg := "There are " + strconv.Itoa(len(subs)) + " total subscribers:"
		for i, target := range subs {
			var x string
			if target.Ignored {
				x = ", ignored"
			} else if target.Admin {
				x = ", admin"
			}
			msg += fmt.Sprintf("\n%v: (%v) %v%v (%v subscriptions)",
				strconv.Itoa(i+1), target.API, target.Contact, x, target.Events.Len())
		}
		return &CommandReply{Reply: msg}, nil
	}
	s, err := c.Subs.GetSubscriber(h.Text[1], h.API)
	if err != nil {
		return &CommandReply{Reply: "Subscriber does not exist: " + h.Text[1]}, nil
	}

	subs := s.Events.Names()
	if len(subs) == 0 {
		return &CommandReply{Reply: h.Text[1] + " has no subscriptions."}, nil
	}
	var x string
	if s.Ignored {
		x = " (ignored)"
	} else if s.Admin {
		x = " (admin)"
	}
	msg := s.Contact + x + " has " + strconv.Itoa(len(subs)) + " subscriptions:"
	i := 0
	for _, event := range subs {
		i++
		msg += "\n" + strconv.Itoa(i) + ": " + event
		if s.Events.IsPaused(event) {
			msg += " (paused)"
		}
	}
	return &CommandReply{Reply: msg}, nil
}

func (c *Chat) cmdAdminUnadmin(h *CommandHandler) (*CommandReply, error) {
	if len(h.Text) != 2 {
		return &CommandReply{}, ErrorBadUsage
	}
	target, err := c.Subs.GetSubscriber(h.Text[1], h.API)
	if err != nil {
		return &CommandReply{Reply: "Subscriber does not exist: " + h.Text[1]}, ErrorBadUsage
	}
	target.Admin = false
	return &CommandReply{Reply: "Subscriber '" + target.Contact + "' updated without admin privileges."}, nil
}

func (c *Chat) cmdAdminAdmin(h *CommandHandler) (*CommandReply, error) {
	if len(h.Text) != 2 {
		return &CommandReply{}, ErrorBadUsage
	}
	target, err := c.Subs.GetSubscriber(h.Text[1], h.API)
	if err != nil {
		return &CommandReply{Reply: "Subscriber does not exist: " + h.Text[1]}, ErrorBadUsage
	}
	target.Admin = true
	return &CommandReply{Reply: "Subscriber '" + target.Contact + "' updated with admin privileges."}, nil
}

func (c *Chat) cmdAdminUnignore(h *CommandHandler) (*CommandReply, error) {
	if len(h.Text) != 2 {
		return &CommandReply{}, ErrorBadUsage
	}
	target, err := c.Subs.GetSubscriber(h.Text[1], h.API)
	if err != nil {
		return &CommandReply{Reply: "Subscriber does not exist: " + h.Text[1]}, ErrorBadUsage
	}
	target.Ignored = false
	return &CommandReply{Reply: "Subscriber '" + target.Contact + "' no longer ignored."}, nil
}

func (c *Chat) cmdAdminIgnore(h *CommandHandler) (*CommandReply, error) {
	if len(h.Text) != 2 {
		return &CommandReply{}, ErrorBadUsage
	}
	target, err := c.Subs.GetSubscriber(h.Text[1], h.API)
	if err != nil {
		return &CommandReply{Reply: "Subscriber does not exist: " + h.Text[1]}, ErrorBadUsage
	}
	target.Ignored = true
	target.Admin = false
	return &CommandReply{Reply: "Subscriber '" + target.Contact + "' ignored."}, nil
}
