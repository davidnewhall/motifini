package chat

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"golift.io/securityspy/v2"
)

// Command args are parsed out by their count, so make it a constant.
const (
	twoItems = iota + 2
	threeItems
)

const (
	maxsize = 1024 * 1024 // 1mb
	length  = 5 * time.Second
	height  = 800
	quality = 20
)

// nonAdminCommands contains all the built-in non-admin commands.
func (c *Chat) nonAdminCommands() *Commands { //nolint:funlen // it's not that bad.
	return &Commands{
		Title: "User",
		Level: LevelUser,
		List: []*Command{
			{
				Run:  c.cmdCams,
				AKA:  []string{"cams", "cam", "cameras"},
				Desc: "Displays all available cameras by name.",
				Save: false,
			},
			{
				Run:  c.cmdEvents,
				AKA:  []string{"events"},
				Desc: "Displays all available events.",
				Save: false,
			},
			{
				Run:  c.cmdSubs,
				AKA:  []string{"subs", "subscribers"},
				Desc: "Shows your subscriptions.",
				Save: false,
			},
			{
				Run:  c.cmdSub,
				AKA:  []string{"sub", "sun"},
				Use:  "<camera|event>",
				Desc: "Enables notifications from <camera> or <event>.",
				Save: true,
			},
			{
				Run:  c.cmdUnsub,
				AKA:  []string{"unsub", "unsung", "unsubscribe", "unsure", "unseen"},
				Use:  "<cam|event>",
				Desc: "Stops notifications from <cam>, or <event> or everything if '*' is passed.",
				Save: true,
			},
			{
				Run:  c.cmdStop,
				AKA:  []string{"stop", "quit", "pause"},
				Use:  "[mins] [camera]",
				Desc: "Stops all motion notifications for 10 minutes or [mins] on all cameras or [camera].",
				Save: true,
			},
			{
				Run:  c.cmdPics,
				AKA:  []string{"pics", "pic", "pictures"},
				Use:  "[camera]",
				Desc: "Sends pictures from all cameras, or from [camera].",
				Save: false,
			},
			{
				Run:  c.cmdVids,
				AKA:  []string{"vid", "vids", "video"},
				Use:  "[camera]",
				Desc: "Sends video from all cameras, or from [camera].",
				Save: false,
			},
			{
				Run: c.cmdDelay,
				AKA: []string{"delay"},
				Use: "<seconds> <event>",
				Desc: "The default delay between notifications from an event is 60 seconds. " +
					"You can adjust that per event using this command.",
				Save: false,
			},
		},
	}
}

func (c *Chat) cmdCams(_ *Handler) (*Reply, error) {
	cams := c.SSpy.Cameras.All()

	var msg strings.Builder
	fmt.Fprintf(&msg, "There are %d cameras:\n", len(cams))

	for _, cam := range cams {
		fmt.Fprintf(&msg, "%v: %v\n", cam.Number, cam.Name)
	}

	return &Reply{Reply: msg.String()}, nil
}

func (c *Chat) cmdEvents(_ *Handler) (*Reply, error) {
	events := c.Subs.Events.Names()

	var msg strings.Builder
	fmt.Fprintf(&msg, "There are %d events:\n", len(events))

	for i, event := range events {
		description, _ := c.Subs.Events.RuleGetS(event, "description")
		fmt.Fprintf(&msg, "%v: %v - %v\n", i, event, description)
	}

	return &Reply{Reply: msg.String()}, nil
}

func (c *Chat) cmdPics(h *Handler) (*Reply, error) {
	msg := ""

	if len(h.Text) > 1 {
		name := strings.Join(h.Text[1:], " ")
		cam := c.SSpy.Cameras.ByName(name)

		if cam == nil {
			return &Reply{Reply: "Unknown Camera: " + name}, ErrBadUsage
		}

		path := fmt.Sprintf("%vchat_command_%v_%v.jpg", c.TempDir, h.ID, cam.Name)
		if err := cam.SaveJPEG(&securityspy.VidOps{}, path); err != nil {
			log.Printf("[ERROR] [%v] cam.SaveJPEG: %v", h.ID, err)
			msg = "Error Getting '" + cam.Name + "' Picture: " + err.Error()
		}

		return &Reply{Reply: msg, Files: []string{path}}, nil
	}

	var (
		paths = []string{}
		wg    sync.WaitGroup
	)

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

	return &Reply{Reply: msg, Files: paths}, nil
}

func (c *Chat) cmdVids(h *Handler) (*Reply, error) {
	msg := ""
	ops := &securityspy.VidOps{Height: height, Quality: quality, ACodec: "ulaw"}

	if len(h.Text) > 1 {
		name := strings.Join(h.Text[1:], " ")
		cam := c.SSpy.Cameras.ByName(name)

		if cam == nil {
			return &Reply{Reply: "Unknown Camera: " + name}, ErrBadUsage
		}

		path := fmt.Sprintf("%vchat_command_%v_%v.mp4", c.TempDir, h.ID, cam.Name)
		if err := cam.SaveVideo(ops, length, maxsize, path); err != nil {
			log.Printf("[ERROR] [%v] cam.SaveVideo: %v", h.ID, err)
			msg = "Error Getting '" + cam.Name + "' Video: " + err.Error()
		}

		return &Reply{Reply: msg, Files: []string{path}}, nil
	}

	var (
		paths = []string{}
		wg    sync.WaitGroup
	)

	for _, cam := range c.SSpy.Cameras.All() {
		wg.Add(1)

		go func(cam *securityspy.Camera) {
			defer wg.Done()

			path := fmt.Sprintf("%vchat_command_%v_%v.mp4", c.TempDir, h.ID, cam.Name)

			if err := cam.SaveVideo(ops, length, maxsize, path); err != nil {
				log.Printf("[ERROR] [%v] cam.SaveVideo: %v", h.ID, err)
				msg += "Error Getting '" + cam.Name + "' Video: " + err.Error() + "\n"

				return
			}

			paths = append(paths, path)
		}(cam)
	}

	wg.Wait()

	return &Reply{Reply: msg, Files: paths}, nil
}

func (c *Chat) cmdSub(h *Handler) (*Reply, error) {
	kind := "event"

	if len(h.Text) < twoItems {
		return &Reply{Reply: "must provide an event or camera name to subscribe"}, ErrBadUsage
	}

	event := strings.Join(h.Text[1:], " ")

	if !c.Subs.Events.Exists(event) {
		kind = "camera"

		cam := c.SSpy.Cameras.ByName(event)
		if cam == nil {
			return &Reply{Reply: "Event or Camera not found: " + event}, ErrBadUsage
		}

		event = cam.Name
	}

	msg := "You've been subscribed to " + kind + ": " + event

	if err := h.Sub.Subscribe(event); err != nil {
		msg = "You're already subscribed to: " + kind + ": " + event
	}

	msg += "\nYou have " + strconv.Itoa(h.Sub.Events.Len()) + " event subscriptions."

	return &Reply{Reply: msg}, nil
}

func (c *Chat) cmdSubs(h *Handler) (*Reply, error) {
	if h.Sub.Admin && len(h.Text) > 1 {
		// admin asking for subs for someone else.
		return nil, nil //nolint:nilnil // handled by iMessageAdminSubs()
	}

	var msg strings.Builder
	msg.WriteString("Your Subscriptions:")

	for i, event := range h.Sub.Events.Names() {
		fmt.Fprintf(&msg, "\n%v: %v", i, event)

		if h.Sub.Events.IsPaused(event) {
			until := time.Until(h.Sub.Events.PauseTime(event)).Round(time.Second)
			fmt.Fprintf(&msg, ", paused %v", until)
		}

		delay, ok := h.Sub.Events.RuleGetD(event, "delay")
		if ok {
			fmt.Fprintf(&msg, ", delay: %v", delay)
		}
	}

	msg.WriteString("\n")

	if h.Sub.Events.Len() == 0 {
		msg.WriteString("(none)\n")
	}

	return &Reply{Reply: msg.String()}, nil
}

func (c *Chat) cmdUnsub(h *Handler) (*Reply, error) {
	if len(h.Text) < twoItems {
		return &Reply{Reply: "must provide an event or camera name to unsubscribe"}, ErrBadUsage
	}

	event := strings.Join(h.Text[1:], " ")

	if event == "*" {
		for _, e := range h.Sub.Events.Names() {
			h.Sub.Events.Remove(e)
		}

		return &Reply{Reply: "You've been unsubscribed from all events."}, nil
	}

	var msg strings.Builder

	if name := h.Sub.Events.Name(event); name == "" {
		fmt.Fprintf(&msg, "You're not subscribed to: %s", event)
	} else {
		event = name
		fmt.Fprintf(&msg, "You've been unsubscribed from: %s", event)
	}

	h.Sub.Events.Remove(event)
	fmt.Fprintf(&msg, "\nYou have %d event subscriptions.", h.Sub.Events.Len())

	return &Reply{Reply: msg.String()}, nil
}

func (c *Chat) cmdStop(h *Handler) (*Reply, error) {
	if len(h.Text) == 1 {
		h.Text = append(h.Text, "10") // default pause time of 10 minutes.
	}

	dur, err := strconv.Atoi(h.Text[1])
	if err != nil {
		return &Reply{Reply: "Unable to parse into a number: " + h.Text[1]}, ErrBadUsage
	}

	// Pause a single event.
	if len(h.Text) > twoItems {
		event := strings.Join(h.Text[twoItems:], " ")
		if name := h.Sub.Events.Name(event); name != "" {
			event = name
		}

		msg := "Notifications from '" + event + "' paused for at least " + h.Text[1] + " minutes."

		if dur == 0 {
			msg = "Notifications from '" + event + " are no longer paused."
		}

		if err := h.Sub.Events.Pause(event, time.Duration(dur)*time.Minute); err != nil {
			msg = "You're not subscribed to: " + event
		}

		return &Reply{Reply: msg}, nil
	}

	// Pause Everything.
	for _, event := range h.Sub.Events.Names() {
		_ = h.Sub.Events.Pause(event, time.Duration(dur)*time.Minute)
	}

	msg := "Notifications paused for at least " + h.Text[1] + " minutes."

	if dur == 0 {
		msg = "Notifications are no longer paused."
	}

	return &Reply{Reply: msg}, nil
}

func (c *Chat) cmdDelay(h *Handler) (*Reply, error) {
	if len(h.Text) < threeItems {
		return &Reply{Reply: "must provide <seconds> as a number and the event or camera name"}, ErrBadUsage
	}

	dur, err := strconv.Atoi(h.Text[1])
	if err != nil {
		return &Reply{Reply: "Unable to parse into a number: " + h.Text[1]}, ErrBadUsage
	}

	event := strings.Join(h.Text[twoItems:], " ")

	name := h.Sub.Events.Name(event)
	if name == "" {
		return &Reply{Reply: "You are not subscribed to: " + event}, nil
	}

	event = name

	h.Sub.Events.RuleSetD(event, "delay", time.Duration(dur)*time.Second)

	return &Reply{Reply: fmt.Sprintf("Set repeat delay for '%s' to %ds", event, dur)}, nil
}
