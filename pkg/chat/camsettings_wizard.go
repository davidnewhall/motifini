package chat

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Admin per-camera clip settings wizard (Telegram ≤64-byte callbacks).
//
// k              → camera list
// k:{idx}        → camera menu (scale / length / size)
// k:{idx}:s      → scale presets
// k:{idx}:l      → length presets
// k:{idx}:z      → size presets
// k:{idx}:s:half → apply scale
// k:{idx}:l:6    → apply length (seconds)
// k:{idx}:z:N    → apply size (bytes)

func (c *Chat) handleCamSetWizardCallback(handler *Handler, data string) (*Reply, bool, bool) {
	if data != cbCamSetRoot && !strings.HasPrefix(data, "k:") {
		return nil, false, false
	}

	if handler == nil || handler.Sub == nil || !handler.Sub.Admin {
		return &Reply{Reply: "Admins only.", Edit: true, Toast: "Nope"}, false, true
	}

	if data == cbCamSetRoot {
		return c.camSetWizardRoot(), false, true
	}

	payload := strings.TrimPrefix(data, "k:")
	parts := strings.Split(payload, ":")

	switch len(parts) {
	case 1:
		return c.camSetWizardCam(parts[0]), false, true
	case 2:
		return c.camSetWizardKind(parts[0], parts[1]), false, true
	case 3:
		reply, save := c.camSetWizardApply(payload)

		return reply, save, true
	default:
		return &Reply{Reply: "Bad clip-settings pick.", Edit: true, Toast: "Error"}, false, true
	}
}

func (c *Chat) camSetWizardKind(idxStr, kind string) *Reply {
	switch kind {
	case "s":
		return c.camSetWizardScale(idxStr)
	case "l":
		return c.camSetWizardLength(idxStr)
	case "z":
		return c.camSetWizardSize(idxStr)
	default:
		return &Reply{Reply: "Bad clip-settings pick.", Edit: true, Toast: "Error"}
	}
}

func (c *Chat) cmdCamSet(_ *Handler) (*Reply, error) {
	root := c.camSetWizardRoot()
	root.Edit = false

	return root, nil
}

func (c *Chat) camSetWizardRoot() *Reply {
	c.refreshCameras()
	cams := c.allCameras()
	if len(cams) == 0 {
		return c.noCamerasReply()
	}

	rows := make([][]Button, 0, len(cams)+1)
	var msg strings.Builder
	msg.WriteString("Per-camera clip settings (everyone gets the same clip).\n\n")

	for idx, cam := range cams {
		settings := GetCameraClipSettings(c.Subs, cam.Name)
		summary := FormatClipSettings(settings)
		fmt.Fprintf(&msg, "• %s — %s", cam.Name, summary)
		if frame := cameraFrameSize(cam); frame != "" {
			fmt.Fprintf(&msg, " (%s)", frame)
		}
		msg.WriteByte('\n')
		rows = append(rows, []Button{{
			Label: fmt.Sprintf("%s (%s)", cam.Name, summary),
			Data:  fmt.Sprintf("k:%d", idx),
		}})
	}

	rows = append(rows, []Button{{Label: "Done", Data: cbCancel}})

	return &Reply{Reply: msg.String(), Edit: true, Keyboard: rows}
}

func (c *Chat) camSetWizardCam(idxStr string) *Reply {
	idx := atoiDefault(idxStr, -1)
	cams := c.allCameras()
	if idx < 0 || idx >= len(cams) {
		return &Reply{Reply: "Camera gone — try again.", Edit: true, Toast: "Missing"}
	}

	cam := cams[idx]
	settings := GetCameraClipSettings(c.Subs, cam.Name)
	current := FormatClipSettings(settings)
	if frame := cameraFrameSize(cam); frame != "" {
		current += " (" + frame + ")"
	}

	return &Reply{
		Reply: fmt.Sprintf("%s clip settings\n\nCurrent: %s\n\nChoose what to change:",
			cam.Name, current),
		Edit: true,
		Keyboard: [][]Button{
			{
				{Label: "Scale", Data: fmt.Sprintf("k:%d:s", idx)},
				{Label: "Length", Data: fmt.Sprintf("k:%d:l", idx)},
				{Label: "Size", Data: fmt.Sprintf("k:%d:z", idx)},
			},
			{{Label: "« Cameras", Data: cbCamSetRoot}, {Label: "Done", Data: cbCancel}},
		},
	}
}

func (c *Chat) camSetWizardScale(idxStr string) *Reply {
	idx := atoiDefault(idxStr, -1)
	cams := c.allCameras()
	if idx < 0 || idx >= len(cams) {
		return &Reply{Reply: "Camera gone — try again.", Edit: true, Toast: "Missing"}
	}

	return &Reply{
		Reply: "Video scale relative to the camera's native resolution.\n\n" +
			"Full = native (may stream-copy HEVC).\n" +
			"Half = ½ height.\n" +
			"Quarter = ¼ height (smaller files, usually recompressed).",
		Edit: true,
		Keyboard: [][]Button{
			{
				{Label: "Full", Data: fmt.Sprintf("k:%d:s:%s", idx, ScaleFull)},
				{Label: "Half", Data: fmt.Sprintf("k:%d:s:%s", idx, ScaleHalf)},
				{Label: "Quarter", Data: fmt.Sprintf("k:%d:s:%s", idx, ScaleQuarter)},
			},
			{{Label: "« Back", Data: fmt.Sprintf("k:%d", idx)}, {Label: "Done", Data: cbCancel}},
		},
	}
}

func (c *Chat) camSetWizardLength(idxStr string) *Reply {
	idx := atoiDefault(idxStr, -1)
	cams := c.allCameras()
	if idx < 0 || idx >= len(cams) {
		return &Reply{Reply: "Camera gone — try again.", Edit: true, Toast: "Missing"}
	}

	secs := []int{2, 3, 4, 5, 6, 8, 10, 15}
	rows := make([][]Button, 0, 4)
	row := make([]Button, 0, 4)

	for _, sec := range secs {
		row = append(row, Button{
			Label: fmt.Sprintf("%ds", sec),
			Data:  fmt.Sprintf("k:%d:l:%d", idx, sec),
		})
		if len(row) == 4 {
			rows = append(rows, row)
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	rows = append(rows, []Button{
		{Label: "« Back", Data: fmt.Sprintf("k:%d", idx)},
		{Label: "Done", Data: cbCancel},
	})

	return &Reply{
		Reply:    "Max clip length (capture stops earlier if the size limit is hit first).",
		Edit:     true,
		Keyboard: rows,
	}
}

func (c *Chat) camSetWizardSize(idxStr string) *Reply {
	idx := atoiDefault(idxStr, -1)
	cams := c.allCameras()
	if idx < 0 || idx >= len(cams) {
		return &Reply{Reply: "Camera gone — try again.", Edit: true, Toast: "Missing"}
	}

	sizes := []int{
		500 * 1024,
		600 * 1024,
		800 * 1024,
		1 * 1024 * 1024,
		1200 * 1024,
		1572864,
		2 * 1024 * 1024,
		2560 * 1024,
		3 * 1024 * 1024,
	}
	rows := make([][]Button, 0, 4)
	row := make([]Button, 0, 3)

	for _, size := range sizes {
		row = append(row, Button{
			Label: formatByteSize(size),
			Data:  fmt.Sprintf("k:%d:z:%d", idx, size),
		})
		if len(row) == 3 {
			rows = append(rows, row)
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	rows = append(rows, []Button{
		{Label: "« Back", Data: fmt.Sprintf("k:%d", idx)},
		{Label: "Done", Data: cbCancel},
	})

	return &Reply{
		Reply:    "Max clip file size (capture stops when this is reached).",
		Edit:     true,
		Keyboard: rows,
	}
}

func (c *Chat) camSetWizardApply(payload string) (*Reply, bool) {
	parts := strings.Split(payload, ":")
	if len(parts) != 3 {
		return &Reply{Reply: "Bad clip-settings pick.", Edit: true, Toast: "Error"}, false
	}

	idx := atoiDefault(parts[0], -1)
	kind := parts[1]
	value := parts[2]
	cams := c.allCameras()

	if idx < 0 || idx >= len(cams) {
		return &Reply{Reply: "Camera gone.", Edit: true, Toast: "Missing"}, false
	}

	cam := cams[idx]
	EnsureCameraSettings(c.Subs, cam.Name)
	key := CamSettingsKey(cam.Name)

	switch kind {
	case "s":
		if !validScale(value) {
			return &Reply{Reply: "Bad scale.", Edit: true, Toast: "Error"}, false
		}

		c.Subs.Events.RuleSetS(key, ruleScale, value)
	case "l":
		secs, err := strconv.Atoi(value)
		if err != nil || !allowedClipLengthSecs(secs) {
			return &Reply{
				Reply: fmt.Sprintf("Length must be %d–%ds.", MinClipLengthSecs, MaxClipLengthSecs),
				Edit:  true,
				Toast: "Error",
			}, false
		}

		c.Subs.Events.RuleSetD(key, ruleLength, time.Duration(secs)*time.Second)
	case "z":
		size, err := strconv.Atoi(value)
		if err != nil || !allowedClipSizeBytes(size) {
			return &Reply{
				Reply: fmt.Sprintf("Size must be %s–%s.",
					formatByteSize(MinClipSizeBytes), formatByteSize(MaxClipSizeBytes)),
				Edit:  true,
				Toast: "Error",
			}, false
		}

		c.Subs.Events.RuleSetI(key, ruleSize, size)
	default:
		return &Reply{Reply: "Bad clip-settings pick.", Edit: true, Toast: "Error"}, false
	}

	settings := GetCameraClipSettings(c.Subs, cam.Name)
	next := c.camSetWizardCam(strconv.Itoa(idx))
	next.Reply = fmt.Sprintf("Updated %s → %s\n\n", cam.Name, FormatClipSettings(settings)) +
		fmt.Sprintf("%s clip settings\n\nCurrent: %s\n\nChoose what to change:",
			cam.Name, FormatClipSettings(settings))
	next.Toast = "Saved"

	return next, true
}
