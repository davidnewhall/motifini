package chat

import (
	"fmt"
	"strconv"
	"time"
)

// adminCommands contains all the built-in admin commands like 'ignore'.
func (c *Chat) adminCommands() *Commands {
	return &Commands{
		Title: "Admin",
		Level: LevelAdmin,
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

func (c *Chat) cmdAdminAdmins(h *Handler) (*Reply, error) {
	admins := c.Subs.GetAdmins()
	msg := "There are " + strconv.Itoa(len(admins)) + " admins:"

	for i, admin := range admins {
		msg += fmt.Sprintf("\n%v: (%v) %v (%v subscriptions)",
			strconv.Itoa(i+1), admin.API, admin.Contact, admin.Events.Len())
	}

	return &Reply{Reply: msg}, nil
}

func (c *Chat) cmdAdminIgnores(h *Handler) (*Reply, error) {
	ignores := c.Subs.GetIgnored()
	r := &Reply{Reply: fmt.Sprintf("There are %d ignored subscribers:", len(ignores))}

	for i, ignore := range ignores {
		r.Reply += fmt.Sprintf("\n%v: (%v) %v (%v subscriptions)",
			strconv.Itoa(i+1), ignore.API, ignore.Contact, ignore.Events.Len())
	}

	return r, nil
}

func (c *Chat) cmdAdminSubs(h *Handler) (*Reply, error) { //nolint:cyclop
	if len(h.Text) == 1 {
		subs := c.Subs.Subscribers
		r := &Reply{Reply: fmt.Sprintf("There are %d total subscribers:", len(subs))}

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
		return &Reply{Reply: "Subscriber does not exist: " + h.Text[1]}, nil // nolint:nilerr
	}

	subs := s.Events.Names()
	if len(subs) == 0 {
		return &Reply{Reply: h.Text[1] + " has no subscriptions."}, nil
	}

	var x string

	if s.Ignored {
		x = ", ignored"
	} else if s.Admin {
		x = ", admin"
	}

	r := &Reply{Reply: fmt.Sprintf("%s%s has %d subscriptions:", s.Contact, x, len(subs))}
	i := 0

	for _, event := range subs {
		i++
		r.Reply += fmt.Sprintf("\n%d: %s", i, event)

		if s.Events.IsPaused(event) {
			until := time.Until(s.Events.PauseTime(event)).Round(time.Second)
			r.Reply += fmt.Sprintf(", paused %v", until)
		}

		delay, ok := h.Sub.Events.RuleGetD(event, "delay")
		if ok {
			r.Reply += fmt.Sprintf(", delay: %v", delay)
		}
	}

	return r, nil
}

func (c *Chat) cmdAdminUnadmin(h *Handler) (*Reply, error) {
	if len(h.Text) != twoItems {
		return &Reply{}, ErrorBadUsage
	}

	target, err := c.Subs.GetSubscriber(h.Text[1], h.API)
	if err != nil {
		return &Reply{Reply: "Subscriber does not exist: " + h.Text[1]}, ErrorBadUsage
	}

	target.Admin = false

	return &Reply{Reply: "Subscriber '" + target.Contact + "' updated without admin privileges."}, nil
}

func (c *Chat) cmdAdminAdmin(h *Handler) (*Reply, error) {
	if len(h.Text) != twoItems {
		return &Reply{}, ErrorBadUsage
	}

	target, err := c.Subs.GetSubscriber(h.Text[1], h.API)
	if err != nil {
		return &Reply{Reply: "Subscriber does not exist: " + h.Text[1]}, ErrorBadUsage
	}

	target.Admin = true

	return &Reply{Reply: "Subscriber '" + target.Contact + "' updated with admin privileges."}, nil
}

func (c *Chat) cmdAdminUnignore(h *Handler) (*Reply, error) {
	if len(h.Text) != twoItems {
		return &Reply{}, ErrorBadUsage
	}

	target, err := c.Subs.GetSubscriber(h.Text[1], h.API)
	if err != nil {
		return &Reply{Reply: "Subscriber does not exist: " + h.Text[1]}, ErrorBadUsage
	}

	target.Ignored = false

	return &Reply{Reply: "Subscriber '" + target.Contact + "' no longer ignored."}, nil
}

func (c *Chat) cmdAdminIgnore(h *Handler) (*Reply, error) {
	if len(h.Text) != twoItems {
		return &Reply{}, ErrorBadUsage
	}

	target, err := c.Subs.GetSubscriber(h.Text[1], h.API)
	if err != nil {
		return &Reply{Reply: "Subscriber does not exist: " + h.Text[1]}, ErrorBadUsage
	}

	target.Ignored = true
	target.Admin = false

	return &Reply{Reply: "Subscriber '" + target.Contact + "' ignored."}, nil
}
