package chat

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"golift.io/subscribe"
)

// Inline callback payloads (keep under Telegram's 64-byte limit).
const (
	cbSubRoot   = "s"
	cbSubCam    = "s:c"
	cbSubEvt    = "s:e"
	cbSubClass  = "s:k:" // + class short (*|m|h|v|a)
	cbUnsubRoot = "u"
	cbCancel    = "x"
)

func (c *Chat) handleWizardCallback(handler *Handler) (*Reply, bool) {
	data := handler.Callback
	if data == "" && len(handler.Text) > 0 {
		data = handler.Text[0]
	}

	if data == cbCancel {
		clearPendingRename(handler.Sub)

		return &Reply{Reply: "Done.", Edit: true, Toast: "OK"}, true
	}

	if reply, save, ok := c.handleSubUnsubWizardCallback(handler, data); ok {
		return reply, save
	}

	if reply, save, ok := c.handleCommandWizardCallback(handler, data); ok {
		return reply, save
	}

	return &Reply{Reply: "Unknown menu action.", Edit: true, Toast: "??"}, false
}

func (c *Chat) handleSubUnsubWizardCallback(handler *Handler, data string) (*Reply, bool, bool) {
	switch {
	case data == cbSubRoot:
		return c.subWizardRoot(), false, true
	case data == cbSubCam:
		return c.subWizardClasses(), false, true
	case data == cbSubEvt:
		return c.subWizardEvents(), false, true
	case strings.HasPrefix(data, cbSubClass):
		return c.subWizardCameras(handler, strings.TrimPrefix(data, cbSubClass)), false, true
	case strings.HasPrefix(data, "s:a:"):
		reply, save := c.subWizardSubscribeCam(handler, strings.TrimPrefix(data, "s:a:"))

		return reply, save, true
	case strings.HasPrefix(data, "s:e:") && data != cbSubEvt:
		reply, save := c.subWizardSubscribeEvt(handler, strings.TrimPrefix(data, "s:e:"))

		return reply, save, true
	case data == cbUnsubRoot:
		return c.unsubWizardRoot(handler), false, true
	case strings.HasPrefix(data, "u:"):
		reply, save := c.unsubWizardPick(handler, strings.TrimPrefix(data, "u:"))

		return reply, save, true
	default:
		return nil, false, false
	}
}

func (c *Chat) subWizardRoot() *Reply {
	return &Reply{
		Reply: "Get notified when something happens.\n\n" +
			"Camera = motion / human / vehicle / animal video clips from SecuritySpy.\n" +
			"Event = text alerts for stream up/down, camera offline, SecuritySpy errors, etc.",
		Edit: true,
		Keyboard: [][]Button{
			{{Label: "Camera", Data: cbSubCam}, {Label: "Event", Data: cbSubEvt}},
			{{Label: "Cancel", Data: cbCancel}},
		},
	}
}

func (c *Chat) subWizardClasses() *Reply {
	return &Reply{
		Reply: "What should trigger a video to your phone?\n\n" +
			"Motion = any motion.\n" +
			"Human / Vehicle / Animal = only when SecuritySpy classifies that type.",
		Edit: true,
		Keyboard: [][]Button{
			{
				{Label: "Motion", Data: cbSubClass + classShort(ClassMotion)},
				{Label: "Human", Data: cbSubClass + classShort(ClassHuman)},
			},
			{
				{Label: "Vehicle", Data: cbSubClass + classShort(ClassVehicle)},
				{Label: "Animal", Data: cbSubClass + classShort(ClassAnimal)},
			},
			{{Label: "« Back", Data: cbSubRoot}, {Label: "Cancel", Data: cbCancel}},
		},
	}
}

func (c *Chat) subWizardCameras(handler *Handler, classShortCode string) *Reply {
	class := classFromShort(classShortCode)
	if class == ClassAny {
		return c.subWizardClasses()
	}

	cams := c.SSpy.Cameras.All()
	rows := make([][]Button, 0, len(cams)/2+2)

	var sub *subscribe.Subscriber
	if handler != nil {
		sub = handler.Sub
	}

	var row []Button
	for idx, cam := range cams {
		label := cam.Name
		if badges := cameraSubBadges(sub, cam.Name); badges != "" {
			label += " " + badges
		}
		if !cam.Connected.Val {
			label += " ⚠"
		}
		row = append(row, Button{
			Label: label,
			Data:  fmt.Sprintf("s:a:%s:%d", classShort(class), idx),
		})
		if len(row) == 2 {
			rows = append(rows, row)
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	rows = append(rows, []Button{
		{Label: "« Back", Data: cbSubCam},
		{Label: "Cancel", Data: cbCancel},
	})

	return &Reply{
		Reply: fmt.Sprintf(
			"Pick a camera for %s alerts.\n\n"+
				"You'll get a short video when SecuritySpy sees %s on that camera.\n"+
				"[M] motion · [H] human · [V] vehicle · [A] animal",
			strings.ToLower(classLabel(class)), strings.ToLower(classLabel(class))),
		Edit:     true,
		Keyboard: rows,
	}
}

func (c *Chat) subWizardEvents() *Reply {
	names := c.Subs.Events.Names()
	rows := make([][]Button, 0, len(names)+1)

	for i, name := range names {
		rows = append(rows, []Button{{
			Label: name,
			Data:  fmt.Sprintf("s:e:%d", i),
		}})
	}
	if len(names) == 0 {
		return &Reply{
			Reply: "No custom events configured.\nUse Camera subscriptions instead.",
			Edit:  true,
			Keyboard: [][]Button{
				{{Label: "« Back", Data: cbSubRoot}},
			},
		}
	}

	rows = append(rows, []Button{
		{Label: "« Back", Data: cbSubRoot},
		{Label: "Cancel", Data: cbCancel},
	})

	return &Reply{
		Reply:    "Pick an event:",
		Edit:     true,
		Keyboard: rows,
	}
}

func (c *Chat) subWizardSubscribeCam(handler *Handler, payload string) (*Reply, bool) {
	classShortCode, idxStr, ok := strings.Cut(payload, ":")
	if !ok {
		return &Reply{Reply: "Bad camera pick.", Edit: true, Toast: "Error"}, false
	}

	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		return &Reply{Reply: "Bad camera index.", Edit: true, Toast: "Error"}, false
	}

	cams := c.SSpy.Cameras.All()
	if idx < 0 || idx >= len(cams) {
		return &Reply{Reply: "Camera gone — try again.", Edit: true, Toast: "Missing"}, false
	}

	cam := cams[idx]
	class := classFromShort(classShortCode)
	if class == ClassAny {
		return c.subWizardClasses(), false
	}

	key := CameraSubKey(cam.Name, class)
	toast := "Subscribed ✓"
	msg := fmt.Sprintf("Subscribed to %s (%s).", cam.Name, classLabel(class))

	err = handler.Sub.Subscribe(key)
	if err != nil {
		msg = fmt.Sprintf("Already subscribed to %s (%s).", cam.Name, classLabel(class))
		toast = "Already on"
	}

	next := c.subWizardCameras(handler, classShort(class))
	next.Reply = msg + fmt.Sprintf(" (%d total)\n\n", handler.Sub.Events.Len()) + next.Reply
	next.Toast = toast

	return next, true
}

func (c *Chat) subWizardSubscribeEvt(handler *Handler, idxStr string) (*Reply, bool) {
	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		return &Reply{Reply: "Bad event index.", Edit: true, Toast: "Error"}, false
	}

	names := c.Subs.Events.Names()
	if idx < 0 || idx >= len(names) {
		return &Reply{Reply: "Event gone — try again.", Edit: true, Toast: "Missing"}, false
	}

	event := names[idx]
	msg := "Subscribed to event: " + event
	toast := "Subscribed ✓"

	err = handler.Sub.Subscribe(event)
	if err != nil {
		msg = "Already subscribed to: " + event
		toast = "Already on"
	}

	msg += fmt.Sprintf("\nYou have %d subscriptions.", handler.Sub.Events.Len())

	return &Reply{
		Reply: msg,
		Edit:  true,
		Toast: toast,
		Keyboard: [][]Button{
			{{Label: "Subscribe another", Data: cbSubRoot}},
			{{Label: "Done", Data: cbCancel}},
		},
	}, true
}

func (c *Chat) unsubWizardRoot(handler *Handler) *Reply {
	names := handler.Sub.Events.Names()
	if len(names) == 0 {
		return &Reply{
			Reply: "You're not subscribed to anything.",
			Edit:  true,
			Toast: "Empty",
			Keyboard: [][]Button{
				{{Label: "Subscribe", Data: cbSubRoot}},
			},
		}
	}

	rows := make([][]Button, 0, len(names)+1)
	for i, name := range names {
		rows = append(rows, []Button{{
			Label: formatSubLabel(name),
			Data:  fmt.Sprintf("u:%d", i),
		}})
	}

	rows = append(rows, []Button{
		{Label: "Unsubscribe all", Data: "u:*"},
		{Label: "Cancel", Data: cbCancel},
	})

	return &Reply{
		Reply: "Tap a subscription to stop getting those alerts.\n\n" +
			"Unsubscribe all removes every camera/event at once.",
		Edit:     true,
		Keyboard: rows,
	}
}

func (c *Chat) unsubWizardPick(handler *Handler, pick string) (*Reply, bool) {
	if pick == "*" {
		for _, e := range handler.Sub.Events.Names() {
			handler.Sub.Events.Remove(e)
		}

		return &Reply{
			Reply: "Unsubscribed from everything.",
			Edit:  true,
			Toast: "Cleared",
			Keyboard: [][]Button{
				{{Label: "Subscribe", Data: cbSubRoot}},
			},
		}, true
	}

	idx, err := strconv.Atoi(pick)
	if err != nil {
		return &Reply{Reply: "Bad pick.", Edit: true, Toast: "Error"}, false
	}

	names := handler.Sub.Events.Names()
	if idx < 0 || idx >= len(names) {
		return &Reply{Reply: "That subscription is gone.", Edit: true, Toast: "Missing"}, false
	}

	event := names[idx]
	handler.Sub.Events.Remove(event)

	msg := "Unsubscribed from: " + formatSubLabel(event)
	msg += fmt.Sprintf("\nYou have %d subscriptions.", handler.Sub.Events.Len())

	// Keep the menu open if anything remains.
	if handler.Sub.Events.Len() > 0 {
		next := c.unsubWizardRoot(handler)
		next.Reply = msg + "\n\n" + next.Reply
		next.Toast = "Removed"

		return next, true
	}

	return &Reply{
		Reply: msg,
		Edit:  true,
		Toast: "Removed",
		Keyboard: [][]Button{
			{{Label: "Subscribe", Data: cbSubRoot}},
		},
	}, true
}

func formatSubLabel(key string) string {
	cam, class := ParseCameraSubKey(key)
	if class == ClassAny && !strings.Contains(key, classSep) {
		return key
	}

	return fmt.Sprintf("%s · %s", cam, classLabel(class))
}

// eventDelay returns the configured repeat delay for an event, or DefaultRepeatDelay.
func eventDelay(events *subscribe.Events, event string) time.Duration {
	if events == nil {
		return DefaultRepeatDelay
	}

	if delay, ok := events.RuleGetD(event, "delay"); ok {
		return delay
	}

	return DefaultRepeatDelay
}

// resolveSubTarget turns /sub args into a subscription key and kind label.
func (c *Chat) resolveSubTarget(args []string) (string, string, error) {
	if len(args) == 0 {
		return "", "", ErrBadUsage
	}

	joined := strings.Join(args, " ")
	key, kind, err := c.resolveCameraClassTarget(args, joined)
	if key != "" || err != nil {
		return key, kind, err
	}

	if c.Subs.Events.Exists(joined) {
		return joined, "event", nil
	}

	if cam := c.SSpy.Cameras.ByName(joined); cam != nil {
		return "", "", fmt.Errorf("%w: specify a class for %s (motion|human|vehicle|animal)", ErrBadUsage, cam.Name)
	}

	return "", "", fmt.Errorf("%w: event or camera not found: %s", ErrBadUsage, joined)
}

func (c *Chat) resolveCameraClassTarget(args []string, joined string) (string, string, error) {
	// Power-user forms: "Office:human" or "Office human"
	if strings.Contains(joined, classSep) {
		camName, class := ParseCameraSubKey(joined)
		if class == ClassAny {
			return "", "", fmt.Errorf("%w: choose motion, human, vehicle, or animal", ErrBadUsage)
		}

		if cam := c.SSpy.Cameras.ByName(camName); cam != nil {
			return CameraSubKey(cam.Name, class), "camera", nil
		}
	}

	if len(args) < 2 || !isClassWord(args[len(args)-1]) {
		return "", "", nil
	}

	camName := strings.Join(args[:len(args)-1], " ")
	class := normalizeClass(args[len(args)-1])
	if class == ClassAny {
		return "", "", fmt.Errorf("%w: choose motion, human, vehicle, or animal", ErrBadUsage)
	}

	if cam := c.SSpy.Cameras.ByName(camName); cam != nil {
		return CameraSubKey(cam.Name, class), "camera", nil
	}

	return "", "", nil
}

func isClassWord(word string) bool {
	switch strings.ToLower(strings.TrimSpace(word)) {
	case ClassMotion, "m",
		ClassHuman, "h", "person", "people",
		ClassVehicle, "v", "car",
		ClassAnimal, "a":
		return true
	default:
		return false
	}
}

// CollectSubscribers returns unique subscribers matching any of the event keys.
func CollectSubscribers(data *subscribe.Subscribe, keys []string) []*subscribe.Subscriber {
	if data == nil || len(keys) == 0 {
		return nil
	}

	seen := make(map[*subscribe.Subscriber]bool)
	out := make([]*subscribe.Subscriber, 0)

	for _, key := range keys {
		for _, sub := range data.GetSubscribers(key) {
			if seen[sub] {
				continue
			}
			seen[sub] = true
			out = append(out, sub)
		}
	}

	return out
}

// ActiveKeysAmong returns which of keys the subscriber is subscribed to and not paused.
func ActiveKeysAmong(sub *subscribe.Subscriber, keys []string) []string {
	if sub == nil || sub.Events == nil {
		return nil
	}

	out := make([]string, 0, len(keys))
	for _, key := range keys {
		if !sub.Events.Exists(key) {
			continue
		}

		if sub.Events.IsPaused(key) {
			continue
		}

		out = append(out, key)
	}

	return out
}
