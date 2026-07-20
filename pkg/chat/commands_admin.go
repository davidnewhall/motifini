package chat

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	subscriberArgRequired = "<subscriber>"
	subscriberArgOptional = "[subscriber]"
)

// adminCommands contains all the built-in admin commands like 'ignore'.
func (c *Chat) adminCommands() *Commands { //nolint:funlen // it's not that bad.
	return &Commands{
		Title: "Admin",
		Level: LevelAdmin,
		List: []*Command{
			{
				Run:  getIP,
				AKA:  []string{"ip"},
				Desc: "Returns public IP from ifconfig.me.",
			},
			{
				Run:  func(_ *Handler) (*Reply, error) { return &Reply{Reply: "Saved"}, c.Subs.StateFileSave() },
				AKA:  []string{"save"},
				Use:  "",
				Desc: "Saves subscriber data to a file.",
			},
			{
				Run:  c.cmdAdminSubs,
				AKA:  []string{"subs", "subscribers"},
				Use:  subscriberArgOptional,
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
				Use:  subscriberArgRequired,
				Desc: "Ignores a subscriber.",
				Save: true,
			},
			{
				Run:  c.cmdAdminUnignore,
				AKA:  []string{"unignore"},
				Use:  subscriberArgRequired,
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
				Use:  subscriberArgRequired,
				Desc: "Gives a subscriber administrative access.",
				Save: true,
			},
			{
				Run:  c.cmdAdminUnadmin,
				AKA:  []string{"unadmin", "unmasking", "inadmissible", "unassuming"},
				Use:  subscriberArgRequired,
				Desc: "Removes a subscriber's administrative access.",
				Save: true,
			},
		},
	}
}

func (c *Chat) cmdAdminAdmins(_ *Handler) (*Reply, error) {
	admins := c.Subs.GetAdmins()

	var msg strings.Builder
	fmt.Fprintf(&msg, "There are %d admins:", len(admins))

	for i, admin := range admins {
		fmt.Fprintf(&msg, "\n%d: (%v) %v (%v) (%d subscriptions)",
			i+1, admin.API, admin.ID, admin.Contact, admin.Events.Len())
	}

	return &Reply{Reply: msg.String()}, nil
}

func (c *Chat) cmdAdminIgnores(_ *Handler) (*Reply, error) {
	ignores := c.Subs.GetIgnored()

	var msg strings.Builder
	fmt.Fprintf(&msg, "There are %d ignored subscribers:", len(ignores))

	for i, ignore := range ignores {
		fmt.Fprintf(&msg, "\n%d: (%v) %v (%v) (%d subscriptions)",
			i+1, ignore.API, ignore.ID, ignore.Contact, ignore.Events.Len())
	}

	return &Reply{Reply: msg.String()}, nil
}

func (c *Chat) cmdAdminSubs(h *Handler) (*Reply, error) { //nolint:cyclop // it's not that bad.
	if len(h.Text) == 1 {
		subs := c.Subs.Subscribers

		var msg strings.Builder
		fmt.Fprintf(&msg, "There are %d total subscribers:", len(subs))

		for i, target := range subs {
			var x string

			if target.Ignored {
				x = ", ignored"
			} else if target.Admin {
				x = ", admin"
			}

			fmt.Fprintf(&msg, "\n%d: (%v) %v (%v)%s (%d subscriptions)",
				i+1, target.API, target.ID, target.Contact, x, target.Events.Len())
		}

		return &Reply{Reply: msg.String()}, nil
	}

	s, err := c.getSubscriber(h.Text[1], h.API)
	if err != nil {
		return &Reply{Reply: "Subscriber does not exist: " + h.Text[1]}, nil //nolint:nilerr // we do not use the error.
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

	var msg strings.Builder
	fmt.Fprintf(&msg, "%s%s has %d subscriptions:", s.Contact, x, len(subs))

	for i, event := range subs {
		fmt.Fprintf(&msg, "\n%d: %s", i+1, event)

		if s.Events.IsPaused(event) {
			until := time.Until(s.Events.PauseTime(event)).Round(time.Second)
			fmt.Fprintf(&msg, ", paused %v", until)
		}

		delay, ok := h.Sub.Events.RuleGetD(event, "delay")
		if ok {
			fmt.Fprintf(&msg, ", delay: %v", delay)
		}
	}

	return &Reply{Reply: msg.String()}, nil
}

func (c *Chat) cmdAdminUnadmin(h *Handler) (*Reply, error) {
	if len(h.Text) != twoItems {
		return &Reply{}, ErrBadUsage
	}

	target, err := c.getSubscriber(h.Text[1], h.API)
	if err != nil {
		return &Reply{Reply: "Subscriber does not exist: " + h.Text[1]}, ErrBadUsage
	}

	target.Admin = false

	return &Reply{Reply: "Subscriber '" + target.Contact + "' updated without admin privileges."}, nil
}

func (c *Chat) cmdAdminAdmin(h *Handler) (*Reply, error) {
	if len(h.Text) != twoItems {
		return &Reply{}, ErrBadUsage
	}

	target, err := c.getSubscriber(h.Text[1], h.API)
	if err != nil {
		return &Reply{Reply: "Subscriber does not exist: " + h.Text[1]}, ErrBadUsage
	}

	target.Admin = true

	return &Reply{Reply: "Subscriber '" + target.Contact + "' updated with admin privileges."}, nil
}

func (c *Chat) cmdAdminUnignore(h *Handler) (*Reply, error) {
	if len(h.Text) != twoItems {
		return &Reply{}, ErrBadUsage
	}

	target, err := c.getSubscriber(h.Text[1], h.API)
	if err != nil {
		return &Reply{Reply: "Subscriber does not exist: " + h.Text[1]}, ErrBadUsage
	}

	target.Ignored = false

	return &Reply{Reply: "Subscriber '" + target.Contact + "' no longer ignored."}, nil
}

func (c *Chat) cmdAdminIgnore(h *Handler) (*Reply, error) {
	if len(h.Text) != twoItems {
		return &Reply{}, ErrBadUsage
	}

	target, err := c.getSubscriber(h.Text[1], h.API)
	if err != nil {
		return &Reply{Reply: "Subscriber does not exist: " + h.Text[1]}, ErrBadUsage
	}

	target.Ignored = true
	target.Admin = false

	return &Reply{Reply: "Subscriber '" + target.Contact + "' ignored."}, nil
}

func getIP(*Handler) (*Reply, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://ifconfig.me", http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating http request: %w", err)
	}

	rep, err := (&http.Client{}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("making http request: %w", err)
	}
	defer rep.Body.Close()

	body, err := io.ReadAll(rep.Body)
	if err != nil {
		return nil, fmt.Errorf("reading http response: %w", err)
	}

	return &Reply{Reply: "Public IP: " + string(body)}, nil
}
