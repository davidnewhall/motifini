package chat

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"golift.io/securityspy"
)

// NonAdminCommands contains all the built-in non-admin commands.
func (c *Chat) NonAdminCommands() *CommandMap {
	return &CommandMap{
		Title: "User",
		Level: 1,
		List: []*Command{
			{
				Aliases:     []string{"cams", "cameras"},
				Description: "Displays all available cameras by name.",
				Run:         c.cmdCams,
				Save:        false,
			},
			{
				Aliases:     []string{"events"},
				Description: "Displays all available events.",
				Run:         c.cmdEvents,
				Save:        false,
			},
			{
				Aliases:     []string{"subs", "subscribers"},
				Description: "Shows your subscriptions.",
				Run:         c.cmdSubs,
				Save:        false,
			},
			{
				Aliases:     []string{"sub", "sun"},
				Usage:       "<camera|event>",
				Description: "Enables notifications from <camera> or <event>.",
				Run:         c.cmdSub,
				Save:        true,
			},
			{
				Aliases:     []string{"unsub", "unsung", "unsubscribe", "unsure", "unseen"},
				Usage:       "<cam|event>",
				Description: "Stops notifications from <cam>, or <event> or everything if '*' is passed.",
				Run:         c.cmdUnsub,
				Save:        true,
			},
			{
				Aliases:     []string{"stop", "quit", "pause"},
				Usage:       "[mins] [camera]",
				Description: "Stops all motion notifications for 10 minutes or [mins] on all cameras or [camera].",
				Run:         c.cmdStop,
				Save:        true,
			},
			{
				Aliases:     []string{"pics", "pictures"},
				Usage:       "[camera]",
				Description: "Sends pictures from all cameras, or from [camera].",
				Run:         c.cmdPics,
				Save:        false,
			},
		},
	}
}

func (c *Chat) cmdCams(h *CommandHandler) (*CommandReply, error) {
	msg := "There are " + strconv.Itoa(len(c.SSpy.Cameras.All())) + " cameras:\n"
	for _, cam := range c.SSpy.Cameras.All() {
		msg += fmt.Sprintf("%v: %v\n", cam.Number, cam.Name)
	}
	return &CommandReply{Reply: msg}, nil
}

func (c *Chat) cmdEvents(h *CommandHandler) (*CommandReply, error) {
	events := c.Subs.Events.Names()
	msg := "There are " + strconv.Itoa(len(events)) + " events:\n"
	for i, event := range events {
		description, _ := c.Subs.Events.RuleGetS(event, "description")
		msg += strconv.Itoa(i) + ": " + event + " - " + description + "\n"
	}
	return &CommandReply{Reply: msg}, nil
}

func (c *Chat) cmdPics(h *CommandHandler) (*CommandReply, error) {
	msg := ""
	if len(h.Text) > 1 {
		name := strings.Join(h.Text[1:], " ")
		cam := c.SSpy.Cameras.ByName(name)
		if cam == nil {
			return &CommandReply{Reply: "Unknown Camera: " + name}, ErrorBadUsage
		}
		path := fmt.Sprintf("%vchat_command_%v_%v.jpg", c.TempDir, h.ID, cam.Name)
		if err := cam.SaveJPEG(&securityspy.VidOps{}, path); err != nil {
			log.Printf("[ERROR] [%v] cam.SaveJPEG: %v", h.ID, err)
			msg = "Error Getting '" + name + "' Picture: " + err.Error()
		}
		return &CommandReply{Reply: msg, Files: []string{path}}, nil
	}
	paths := []string{}
	var wg sync.WaitGroup
	for _, cam := range c.SSpy.Cameras.All() {
		wg.Add(1)
		go func(cam *securityspy.Camera) {
			defer wg.Done()
			path := fmt.Sprintf("%vchat_command_%v_%v.jpg", c.TempDir, h.ID, cam.Name)
			if err := cam.SaveJPEG(&securityspy.VidOps{}, path); err != nil {
				log.Printf("[ERROR] [%v] cam.SaveJPEG: %v", h.ID, err)
				msg += "Error Getting '" + cam.Name + "' Picture: " + err.Error() + "\n"
				return
			}
			paths = append(paths, path)
		}(cam)
	}
	wg.Wait()
	return &CommandReply{Reply: msg, Files: paths}, nil
}

func (c *Chat) cmdSub(h *CommandHandler) (*CommandReply, error) {
	kind := "event"
	if len(h.Text) < 2 {
		return nil, ErrorBadUsage
	}
	event := strings.Join(h.Text[1:], " ")
	if !c.Subs.Events.Exists(event) {
		kind = "camera"
		if cam := c.SSpy.Cameras.ByName(event); cam == nil {
			return &CommandReply{Reply: "Event or Camera not found: " + event}, ErrorBadUsage
		}
	}
	msg := "You've been subscribed to " + kind + ": " + event
	if err := h.Sub.Subscribe(event); err != nil {
		msg = "You're already subscribed to: " + kind + ": " + event
	}
	msg += "\nYou have " + strconv.Itoa(h.Sub.Events.Len()) + " event subscriptions."
	return &CommandReply{Reply: msg}, nil
}

func (c *Chat) cmdSubs(h *CommandHandler) (*CommandReply, error) {
	if h.Sub.Admin && len(h.Text) > 1 {
		// admin asking for subs for someone else. handled by iMessageAdminSubs()
		return nil, nil
	}
	msg := "Your Subscriptions:"
	for i, event := range h.Sub.Events.Names() {
		msg += "\n" + strconv.Itoa(i) + ": " + event
		if h.Sub.Events.IsPaused(event) {
			msg += " (paused)"
		}
	}
	if msg += "\n"; h.Sub.Events.Len() == 0 {
		msg += "(none)\n"
	}
	return &CommandReply{Reply: msg}, nil
}

func (c *Chat) cmdUnsub(h *CommandHandler) (*CommandReply, error) {
	if len(h.Text) < 2 {
		return nil, ErrorBadUsage
	}
	event := strings.Join(h.Text[1:], " ")
	msg := "You've been unsubscribed from: " + event
	if event == "*" {
		for _, e := range h.Sub.Events.Names() {
			h.Sub.Events.Remove(e)
		}
		return &CommandReply{Reply: "You've been unsubscribed all events."}, nil
	}
	if !h.Sub.Events.Exists(event) {
		msg = "You're not subscribed to: " + event
	}
	h.Sub.Events.Remove(event)
	msg += "\nYou have " + strconv.Itoa(h.Sub.Events.Len()) + " event subscriptions."
	return &CommandReply{Reply: msg}, nil
}

func (c *Chat) cmdStop(h *CommandHandler) (*CommandReply, error) {
	if len(h.Text) < 2 {
		return nil, ErrorBadUsage
	}
	dur, err := strconv.Atoi(h.Text[1])
	if err != nil {
		return &CommandReply{Reply: "Unable to parse into a number: " + h.Text[1]}, ErrorBadUsage
	}

	// Pause a single event.
	if len(h.Text) > 2 {
		event := strings.Join(h.Text[2:], " ")
		msg := "Notifications from '" + event + "' paused for at least " + h.Text[1] + " minutes."
		if dur == 0 {
			msg = "Notifications from '" + event + " are no longer paused."
		}
		if err := h.Sub.Events.Pause(event, time.Duration(dur)*time.Minute); err != nil {
			msg = "You're not subscribed to: " + event
		}
		return &CommandReply{Reply: msg}, nil
	}

	// Pause Everything.
	for _, event := range h.Sub.Events.Names() {
		_ = h.Sub.Events.Pause(event, time.Duration(dur)*time.Minute)
	}
	msg := "Notifications paused for at least " + h.Text[1] + " minutes."
	if dur == 0 {
		msg = "Notifications are no longer paused."
	}
	return &CommandReply{Reply: msg}, nil
}
