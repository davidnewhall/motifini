package chat

import (
	"fmt"
	"strconv"
)

// AdminCommands contains all the admin commands like 'ignore'
var AdminCommands = commandMap{
	Kind: "Admin",
	Map: map[string]chatCommands{
		"subs": chatCommands{
			Usage:       "[subscriber]",
			Description: "Displays all subscribers.",
			Run:         cmdAdminSubs,
			Save:        false,
		},
		"ignores": chatCommands{
			Description: "Displays all ignored subscribers.",
			Run:         cmdAdminIgnores,
			Save:        false,
		},
		"ignore": chatCommands{
			Usage:       "<subscriber>",
			Description: "Ignores a subscriber.",
			Run:         cmdAdminIgnore,
			Save:        true,
		},
		"unignore": chatCommands{
			Usage:       "<subscriber>",
			Description: "Removes a subscriber's ignore.",
			Run:         cmdAdminUnignore,
			Save:        true,
		},
		"admins": chatCommands{
			Description: "Displays all administrative subscribers.",
			Run:         cmdAdminAdmins,
			Save:        false,
		},
		"admin": chatCommands{
			Usage:       "<subscriber>",
			Description: "Gives a subscriber administrative access.",
			Run:         cmdAdminAdmin,
			Save:        true,
		},
		"unadmin": chatCommands{
			Usage:       "<subscriber>",
			Description: "Removes a subscriber's administrative access.",
			Run:         cmdAdminUnadmin,
			Save:        true,
		},
	},
}

func cmdAdminAdmins(c *CommandHandle) (string, []string, error) {
	admins := Subs.GetAdmins()
	msg := "There are " + strconv.Itoa(len(admins)) + " admins:"
	for i, admin := range admins {
		msg += fmt.Sprintf("\n%v: (%v) %v (%v subscriptions)",
			strconv.Itoa(i+1), admin.API, admin.Contact, admin.Events.Len())
	}
	return msg, nil, nil
}

func cmdAdminIgnores(c *CommandHandle) (string, []string, error) {
	ignores := Subs.GetIgnored()
	msg := "There are " + strconv.Itoa(len(ignores)) + " ignored subscribers:"
	for i, ignore := range ignores {
		msg += fmt.Sprintf("\n%v: (%v) %v (%v subscriptions)",
			strconv.Itoa(i+1), ignore.API, ignore.Contact, ignore.Events.Len())
	}
	return msg, nil, nil
}

func cmdAdminSubs(c *CommandHandle) (string, []string, error) {
	if len(c.Text) == 1 {
		subs := Subs.Subscribers
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
	s, err := Subs.GetSubscriber(c.Text[1], c.API)
	if err != nil {
		return "Subscriber does not exist: " + c.Text[1], nil, nil
	}

	subs := s.Events.Names()
	if len(subs) == 0 {
		return c.Text[1] + " has no subscriptions.", nil, nil
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

func cmdAdminUnadmin(c *CommandHandle) (string, []string, error) {
	if len(c.Text) != 2 {
		return "", nil, ErrorBadUsage
	}
	target, err := Subs.GetSubscriber(c.Text[1], c.API)
	if err != nil {
		return "Subscriber does not exist: " + c.Text[1], nil, ErrorBadUsage
	}
	target.Admin = false
	return "Subscriber '" + target.Contact + "' updated without admin privileges.", nil, nil
}

func cmdAdminAdmin(c *CommandHandle) (string, []string, error) {
	if len(c.Text) != 2 {
		return "", nil, ErrorBadUsage
	}
	target, err := Subs.GetSubscriber(c.Text[1], c.API)
	if err != nil {
		return "Subscriber does not exist: " + c.Text[1], nil, ErrorBadUsage
	}
	target.Admin = true
	return "Subscriber '" + target.Contact + "' updated with admin privileges.", nil, nil
}

func cmdAdminUnignore(c *CommandHandle) (string, []string, error) {
	if len(c.Text) != 2 {
		return "", nil, ErrorBadUsage
	}
	target, err := Subs.GetSubscriber(c.Text[1], c.API)
	if err != nil {
		return "Subscriber does not exist: " + c.Text[1], nil, ErrorBadUsage
	}
	target.Ignored = false
	return "Subscriber '" + target.Contact + "' no longer ignored.", nil, nil
}

func cmdAdminIgnore(c *CommandHandle) (string, []string, error) {
	if len(c.Text) != 2 {
		return "", nil, ErrorBadUsage
	}
	target, err := Subs.GetSubscriber(c.Text[1], c.API)
	if err != nil {
		return "Subscriber does not exist: " + c.Text[1], nil, ErrorBadUsage
	}
	target.Ignored = true
	target.Admin = false
	return "Subscriber '" + target.Contact + "' ignored.", nil, nil
}
