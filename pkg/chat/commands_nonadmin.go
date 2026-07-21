package chat

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"golift.io/securityspy/v2"
)

// Command args are parsed out by their count, so make it a constant.
const (
	twoItems = iota + 2
	threeItems
)

const (
	maxsize    = 1024 * 1024 // 1mb
	length     = 5 * time.Second
	height     = 720 // SS RTSP resize; 800 often stalls to ~1fps under load
	jpegHeight = 1080
	quality    = 20 // JPEG only; stripped from RTSP by securityspy
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
				Desc: "Browse cameras — tap for snapshot or video.",
				Save: false,
			},
			{
				Run:  c.cmdEvents,
				AKA:  []string{"events"},
				Desc: "List events — tap to subscribe.",
				Save: false,
			},
			{
				Run:  c.cmdSubs,
				AKA:  []string{"subs", "subscribers"},
				Desc: "Your subscriptions with manage / pause / delay shortcuts.",
				Save: false,
			},
			{
				Run:  c.cmdSub,
				AKA:  []string{"sub", "sun"},
				Use:  "[camera class|event|camera:class]",
				Desc: "Subscribe via menu, or text: /sub Office human",
				Save: true,
			},
			{
				Run:  c.cmdUnsub,
				AKA:  []string{"unsub", "unsung", "unsubscribe", "unsure", "unseen"},
				Use:  "[cam|event|*]",
				Desc: "Unsubscribe via menu, or text: /unsub Office · /unsub * clears all",
				Save: true,
			},
			{
				Run:  c.cmdStop,
				AKA:  []string{"stop", "quit", "pause"},
				Use:  "[mins] [camera]",
				Desc: "Pause alerts via menu, or text: /stop 10 · /stop 10 Office",
				Save: true,
			},
			{
				Run:  c.cmdPics,
				AKA:  []string{"pics", "pic", "pictures"},
				Use:  "[camera]",
				Desc: "Snapshot via menu, or text: /pics Office",
				Save: false,
			},
			{
				Run:  c.cmdVids,
				AKA:  []string{"vid", "vids", "video"},
				Use:  "[camera]",
				Desc: "Video clip via menu, or text: /vid Office",
				Save: false,
			},
			{
				Run:  c.cmdDelay,
				AKA:  []string{"delay"},
				Use:  "[seconds] [event]",
				Desc: "Set repeat delay via menu, or text: /delay 60 Office:human",
				Save: true,
			},
		},
	}
}

func (c *Chat) cmdCams(_ *Handler) (*Reply, error) {
	root := c.camsWizardRoot()
	root.Edit = false

	return root, nil
}

func (c *Chat) cmdEvents(_ *Handler) (*Reply, error) {
	root := c.eventsWizardRoot()
	root.Edit = false

	return root, nil
}

func (c *Chat) cmdPics(handler *Handler) (*Reply, error) {
	if len(handler.Text) > 1 {
		name := strings.Join(handler.Text[1:], " ")
		cam := c.SSpy.Cameras.ByName(name)

		if cam == nil {
			return &Reply{Reply: "Unknown Camera: " + name}, ErrBadUsage
		}

		path, errMsg := c.snapOne(handler, cam, false)
		files := []string{}
		if path != "" {
			files = []string{path}
		}

		return &Reply{Reply: nonEmpty(errMsg, CameraCaption(cam.Name, CaptionPhoto)), Files: files}, nil
	}

	root := c.picsWizardRoot()
	root.Edit = false

	return root, nil
}

func clipVidOps(cam *securityspy.Camera) *securityspy.VidOps {
	ops := VideoClipOps(cam, height)
	ops.Quality = quality // JPEG only; stripped from RTSP by securityspy

	return ops
}

// VideoClipOps builds RTSP remux options using the camera's native codec, scaled to targetHeight.
// Width is derived from the camera aspect ratio.
//
// SecuritySpy stream-copies native HEVC (ignoring height) when the requested height is
// >= half the camera's native height — e.g. a camera with 1440p (2560x1440) stays 2560x1440 at height=720,
// but recompresses at height=719. A camera with 1728p (2560x1728) still scales at 720 because 720 < 864.
func VideoClipOps(cam *securityspy.Camera, targetHeight int) *securityspy.VidOps {
	ops := &securityspy.VidOps{
		Height: targetHeight,
		ACodec: "aac",
	}
	if cam == nil {
		return ops
	}

	ops.VCodec = cam.PreferredVCodec()
	ops.Height = scaledStreamHeight(cam.Height, targetHeight)

	if cam.Width > 0 && cam.Height > 0 && ops.Height > 0 {
		w := cam.Width * ops.Height / cam.Height
		ops.Width = w - w%2 // encoder-friendly even width
	}

	return ops
}

// scaledStreamHeight keeps the request strictly below half the native height so
// SecuritySpy recompresses HEVC instead of stream-copying the main stream.
func scaledStreamHeight(nativeHeight, targetHeight int) int {
	if targetHeight < 2 {
		return targetHeight
	}

	height := targetHeight
	if nativeHeight > 0 && height*2 >= nativeHeight {
		height = nativeHeight/2 - 1
	}

	if height%2 != 0 {
		height--
	}

	if height < 2 {
		return targetHeight
	}

	return height
}

func (c *Chat) cmdVids(handler *Handler) (*Reply, error) {
	if len(handler.Text) > 1 {
		name := strings.Join(handler.Text[1:], " ")
		cam := c.SSpy.Cameras.ByName(name)

		if cam == nil {
			return &Reply{Reply: "Unknown Camera: " + name}, ErrBadUsage
		}

		path, errMsg := c.snapOne(handler, cam, true)
		files := []string{}
		if path != "" {
			files = []string{path}
		}

		return &Reply{Reply: nonEmpty(errMsg, CameraCaption(cam.Name, CaptionVideo)), Files: files}, nil
	}

	root := c.vidsWizardRoot()
	root.Edit = false

	return root, nil
}

func (c *Chat) cmdSub(handler *Handler) (*Reply, error) {
	if len(handler.Text) < twoItems {
		root := c.subWizardRoot()
		root.Edit = false // first message, not an edit

		return root, nil
	}

	key, kind, err := c.resolveSubTarget(handler.Text[1:])
	if err != nil {
		return &Reply{Reply: err.Error()}, ErrBadUsage
	}

	msg := "You've been subscribed to " + kind + ": " + formatSubLabel(key)

	err = handler.Sub.Subscribe(key)
	if err != nil {
		msg = "You're already subscribed to " + kind + ": " + formatSubLabel(key)
	}

	msg += "\nYou have " + strconv.Itoa(handler.Sub.Events.Len()) + " event subscriptions."

	return &Reply{Reply: msg}, nil
}

func (c *Chat) cmdSubs(handler *Handler) (*Reply, error) {
	if handler.Sub.Admin && len(handler.Text) > 1 {
		// admin asking for subs for someone else.
		return nil, nil //nolint:nilnil // handled by cmdAdminSubs()
	}

	root := c.subsWizardRoot(handler)
	root.Edit = false

	return root, nil
}

func (c *Chat) cmdUnsub(handler *Handler) (*Reply, error) {
	if len(handler.Text) < twoItems {
		root := c.unsubWizardRoot(handler)
		root.Edit = false

		return root, nil
	}

	event := strings.Join(handler.Text[1:], " ")

	if event == "*" {
		for _, e := range handler.Sub.Events.Names() {
			handler.Sub.Events.Remove(e)
		}

		return &Reply{Reply: "You've been unsubscribed from all events."}, nil
	}

	// Allow "Office human" / "Office:human" forms.
	key, _, err := c.resolveSubTarget(handler.Text[1:])
	if err == nil {
		if name := handler.Sub.Events.Name(key); name != "" {
			event = name
		} else {
			event = key
		}
	}

	var msg strings.Builder

	if name := handler.Sub.Events.Name(event); name == "" {
		fmt.Fprintf(&msg, "You're not subscribed to: %s", formatSubLabel(event))
	} else {
		event = name
		fmt.Fprintf(&msg, "You've been unsubscribed from: %s", formatSubLabel(event))
	}

	handler.Sub.Events.Remove(event)
	fmt.Fprintf(&msg, "\nYou have %d event subscriptions.", handler.Sub.Events.Len())

	return &Reply{Reply: msg.String()}, nil
}

func (c *Chat) cmdStop(handler *Handler) (*Reply, error) {
	if len(handler.Text) == 1 {
		root := c.stopWizardRoot()
		root.Edit = false

		return root, nil
	}

	dur, err := strconv.Atoi(handler.Text[1])
	if err != nil {
		return &Reply{Reply: "Unable to parse into a number: " + handler.Text[1]}, ErrBadUsage
	}

	if dur < 0 || dur > MaxPauseMinutes {
		return &Reply{Reply: fmt.Sprintf("Pause must be 0–%d minutes (24 hours).", MaxPauseMinutes)}, ErrBadUsage
	}

	// Pause a single event.
	if len(handler.Text) > twoItems {
		event := strings.Join(handler.Text[twoItems:], " ")
		if name := handler.Sub.Events.Name(event); name != "" {
			event = name
		}

		msg := "Notifications from '" + event + "' paused for at least " + handler.Text[1] + " minutes."

		if dur == 0 {
			msg = "Notifications from '" + event + " are no longer paused."
		}

		err := handler.Sub.Events.Pause(event, time.Duration(dur)*time.Minute)
		if err != nil {
			msg = "You're not subscribed to: " + event
		}

		return &Reply{Reply: msg}, nil
	}

	// Pause Everything.
	for _, event := range handler.Sub.Events.Names() {
		_ = handler.Sub.Events.Pause(event, time.Duration(dur)*time.Minute)
	}

	msg := "Notifications paused for at least " + handler.Text[1] + " minutes."

	if dur == 0 {
		msg = "Notifications are no longer paused."
	}

	return &Reply{Reply: msg}, nil
}

func (c *Chat) cmdDelay(handler *Handler) (*Reply, error) {
	if len(handler.Text) < threeItems {
		root := c.delayWizardRoot(handler)
		root.Edit = false

		return root, nil
	}

	dur, err := strconv.Atoi(handler.Text[1])
	if err != nil {
		return &Reply{Reply: "Unable to parse into a number: " + handler.Text[1]}, ErrBadUsage
	}

	event := strings.Join(handler.Text[twoItems:], " ")

	name := handler.Sub.Events.Name(event)
	if name == "" {
		return &Reply{Reply: "You are not subscribed to: " + event}, nil
	}

	event = name

	handler.Sub.Events.RuleSetD(event, "delay", time.Duration(dur)*time.Second)

	return &Reply{Reply: fmt.Sprintf("Set repeat delay for '%s' to %s",
		event, formatDuration(time.Duration(dur)*time.Second))}, nil
}
