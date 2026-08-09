package chat

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"golift.io/subscribe"
)

// Admin manage-another-user's-subscriptions wizard (under /users, Telegram ≤64 bytes).
//
// m:subs:{uid}              → list target's subscriptions
// m:si:{uid}:{idx}          → manage one subscription
// m:sp:{uid}:{mins}:{idx}   → pause / clear pause
// m:sd:{uid}:{idx}          → delay presets
// m:sda:{uid}:{idx}:{secs}  → apply delay
// m:su:{uid}:{idx}          → unsubscribe
// m:ss:{uid}                → subscribe: pick trigger class
// m:ss:{uid}:{class}        → subscribe: pick camera
// m:ssa:{uid}:{class}:{idx} → subscribe: apply

func (c *Chat) adminSubsWizardRoot(handler *Handler, idStr string) *Reply {
	if reply := c.requireAdmin(handler); reply != nil {
		return reply
	}

	target, err := c.adminTargetByID(handler.API, idStr)
	if err != nil {
		return c.adminTargetGone()
	}

	names := targetEventNames(target)
	display := subscriberDisplayName(target)

	var msg strings.Builder
	fmt.Fprintf(&msg, "Subscriptions for %s (%d).\n\n", display, len(names))
	msg.WriteString("Tap one to pause, change delay, or unsubscribe.\n")

	rows := make([][]Button, 0, len(names)+2)
	for idx, event := range names {
		line := formatSubLabel(event)
		fmt.Fprintf(&msg, "\n• %s · every %s", line, formatDuration(eventDelay(target.Events, event)))
		if target.Events.IsPaused(event) {
			until := time.Until(target.Events.PauseTime(event))
			fmt.Fprintf(&msg, " (paused %s)", formatDuration(until))
		}
		rows = append(rows, []Button{{
			Label: line,
			Data:  fmt.Sprintf("m:si:%d:%d", target.ID, idx),
		}})
	}

	if len(names) == 0 {
		msg.WriteString("\n(none yet)")
	}

	rows = append(rows,
		[]Button{{Label: "Subscribe for them", Data: fmt.Sprintf("m:ss:%d", target.ID)}},
		[]Button{
			{Label: "« User", Data: fmt.Sprintf("m:i:%d", target.ID)},
			{Label: "Done", Data: cbCancel},
		},
	)

	return &Reply{Reply: msg.String(), Edit: true, Keyboard: rows}
}

func (c *Chat) adminSubsWizardItem(handler *Handler, payload string) *Reply {
	if reply := c.requireAdmin(handler); reply != nil {
		return reply
	}

	idStr, idxStr, found := strings.Cut(payload, ":")
	if !found {
		return &Reply{Reply: "Bad pick.", Edit: true, Toast: "Error"}
	}

	target, err := c.adminTargetByID(handler.API, idStr)
	if err != nil {
		return c.adminTargetGone()
	}

	idx := atoiDefault(idxStr, -1)
	names := targetEventNames(target)
	if idx < 0 || idx >= len(names) {
		return &Reply{Reply: "Subscription gone.", Edit: true, Toast: "Missing"}
	}

	event := names[idx]
	label := formatSubLabel(event)
	uid := target.ID

	return &Reply{
		Reply: fmt.Sprintf("Manage %s for %s\n\n"+
			"Pause = silence this subscription for a while.\n"+
			"Set delay = how often clips may arrive.\n"+
			"Unsubscribe = remove this subscription.",
			label, subscriberDisplayName(target)),
		Edit: true,
		Keyboard: [][]Button{
			{
				{Label: "Pause 10m", Data: fmt.Sprintf("m:sp:%d:10:%d", uid, idx)},
				{Label: "Clear pause", Data: fmt.Sprintf("m:sp:%d:0:%d", uid, idx)},
			},
			{
				{Label: "Set delay", Data: fmt.Sprintf("m:sd:%d:%d", uid, idx)},
				{Label: "Unsubscribe", Data: fmt.Sprintf("m:su:%d:%d", uid, idx)},
			},
			{
				{Label: "« Subs", Data: fmt.Sprintf("m:subs:%d", uid)},
				{Label: "Done", Data: cbCancel},
			},
		},
	}
}

func (c *Chat) adminSubsWizardPause(handler *Handler, payload string) (*Reply, bool) {
	if reply := c.requireAdmin(handler); reply != nil {
		return reply, false
	}

	idStr, rest, found := strings.Cut(payload, ":")
	if !found {
		return &Reply{Reply: "Bad pause pick.", Edit: true, Toast: "Error"}, false
	}

	minsStr, idxStr, found := strings.Cut(rest, ":")
	if !found {
		return &Reply{Reply: "Bad pause pick.", Edit: true, Toast: "Error"}, false
	}

	target, err := c.adminTargetByID(handler.API, idStr)
	if err != nil {
		return c.adminTargetGone(), false
	}

	mins, err := strconv.Atoi(minsStr)
	if err != nil || mins < 0 || mins > MaxPauseMinutes {
		return &Reply{
			Reply: fmt.Sprintf("Pause must be 0–%d minutes (24 hours).", MaxPauseMinutes),
			Edit:  true,
			Toast: "Error",
		}, false
	}

	idx := atoiDefault(idxStr, -1)
	names := targetEventNames(target)
	if idx < 0 || idx >= len(names) {
		return &Reply{Reply: "Subscription gone.", Edit: true, Toast: "Missing"}, false
	}

	event := names[idx]
	err = target.Events.Pause(event, time.Duration(mins)*time.Minute)
	if err != nil {
		return &Reply{Reply: "Subscription gone.", Edit: true, Toast: "Missing"}, false
	}

	msg := fmt.Sprintf("Paused %s for %s (%d min).",
		formatSubLabel(event), subscriberDisplayName(target), mins)
	if mins == 0 {
		msg = fmt.Sprintf("Cleared pause on %s for %s.",
			formatSubLabel(event), subscriberDisplayName(target))
	}

	next := c.adminSubsWizardRoot(handler, idStr)
	next.Reply = msg + "\n\n" + next.Reply
	next.Toast = "Saved"

	return next, true
}

func (c *Chat) adminSubsWizardDelayPick(handler *Handler, payload string) *Reply {
	if reply := c.requireAdmin(handler); reply != nil {
		return reply
	}

	idStr, idxStr, found := strings.Cut(payload, ":")
	if !found {
		return &Reply{Reply: "Bad delay pick.", Edit: true, Toast: "Error"}
	}

	target, err := c.adminTargetByID(handler.API, idStr)
	if err != nil {
		return c.adminTargetGone()
	}

	idx := atoiDefault(idxStr, -1)
	names := targetEventNames(target)
	if idx < 0 || idx >= len(names) {
		return &Reply{Reply: "Subscription gone.", Edit: true, Toast: "Missing"}
	}

	uid := target.ID

	return &Reply{
		Reply: fmt.Sprintf("Repeat delay for %s (%s).",
			formatSubLabel(names[idx]), subscriberDisplayName(target)),
		Edit: true,
		Keyboard: [][]Button{
			{
				{Label: "15s", Data: fmt.Sprintf("m:sda:%d:%d:15", uid, idx)},
				{Label: "30s", Data: fmt.Sprintf("m:sda:%d:%d:30", uid, idx)},
				{Label: "60s", Data: fmt.Sprintf("m:sda:%d:%d:60", uid, idx)},
			},
			{
				{Label: "2 min", Data: fmt.Sprintf("m:sda:%d:%d:120", uid, idx)},
				{Label: "5 min", Data: fmt.Sprintf("m:sda:%d:%d:300", uid, idx)},
				{Label: "10 min", Data: fmt.Sprintf("m:sda:%d:%d:600", uid, idx)},
			},
			{
				{Label: "« Back", Data: fmt.Sprintf("m:si:%d:%d", uid, idx)},
				{Label: "Done", Data: cbCancel},
			},
		},
	}
}

func (c *Chat) adminSubsWizardDelayApply(handler *Handler, payload string) (*Reply, bool) {
	if reply := c.requireAdmin(handler); reply != nil {
		return reply, false
	}

	parts := strings.Split(payload, ":")
	if len(parts) != 3 {
		return &Reply{Reply: "Bad delay pick.", Edit: true, Toast: "Error"}, false
	}

	target, err := c.adminTargetByID(handler.API, parts[0])
	if err != nil {
		return c.adminTargetGone(), false
	}

	idx := atoiDefault(parts[1], -1)
	secs := atoiDefault(parts[2], 60)
	names := targetEventNames(target)
	if idx < 0 || idx >= len(names) {
		return &Reply{Reply: "Subscription gone.", Edit: true, Toast: "Missing"}, false
	}

	event := names[idx]
	target.Events.RuleSetD(event, "delay", time.Duration(secs)*time.Second)

	next := c.adminSubsWizardRoot(handler, parts[0])
	next.Reply = fmt.Sprintf("Delay for %s set to %s.\n\n",
		formatSubLabel(event), formatDuration(time.Duration(secs)*time.Second)) + next.Reply
	next.Toast = "Saved"

	return next, true
}

func (c *Chat) adminSubsWizardUnsub(handler *Handler, payload string) (*Reply, bool) {
	if reply := c.requireAdmin(handler); reply != nil {
		return reply, false
	}

	idStr, idxStr, found := strings.Cut(payload, ":")
	if !found {
		return &Reply{Reply: "Bad pick.", Edit: true, Toast: "Error"}, false
	}

	target, err := c.adminTargetByID(handler.API, idStr)
	if err != nil {
		return c.adminTargetGone(), false
	}

	idx := atoiDefault(idxStr, -1)
	names := targetEventNames(target)
	if idx < 0 || idx >= len(names) {
		return &Reply{Reply: "Subscription gone.", Edit: true, Toast: "Missing"}, false
	}

	event := names[idx]
	target.Events.Remove(event)

	next := c.adminSubsWizardRoot(handler, idStr)
	next.Reply = fmt.Sprintf("Unsubscribed %s from %s.\n\n",
		subscriberDisplayName(target), formatSubLabel(event)) + next.Reply
	next.Toast = "Removed"

	return next, true
}

func (c *Chat) adminSubsWizardSubClass(handler *Handler, idStr string) *Reply {
	if reply := c.requireAdmin(handler); reply != nil {
		return reply
	}

	target, err := c.adminTargetByID(handler.API, idStr)
	if err != nil {
		return c.adminTargetGone()
	}

	uid := target.ID

	return &Reply{
		Reply: fmt.Sprintf("Subscribe %s — which trigger?", subscriberDisplayName(target)),
		Edit:  true,
		Keyboard: [][]Button{
			{
				{Label: "Motion", Data: fmt.Sprintf("m:ss:%d:%s", uid, classShort(ClassMotion))},
				{Label: "Human", Data: fmt.Sprintf("m:ss:%d:%s", uid, classShort(ClassHuman))},
			},
			{
				{Label: "Vehicle", Data: fmt.Sprintf("m:ss:%d:%s", uid, classShort(ClassVehicle))},
				{Label: "Animal", Data: fmt.Sprintf("m:ss:%d:%s", uid, classShort(ClassAnimal))},
			},
			{
				{Label: "« Subs", Data: fmt.Sprintf("m:subs:%d", uid)},
				{Label: "Done", Data: cbCancel},
			},
		},
	}
}

func (c *Chat) adminSubsWizardSubCameras(handler *Handler, payload string) *Reply {
	if reply := c.requireAdmin(handler); reply != nil {
		return reply
	}

	idStr, classShortCode, found := strings.Cut(payload, ":")
	if !found {
		return &Reply{Reply: "Bad pick.", Edit: true, Toast: "Error"}
	}

	target, err := c.adminTargetByID(handler.API, idStr)
	if err != nil {
		return c.adminTargetGone()
	}

	class := classFromShort(classShortCode)
	if class == ClassAny {
		return c.adminSubsWizardSubClass(handler, idStr)
	}

	cams := c.allCameras()
	if len(cams) == 0 {
		return c.noCamerasReply()
	}

	uid := target.ID
	rows := make([][]Button, 0, len(cams)/2+2)
	row := make([]Button, 0, 2)

	for camIdx, cam := range cams {
		label := cam.Name
		if badges := cameraSubBadges(target, cam.Name); badges != "" {
			label += " " + badges
		}
		if !cam.Connected.Val {
			label += " ⚠"
		}
		row = append(row, Button{
			Label: label,
			Data:  fmt.Sprintf("m:ssa:%d:%s:%d", uid, classShort(class), camIdx),
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
		{Label: "« Back", Data: fmt.Sprintf("m:ss:%d", uid)},
		{Label: "Done", Data: cbCancel},
	})

	return &Reply{
		Reply: fmt.Sprintf("Pick a camera for %s alerts for %s.",
			strings.ToLower(classLabel(class)), subscriberDisplayName(target)),
		Edit:     true,
		Keyboard: rows,
	}
}

func (c *Chat) adminSubsWizardSubApply(handler *Handler, payload string) (*Reply, bool) {
	if reply := c.requireAdmin(handler); reply != nil {
		return reply, false
	}

	parts := strings.Split(payload, ":")
	if len(parts) != 3 {
		return &Reply{Reply: "Bad camera pick.", Edit: true, Toast: "Error"}, false
	}

	target, err := c.adminTargetByID(handler.API, parts[0])
	if err != nil {
		return c.adminTargetGone(), false
	}

	class := classFromShort(parts[1])
	if class == ClassAny {
		return c.adminSubsWizardSubClass(handler, parts[0]), false
	}

	camIdx := atoiDefault(parts[2], -1)
	cams := c.allCameras()
	if camIdx < 0 || camIdx >= len(cams) {
		return &Reply{Reply: "Camera gone — try again.", Edit: true, Toast: "Missing"}, false
	}

	cam := cams[camIdx]
	key := CameraSubKey(cam.Name, class)
	toast := "Subscribed ✓"
	msg := fmt.Sprintf("Subscribed %s to %s (%s).",
		subscriberDisplayName(target), cam.Name, classLabel(class))

	err = target.Subscribe(key)
	if err != nil {
		msg = fmt.Sprintf("%s already has %s (%s).",
			subscriberDisplayName(target), cam.Name, classLabel(class))
		toast = "Already on"
	}

	next := c.adminSubsWizardSubCameras(handler, parts[0]+":"+parts[1])
	next.Reply = msg + "\n\n" + next.Reply
	next.Toast = toast

	return next, true
}

func (c *Chat) requireAdmin(handler *Handler) *Reply {
	if handler == nil || handler.Sub == nil || !SubAdmin(handler.Sub) {
		return &Reply{Reply: "Admins only.", Edit: true, Toast: "Nope"}
	}

	return nil
}

func (c *Chat) adminTargetGone() *Reply {
	return &Reply{
		Reply: "Subscriber gone — try again.", Edit: true, Toast: "Missing",
		Keyboard: [][]Button{{{Label: "« Users", Data: cbUsersRoot}}},
	}
}

func targetEventNames(target *subscribe.Subscriber) []string {
	if target == nil || target.Events == nil {
		return nil
	}

	return target.Events.Names()
}
