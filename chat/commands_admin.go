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
				Run:  c.cmdAdminSubs,
				AKA:  []string{"subs", "subscribers"},
				Use:  "[subscriber]",
				Desc: "Displays all subscribers.",
			},
			{
				Run:  c.cmdAdminIgnores,
				AKA:  []string{"ignores"},
				Desc: "Displays all ignored subscribers.",
			},
			{
				Run:  c.cmdAdminIgnore,
				AKA:  []string{"ignore"},
				Use:  "<subscriber>",
				Desc: "Ignores a subscriber.",
				Save: true,
			},
			{
				Run:  c.cmdAdminUnignore,
				AKA:  []string{"unignore"},
				Use:  "<subscriber>",
				Desc: "Removes a subscriber's ignore.",
				Save: true,
			},
			{
				Run:  c.cmdAdminAdmins,
				AKA:  []string{"admins"},
				Desc: "Displays all administrative subscribers.",
			},
			{
				Run:  c.cmdAdminAdmin,
				AKA:  []string{"admin"},
				Use:  "<subscriber>",
				Desc: "Gives a subscriber administrative access.",
				Save: true,
			},
			{
				Run:  c.cmdAdminUnadmin,
				AKA:  []string{"unadmin", "unmasking", "inadmissible", "unassuming"},
				Use:  "<subscriber>",
				Desc: "Removes a subscriber's administrative access.",
				Save: true,
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
	r := &CommandReply{Reply: fmt.Sprintf("There are %d ignored subscribers:", len(ignores))}
	for i, ignore := range ignores {
		r.Reply += fmt.Sprintf("\n%v: (%v) %v (%v subscriptions)",
			strconv.Itoa(i+1), ignore.API, ignore.Contact, ignore.Events.Len())
	}
	return r, nil
}

func (c *Chat) cmdAdminSubs(h *CommandHandler) (*CommandReply, error) {
	if len(h.Text) == 1 {
		subs := c.Subs.Subscribers
		r := &CommandReply{Reply: fmt.Sprintf("There are %d total subscribers:", len(subs))}
		for i, target := range subs {
			var x string
			if target.Ignored {
				x = ", ignored"
			} else if target.Admin {
				x = ", admin"
			}
			r.Reply += fmt.Sprintf("\n%v: (%v) %v%v (%v subscriptions)",
				strconv.Itoa(i+1), target.API, target.Contact, x, target.Events.Len())
		}
		return r, nil
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
		x = ", ignored"
	} else if s.Admin {
		x = ", admin"
	}
	r := &CommandReply{Reply: fmt.Sprintf("%s%s has %d subscriptions:", s.Contact, x, len(subs))}
	i := 0
	for _, event := range subs {
		i++
		r.Reply += fmt.Sprintf("\n%d: %s", i, event)
		if s.Events.IsPaused(event) {
			r.Reply += " (paused)"
		}
	}
	return r, nil
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
