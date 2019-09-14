package chat

import (
	"fmt"
	"strconv"
)

// AdminCommands contains all the admin commands like 'ignore'
func (c *Chat) AdminCommands() *CommandMap {
	return &CommandMap{
		Kind:  "Admin",
		Level: 10,
		Map: map[string]Command{
			"subs": Command{
				Usage:       "[subscriber]",
				Description: "Displays all subscribers.",
				Run:         c.cmdAdminSubs,
				Save:        false,
			},
			"ignores": Command{
				Description: "Displays all ignored subscribers.",
				Run:         c.cmdAdminIgnores,
				Save:        false,
			},
			"ignore": Command{
				Usage:       "<subscriber>",
				Description: "Ignores a subscriber.",
				Run:         c.cmdAdminIgnore,
				Save:        true,
			},
			"unignore": Command{
				Usage:       "<subscriber>",
				Description: "Removes a subscriber's ignore.",
				Run:         c.cmdAdminUnignore,
				Save:        true,
			},
			"admins": Command{
				Description: "Displays all administrative subscribers.",
				Run:         c.cmdAdminAdmins,
				Save:        false,
			},
			"admin": Command{
				Usage:       "<subscriber>",
				Description: "Gives a subscriber administrative access.",
				Run:         c.cmdAdminAdmin,
				Save:        true,
			},
			"unadmin": Command{
				Usage:       "<subscriber>",
				Description: "Removes a subscriber's administrative access.",
				Run:         c.cmdAdminUnadmin,
				Save:        true,
			},
		},
	}
}

func (c *Chat) cmdAdminAdmins(h *CommandHandle) (string, []string, error) {
	admins := c.Subs.GetAdmins()
	msg := "There are " + strconv.Itoa(len(admins)) + " admins:"
	for i, admin := range admins {
		msg += fmt.Sprintf("\n%v: (%v) %v (%v subscriptions)",
			strconv.Itoa(i+1), admin.API, admin.Contact, admin.Events.Len())
	}
	return msg, nil, nil
}

func (c *Chat) cmdAdminIgnores(h *CommandHandle) (string, []string, error) {
	ignores := c.Subs.GetIgnored()
	msg := "There are " + strconv.Itoa(len(ignores)) + " ignored subscribers:"
	for i, ignore := range ignores {
		msg += fmt.Sprintf("\n%v: (%v) %v (%v subscriptions)",
			strconv.Itoa(i+1), ignore.API, ignore.Contact, ignore.Events.Len())
	}
	return msg, nil, nil
}

func (c *Chat) cmdAdminSubs(h *CommandHandle) (string, []string, error) {
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
		return msg, nil, nil
	}
	s, err := c.Subs.GetSubscriber(h.Text[1], h.API)
	if err != nil {
		return "Subscriber does not exist: " + h.Text[1], nil, nil
	}

	subs := s.Events.Names()
	if len(subs) == 0 {
		return h.Text[1] + " has no subscriptions.", nil, nil
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
	return msg, nil, nil
}

func (c *Chat) cmdAdminUnadmin(h *CommandHandle) (string, []string, error) {
	if len(h.Text) != 2 {
		return "", nil, ErrorBadUsage
	}
	target, err := c.Subs.GetSubscriber(h.Text[1], h.API)
	if err != nil {
		return "Subscriber does not exist: " + h.Text[1], nil, ErrorBadUsage
	}
	target.Admin = false
	return "Subscriber '" + target.Contact + "' updated without admin privileges.", nil, nil
}

func (c *Chat) cmdAdminAdmin(h *CommandHandle) (string, []string, error) {
	if len(h.Text) != 2 {
		return "", nil, ErrorBadUsage
	}
	target, err := c.Subs.GetSubscriber(h.Text[1], h.API)
	if err != nil {
		return "Subscriber does not exist: " + h.Text[1], nil, ErrorBadUsage
	}
	target.Admin = true
	return "Subscriber '" + target.Contact + "' updated with admin privileges.", nil, nil
}

func (c *Chat) cmdAdminUnignore(h *CommandHandle) (string, []string, error) {
	if len(h.Text) != 2 {
		return "", nil, ErrorBadUsage
	}
	target, err := c.Subs.GetSubscriber(h.Text[1], h.API)
	if err != nil {
		return "Subscriber does not exist: " + h.Text[1], nil, ErrorBadUsage
	}
	target.Ignored = false
	return "Subscriber '" + target.Contact + "' no longer ignored.", nil, nil
}

func (c *Chat) cmdAdminIgnore(h *CommandHandle) (string, []string, error) {
	if len(h.Text) != 2 {
		return "", nil, ErrorBadUsage
	}
	target, err := c.Subs.GetSubscriber(h.Text[1], h.API)
	if err != nil {
		return "Subscriber does not exist: " + h.Text[1], nil, ErrorBadUsage
	}
	target.Ignored = true
	target.Admin = false
	return "Subscriber '" + target.Contact + "' ignored.", nil, nil
}
