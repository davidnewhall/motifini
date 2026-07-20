package chat

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golift.io/securityspy/v2"
)

// Extra callback prefixes for command wizards (pics/vid/stop/delay/cams/subs/help).
const (
	cbPicsRoot  = "p"
	cbVidsRoot  = "v"
	cbCamsRoot  = "c"
	cbStopRoot  = "t"
	cbDelayRoot = "d"
	cbSubsRoot  = "l"
	cbHelpRoot  = "h"
	cbEvtsRoot  = "e"
)

func (c *Chat) handleCommandWizardCallback(handler *Handler, data string) (*Reply, bool, bool) {
	if reply, save, ok := c.handleMediaWizardCallback(handler, data); ok {
		return reply, save, true
	}

	if reply, save, ok := c.handlePauseDelayWizardCallback(handler, data); ok {
		return reply, save, true
	}

	if reply, save, ok := c.handleUsersWizardCallback(handler, data); ok {
		return reply, save, true
	}

	if data == cbHelpRoot {
		return c.helpWizardRootFor(handler), false, true
	}

	return nil, false, false
}

func (c *Chat) handleMediaWizardCallback(handler *Handler, data string) (*Reply, bool, bool) {
	switch {
	case data == cbPicsRoot:
		return c.picsWizardRoot(), false, true
	case data == "p:a":
		reply, save := c.picsWizardSnap(handler, -1)

		return reply, save, true
	case strings.HasPrefix(data, "p:"):
		reply, save := c.picsWizardSnap(handler, atoiDefault(strings.TrimPrefix(data, "p:"), -2))

		return reply, save, true
	case data == cbVidsRoot:
		return c.vidsWizardRoot(), false, true
	case data == "v:a":
		reply, save := c.vidsWizardSnap(handler, -1)

		return reply, save, true
	case strings.HasPrefix(data, "v:"):
		reply, save := c.vidsWizardSnap(handler, atoiDefault(strings.TrimPrefix(data, "v:"), -2))

		return reply, save, true
	case data == cbCamsRoot:
		return c.camsWizardRoot(), false, true
	case strings.HasPrefix(data, "c:") && strings.Count(data, ":") == 1:
		return c.camsWizardCam(atoiDefault(strings.TrimPrefix(data, "c:"), -1)), false, true
	case strings.HasPrefix(data, "c:p:"):
		reply, save := c.picsWizardSnap(handler, atoiDefault(strings.TrimPrefix(data, "c:p:"), -2))

		return reply, save, true
	case strings.HasPrefix(data, "c:v:"):
		reply, save := c.vidsWizardSnap(handler, atoiDefault(strings.TrimPrefix(data, "c:v:"), -2))

		return reply, save, true
	case data == cbEvtsRoot:
		return c.eventsWizardRoot(), false, true
	case strings.HasPrefix(data, "e:s:"):
		reply, save := c.subWizardSubscribeEvt(handler, strings.TrimPrefix(data, "e:s:"))

		return reply, save, true
	default:
		return nil, false, false
	}
}

func (c *Chat) handlePauseDelayWizardCallback(handler *Handler, data string) (*Reply, bool, bool) {
	switch {
	case data == cbStopRoot:
		return c.stopWizardRoot(), false, true
	case strings.HasPrefix(data, "t:") && strings.Count(data, ":") == 1:
		return c.stopWizardTargets(handler, strings.TrimPrefix(data, "t:")), false, true
	case strings.HasPrefix(data, "t:"):
		reply, save := c.stopWizardApply(handler, strings.TrimPrefix(data, "t:"))

		return reply, save, true
	case data == cbDelayRoot:
		return c.delayWizardRoot(handler), false, true
	case strings.HasPrefix(data, "d:") && strings.Count(data, ":") == 1:
		return c.delayWizardSeconds(strings.TrimPrefix(data, "d:")), false, true
	case strings.HasPrefix(data, "d:"):
		reply, save := c.delayWizardApply(handler, strings.TrimPrefix(data, "d:"))

		return reply, save, true
	case data == cbSubsRoot:
		return c.subsWizardRoot(handler), false, true
	case strings.HasPrefix(data, "l:"):
		return c.subsWizardItem(handler, strings.TrimPrefix(data, "l:")), false, true
	default:
		return nil, false, false
	}
}

func (c *Chat) handleUsersWizardCallback(handler *Handler, data string) (*Reply, bool, bool) {
	switch {
	case strings.HasPrefix(data, "m:rename:"):
		return c.usersWizardRenamePrompt(handler, strings.TrimPrefix(data, "m:rename:")), true, true
	case strings.HasPrefix(data, "m:i:"):
		clearPendingRename(handler.Sub)

		return c.usersWizardItem(handler, strings.TrimPrefix(data, "m:i:")), false, true
	case data == cbUsersRoot:
		clearPendingRename(handler.Sub)

		return c.usersWizardRoot(handler), false, true
	case strings.HasPrefix(data, "m:delok:"): // before m:del:
		reply, save := c.usersWizardAction(handler, "delok", strings.TrimPrefix(data, "m:delok:"))

		return reply, save, true
	case strings.HasPrefix(data, "m:del:"):
		return c.usersWizardConfirmDelete(handler, strings.TrimPrefix(data, "m:del:")), false, true
	case strings.HasPrefix(data, "m:allow:"):
		reply, save := c.usersWizardAction(handler, "allow", strings.TrimPrefix(data, "m:allow:"))

		return reply, save, true
	case strings.HasPrefix(data, "m:deny:"):
		reply, save := c.usersWizardAction(handler, "deny", strings.TrimPrefix(data, "m:deny:"))

		return reply, save, true
	case strings.HasPrefix(data, "m:unignore:"): // before m:ignore:
		reply, save := c.usersWizardAction(handler, "unignore", strings.TrimPrefix(data, "m:unignore:"))

		return reply, save, true
	case strings.HasPrefix(data, "m:ignore:"):
		reply, save := c.usersWizardAction(handler, "ignore", strings.TrimPrefix(data, "m:ignore:"))

		return reply, save, true
	case strings.HasPrefix(data, "m:unadmin:"): // before m:admin:
		reply, save := c.usersWizardAction(handler, "unadmin", strings.TrimPrefix(data, "m:unadmin:"))

		return reply, save, true
	case strings.HasPrefix(data, "m:admin:"):
		reply, save := c.usersWizardAction(handler, "admin", strings.TrimPrefix(data, "m:admin:"))

		return reply, save, true
	default:
		return nil, false, false
	}
}

func atoiDefault(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}

	return n
}

func (c *Chat) cameraButtonRows(dataPrefix string, includeAll bool) [][]Button {
	cams := c.SSpy.Cameras.All()
	rows := make([][]Button, 0, len(cams)/2+3)

	if includeAll {
		rows = append(rows, []Button{{Label: "All cameras", Data: dataPrefix + "a"}})
	}

	row := make([]Button, 0, len(cams))
	for i, cam := range cams {
		label := cam.Name
		if !cam.Connected.Val {
			label += " ⚠"
		}
		row = append(row, Button{Label: label, Data: fmt.Sprintf("%s%d", dataPrefix, i)})
		if len(row) == 2 {
			rows = append(rows, row)
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	return rows
}

func (c *Chat) picsWizardRoot() *Reply {
	_ = c.SSpy.Refresh()
	rows := c.cameraButtonRows("p:", true)
	rows = append(rows, []Button{{Label: "Cancel", Data: cbCancel}})

	return &Reply{
		Reply: "Grab a still photo from SecuritySpy right now.\n\n" +
			"Pick a camera, or All cameras to get one shot from each " +
			"(photos arrive one at a time as they're ready).",
		Edit:     true,
		Keyboard: rows,
	}
}

func (c *Chat) vidsWizardRoot() *Reply {
	rows := c.cameraButtonRows("v:", true)
	rows = append(rows, []Button{{Label: "Cancel", Data: cbCancel}})

	return &Reply{
		Reply: "Grab a short live video clip from SecuritySpy right now.\n\n" +
			"Pick a camera, or All cameras (this can take a bit).",
		Edit:     true,
		Keyboard: rows,
	}
}

func (c *Chat) camsWizardRoot() *Reply {
	_ = c.SSpy.Refresh()
	rows := c.cameraButtonRows("c:", false)
	rows = append(rows, []Button{{Label: "Cancel", Data: cbCancel}})

	cams := c.SSpy.Cameras.All()
	online := 0
	for _, cam := range cams {
		if cam.Connected.Val {
			online++
		}
	}

	return &Reply{
		Reply: fmt.Sprintf(
			"%d cameras (%d online).\n\nTap a camera to take a snapshot or pull a live video clip.",
			len(cams), online),
		Edit:     true,
		Keyboard: rows,
	}
}

func (c *Chat) camsWizardCam(idx int) *Reply {
	cams := c.SSpy.Cameras.All()
	if idx < 0 || idx >= len(cams) {
		return &Reply{Reply: "Camera gone — try again.", Edit: true, Toast: "Missing"}
	}

	cam := cams[idx]
	status := "online"
	if !cam.Connected.Val {
		status = "down"
	}

	return &Reply{
		Reply: fmt.Sprintf("%s (%s)\n\nSnapshot = one still photo.\nVideo = a short live clip.", cam.Name, status),
		Edit:  true,
		Keyboard: [][]Button{
			{
				{Label: "Snapshot", Data: fmt.Sprintf("c:p:%d", idx)},
				{Label: "Video", Data: fmt.Sprintf("c:v:%d", idx)},
			},
			{{Label: "« Cameras", Data: cbCamsRoot}, {Label: "Cancel", Data: cbCancel}},
		},
	}
}

func (c *Chat) eventsWizardRoot() *Reply {
	names := c.Subs.Events.Names()
	if len(names) == 0 {
		return &Reply{
			Reply: "No custom events are configured on this Motifini.\n\n" +
				"Most people subscribe to cameras instead (motion / human / vehicle / animal).",
			Edit: true,
			Keyboard: [][]Button{
				{{Label: "Subscribe to camera", Data: cbSubCam}},
				{{Label: "Cancel", Data: cbCancel}},
			},
		}
	}

	rows := make([][]Button, 0, len(names)+1)
	for idx, name := range names {
		desc, _ := c.Subs.Events.RuleGetS(name, "description")
		label := name
		if desc != "" {
			label = name + " — " + desc
			if len(label) > 64 {
				label = label[:61] + "…"
			}
		}
		rows = append(rows, []Button{{
			Label: label,
			Data:  fmt.Sprintf("e:s:%d", idx),
		}})
	}
	rows = append(rows, []Button{{Label: "Cancel", Data: cbCancel}})

	return &Reply{
		Reply: "System and custom events (not camera motion clips).\n\n" +
			"These send a text alert — no video.\n" +
			"Built-ins cover stream up/down, cameras going offline/online, and SecuritySpy errors.\n\n" +
			"Tap one to subscribe:",
		Edit:     true,
		Keyboard: rows,
	}
}

func (c *Chat) picsWizardSnap(handler *Handler, idx int) (*Reply, bool) {
	return c.mediaWizardSnap(handler, idx, false)
}

func (c *Chat) vidsWizardSnap(handler *Handler, idx int) (*Reply, bool) {
	return c.mediaWizardSnap(handler, idx, true)
}

func (c *Chat) mediaWizardSnap(handler *Handler, idx int, video bool) (*Reply, bool) {
	if idx == -2 {
		return &Reply{Reply: "Bad camera.", Edit: true, Toast: "Error"}, false
	}

	again := cbPicsRoot
	allMsg := "Snapshots from all cameras."
	kind := CaptionPhoto

	if video {
		again = cbVidsRoot
		allMsg = "Clips from all cameras."
		kind = CaptionVideo
	}

	if idx == -1 {
		files, msg := c.snapAll(handler, video)

		return &Reply{
			Reply: nonEmpty(msg, allMsg),
			Edit:  true,
			Toast: "Sending…",
			Files: files,
			Keyboard: [][]Button{
				{{Label: "Another", Data: again}, {Label: "Done", Data: cbCancel}},
			},
		}, false
	}

	cams := c.SSpy.Cameras.All()
	if idx < 0 || idx >= len(cams) {
		return &Reply{Reply: "Camera gone — try again.", Edit: true, Toast: "Missing"}, false
	}

	cam := cams[idx]
	path, errMsg := c.snapOne(handler, cam, video)
	files := []string{}

	if path != "" {
		files = []string{path}
	}

	return &Reply{
		Reply: nonEmpty(errMsg, CameraCaption(cam.Name, kind)),
		Edit:  true,
		Toast: "Sending…",
		Files: files,
		Keyboard: [][]Button{
			{{Label: "Another", Data: again}, {Label: "Done", Data: cbCancel}},
		},
	}, false
}

func nonEmpty(errMsg, fallback string) string {
	if errMsg != "" {
		return errMsg
	}

	return fallback
}

func (c *Chat) snapOne(handler *Handler, cam *securityspy.Camera, video bool) (string, string) {
	if !cam.Connected.Val {
		return "", "Skipping '" + cam.Name + "' (camera offline)"
	}

	path, errMsg := c.captureCam(handler, cam, video)
	if errMsg != "" {
		return "", errMsg
	}

	kind := CaptionPhoto
	if video {
		kind = CaptionVideo
	}

	caption := CameraCaption(cam.Name, kind)
	if handler.SendFile == nil {
		return path, ""
	}

	err := handler.SendFile(path, caption)
	if err != nil {
		log.Printf("[ERROR] [%v] SendFile %s: %v", handler.ID, cam.Name, err)
		_ = os.Remove(path)

		return "", "Error Sending '" + cam.Name + "': " + err.Error()
	}

	return "", "" // already delivered
}

func (c *Chat) captureCam(handler *Handler, cam *securityspy.Camera, video bool) (string, string) {
	if video {
		path := filepath.Join(c.TempDir, fmt.Sprintf("chat_command_%v_%v.mp4", handler.ID, cam.Name))
		log.Printf("[INFO] [%v] SaveVideo starting for %s", handler.ID, cam.Name)

		err := cam.SaveVideo(clipVidOps(cam), length, maxsize, path)
		if err != nil {
			log.Printf("[ERROR] [%v] cam.SaveVideo: capturing for %s: %v", handler.ID, cam.Name, err)

			return "", "Error Getting '" + cam.Name + "' Video: " + err.Error()
		}

		return path, ""
	}

	path := filepath.Join(c.TempDir, fmt.Sprintf("chat_command_%v_%v.jpg", handler.ID, cam.Name))
	log.Printf("[INFO] [%v] SaveJPEG starting for %s", handler.ID, cam.Name)

	err := cam.SaveJPEG(&securityspy.VidOps{Height: jpegHeight, Quality: quality}, path)
	if err != nil {
		log.Printf("[ERROR] [%v] cam.SaveJPEG: capturing for %s: %v", handler.ID, cam.Name, err)

		return "", "Error Getting '" + cam.Name + "' Picture: " + err.Error()
	}

	return path, ""
}

func (c *Chat) snapAll(handler *Handler, video bool) ([]string, string) {
	// Sequential on purpose: SecuritySpy encodes stills slowly, and Telegram
	// gets each file as it finishes (via handler.SendFile) so the UI doesn't freeze.
	// Refresh first so Connected is current — skip dead cams instead of waiting on them.
	err := c.SSpy.Refresh()
	if err != nil {
		log.Printf("[ERROR] [%v] snapAll Refresh: %v", handler.ID, err)
	}

	var (
		paths []string
		errs  []string
		okN   int
		cams  = c.SSpy.Cameras.All()
		total int
	)

	for _, cam := range cams {
		if !cam.Connected.Val {
			errs = append(errs, "Skipping '"+cam.Name+"' (camera offline)")
			continue
		}

		total++
	}

	log.Printf("[INFO] [%v] snapAll starting (%d online cameras, video=%v)", handler.ID, total, video)

	for _, cam := range cams {
		if !cam.Connected.Val {
			continue
		}

		path, errMsg := c.snapOne(handler, cam, video)
		if errMsg != "" {
			errs = append(errs, errMsg)
			continue
		}

		if path != "" {
			paths = append(paths, path)
		}

		okN++
	}

	summary := fmt.Sprintf("Done — sent %d of %d online cameras.", okN, total)
	if len(errs) > 0 {
		summary += "\n" + strings.Join(errs, "\n")
	}

	return paths, summary
}

func (c *Chat) stopWizardRoot() *Reply {
	return &Reply{
		Reply: "Temporarily silence motion alerts.\n\n" +
			"No video notifications will be sent for the time you pick " +
			"(handy when you're home and don't want a flood of clips).\n\n" +
			"How long should alerts stay quiet?",
		Edit: true,
		Keyboard: [][]Button{
			{
				{Label: "5 min", Data: "t:5"},
				{Label: "10 min", Data: "t:10"},
				{Label: "30 min", Data: "t:30"},
			},
			{
				{Label: "1 hour", Data: "t:60"},
				{Label: "Clear pause", Data: "t:0"},
			},
			{{Label: "Cancel", Data: cbCancel}},
		},
	}
}

func (c *Chat) stopWizardTargets(handler *Handler, minsStr string) *Reply {
	mins := atoiDefault(minsStr, 10)
	names := handler.Sub.Events.Names()
	rows := make([][]Button, 0, 1+len(names)+1)
	rows = append(rows, []Button{{Label: "All subscriptions", Data: fmt.Sprintf("t:%d:a", mins)}})

	for i, name := range names {
		rows = append(rows, []Button{{
			Label: formatSubLabel(name),
			Data:  fmt.Sprintf("t:%d:%d", mins, i),
		}})
	}

	rows = append(rows, []Button{
		{Label: "« Back", Data: cbStopRoot},
		{Label: "Cancel", Data: cbCancel},
	})

	var action string
	if mins == 0 {
		action = "Clear pause — turn alerts back on for:"
	} else {
		action = fmt.Sprintf(
			"Mute alerts for %d minutes.\n\nApply to everything, or just one subscription:",
			mins)
	}

	return &Reply{Reply: action, Edit: true, Keyboard: rows}
}

func (c *Chat) stopWizardApply(handler *Handler, payload string) (*Reply, bool) { //nolint:funlen // is what it is.
	minsStr, rest, ok := strings.Cut(payload, ":")
	if !ok {
		return &Reply{Reply: "Bad pause pick.", Edit: true, Toast: "Error"}, false
	}

	mins := atoiDefault(minsStr, 10)
	if mins < 0 || mins > MaxPauseMinutes {
		return &Reply{
			Reply: fmt.Sprintf("Pause must be 0–%d minutes (24 hours).", MaxPauseMinutes),
			Edit:  true,
			Toast: "Error",
		}, false
	}

	dur := time.Duration(mins) * time.Minute

	if rest == "a" {
		for _, event := range handler.Sub.Events.Names() {
			_ = handler.Sub.Events.Pause(event, dur)
		}

		msg := fmt.Sprintf("All notifications paused for %d minutes.", mins)
		if mins == 0 {
			msg = "All notifications are no longer paused."
		}

		return &Reply{
			Reply: msg,
			Edit:  true,
			Toast: "Paused",
			Keyboard: [][]Button{
				{{Label: "Pause again", Data: cbStopRoot}, {Label: "Done", Data: cbCancel}},
			},
		}, true
	}

	idx := atoiDefault(rest, -1)
	names := handler.Sub.Events.Names()
	if idx < 0 || idx >= len(names) {
		return &Reply{Reply: "Subscription gone.", Edit: true, Toast: "Missing"}, false
	}

	event := names[idx]
	msg := fmt.Sprintf("Paused '%s' for %d minutes.", formatSubLabel(event), mins)
	if mins == 0 {
		msg = fmt.Sprintf("'%s' is no longer paused.", formatSubLabel(event))
	}

	err := handler.Sub.Events.Pause(event, dur)
	if err != nil {
		msg = "You're not subscribed to: " + formatSubLabel(event)
	}

	return &Reply{
		Reply: msg,
		Edit:  true,
		Toast: "OK",
		Keyboard: [][]Button{
			{{Label: "Pause again", Data: cbStopRoot}, {Label: "Done", Data: cbCancel}},
		},
	}, true
}

func (c *Chat) delayWizardRoot(handler *Handler) *Reply {
	names := handler.Sub.Events.Names()
	if len(names) == 0 {
		return &Reply{
			Reply: "You don't have any subscriptions yet, so there's nothing to set a delay on.\n\n" +
				"Subscribe to a camera first, then come back here.",
			Edit: true,
			Keyboard: [][]Button{
				{{Label: "Subscribe", Data: cbSubRoot}},
				{{Label: "Cancel", Data: cbCancel}},
			},
		}
	}

	rows := make([][]Button, 0, len(names)+1)
	for i, name := range names {
		label := fmt.Sprintf("%s (%s)", formatSubLabel(name), eventDelay(handler.Sub.Events, name))
		rows = append(rows, []Button{{Label: label, Data: fmt.Sprintf("d:%d", i)}})
	}
	rows = append(rows, []Button{{Label: "Cancel", Data: cbCancel}})

	return &Reply{
		Reply: "How often should Motifini text you about the same camera?\n\n" +
			"After a video is sent for a subscription, further videos for that " +
			"same subscription are held back for the delay you choose. " +
			"In other words: clips from that camera (and detection type) will only be sent this often.\n\n" +
			"Pick a subscription to change:",
		Edit:     true,
		Keyboard: rows,
	}
}

func (c *Chat) delayWizardSeconds(idxStr string) *Reply {
	idx := atoiDefault(idxStr, -1)

	return &Reply{
		Reply: "Cooldown after each alert video.\n\n" +
			"Example: 60s means if Door·Human just sent you a clip, " +
			"you won't get another Door·Human clip for at least a minute " +
			"(even if motion keeps firing).",
		Edit: true,
		Keyboard: [][]Button{
			{
				{Label: "30s", Data: fmt.Sprintf("d:%d:30", idx)},
				{Label: "60s", Data: fmt.Sprintf("d:%d:60", idx)},
				{Label: "2 min", Data: fmt.Sprintf("d:%d:120", idx)},
			},
			{
				{Label: "5 min", Data: fmt.Sprintf("d:%d:300", idx)},
				{Label: "10 min", Data: fmt.Sprintf("d:%d:600", idx)},
			},
			{{Label: "« Back", Data: cbDelayRoot}, {Label: "Cancel", Data: cbCancel}},
		},
	}
}

func (c *Chat) delayWizardApply(handler *Handler, payload string) (*Reply, bool) {
	idxStr, secsStr, ok := strings.Cut(payload, ":")
	if !ok {
		return &Reply{Reply: "Bad delay pick.", Edit: true, Toast: "Error"}, false
	}

	idx := atoiDefault(idxStr, -1)
	secs := atoiDefault(secsStr, 60)
	names := handler.Sub.Events.Names()

	if idx < 0 || idx >= len(names) {
		return &Reply{Reply: "Subscription gone.", Edit: true, Toast: "Missing"}, false
	}

	event := names[idx]
	handler.Sub.Events.RuleSetD(event, "delay", time.Duration(secs)*time.Second)

	return &Reply{
		Reply: fmt.Sprintf(
			"Got it. After Motifini sends a clip for '%s', it will wait at least %ds "+
				"before sending another for that same subscription.",
			formatSubLabel(event), secs),
		Edit:  true,
		Toast: "Saved",
		Keyboard: [][]Button{
			{{Label: "Set another", Data: cbDelayRoot}, {Label: "Done", Data: cbCancel}},
		},
	}, true
}

func (c *Chat) subsWizardRoot(handler *Handler) *Reply {
	names := handler.Sub.Events.Names()

	var msg strings.Builder
	msg.WriteString("Your alert subscriptions.\n\n")
	msg.WriteString("Tap a subscription below to pause it, change how often clips arrive, or remove it.\n")

	if len(names) == 0 {
		msg.WriteString("\n(none yet — use Subscribe to start)")
	}

	rows := make([][]Button, 0, len(names)+2)
	for idx, event := range names {
		line := formatSubLabel(event)
		fmt.Fprintf(&msg, "\n• %s · every %v", line, eventDelay(handler.Sub.Events, event))

		if handler.Sub.Events.IsPaused(event) {
			until := time.Until(handler.Sub.Events.PauseTime(event)).Round(time.Second)
			fmt.Fprintf(&msg, " (paused %v)", until)
		}

		rows = append(rows, []Button{{Label: line, Data: fmt.Sprintf("l:%d", idx)}})
	}

	rows = append(rows, []Button{
		{Label: "Subscribe", Data: cbSubRoot},
		{Label: "Unsubscribe", Data: cbUnsubRoot},
	}, []Button{
		{Label: "Pause", Data: cbStopRoot},
		{Label: "Delay", Data: cbDelayRoot},
	}, []Button{
		{Label: "Done", Data: cbCancel},
	})

	return &Reply{Reply: msg.String(), Edit: true, Keyboard: rows}
}

func (c *Chat) subsWizardItem(handler *Handler, idxStr string) *Reply {
	idx := atoiDefault(idxStr, -1)
	names := handler.Sub.Events.Names()
	if idx < 0 || idx >= len(names) {
		return &Reply{Reply: "Subscription gone.", Edit: true, Toast: "Missing"}
	}

	event := names[idx]
	label := formatSubLabel(event)

	return &Reply{
		Reply: "Manage " + label + "\n\n" +
			"Pause = silence this subscription for a while.\n" +
			"Set delay = how often clips for this one may arrive.\n" +
			"Unsubscribe = stop getting these alerts for good.",
		Edit: true,
		Keyboard: [][]Button{
			{
				{Label: "Pause 10m", Data: fmt.Sprintf("t:10:%d", idx)},
				{Label: "Clear pause", Data: fmt.Sprintf("t:0:%d", idx)},
			},
			{
				{Label: "Set delay", Data: fmt.Sprintf("d:%d", idx)},
				{Label: "Unsubscribe", Data: fmt.Sprintf("u:%d", idx)},
			},
			{{Label: "« Back", Data: cbSubsRoot}, {Label: "Done", Data: cbCancel}},
		},
	}
}

func (c *Chat) helpWizardRoot() *Reply {
	return &Reply{
		Reply: `What do you want to do?

Here's what each button opens:

• Subscribe — start getting alert videos when a camera sees motion, a person, a vehicle, or an animal
• Unsubscribe — stop alerts you no longer want
• My subs — see what you're subscribed to; tap one to pause, change frequency, or remove it
• Pause — temporarily mute alerts (no clips for N minutes) without unsubscribing
• Snapshot — grab a still photo from a camera right now
• Video — grab a short live clip from a camera right now
• Cameras — browse cameras, then choose snapshot or video
• Events — system alerts (stream up/down, camera offline/online, SecuritySpy errors) and any custom events
• Delay — after a clip is sent for a subscription, wait this long before sending another
  for the same one (so you aren't flooded)

Tap a button below:`,
		Edit: true,
		Keyboard: [][]Button{
			{{Label: "Subscribe", Data: cbSubRoot}, {Label: "Unsubscribe", Data: cbUnsubRoot}},
			{{Label: "My subs", Data: cbSubsRoot}, {Label: "Pause", Data: cbStopRoot}},
			{{Label: "Snapshot", Data: cbPicsRoot}, {Label: "Video", Data: cbVidsRoot}},
			{{Label: "Cameras", Data: cbCamsRoot}, {Label: "Events", Data: cbEvtsRoot}},
			{{Label: "Delay", Data: cbDelayRoot}, {Label: "Done", Data: cbCancel}},
		},
	}
}

func (c *Chat) helpWizardRootFor(handler *Handler) *Reply {
	root := c.helpWizardRoot()
	if handler != nil && handler.Sub != nil && handler.Sub.Admin {
		// Insert Users before Done on the last row.
		rows := root.Keyboard
		if len(rows) > 0 {
			last := rows[len(rows)-1]
			if len(last) > 0 && last[len(last)-1].Data == cbCancel {
				last = append([]Button{{Label: "Users", Data: cbUsersRoot}}, last...)
				rows[len(rows)-1] = last
				root.Keyboard = rows
			}
		}
		root.Reply += "\n• Users (admin) — allow/deny/ignore/admin/delete subscribers"
	}

	return root
}
