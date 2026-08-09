package chat

import (
	"golift.io/securityspy/v2"
)

// cameras returns the current camera list, or nil when SecuritySpy is
// unavailable. GetCameras() is the concurrency-safe read: Refresh() replaces
// the list, and it runs from the retry loop, the event stream and /refresh.
func (c *Chat) cameras() *securityspy.Cameras {
	if c == nil || c.SSpy == nil {
		return nil
	}

	return c.SSpy.GetCameras()
}

// allCameras returns SecuritySpy cameras, or nil when SS is unavailable.
func (c *Chat) allCameras() []*securityspy.Camera {
	cams := c.cameras()
	if cams == nil {
		return nil
	}

	return cams.All()
}

// cameraByName looks up a camera, or nil when SecuritySpy has no camera list yet.
func (c *Chat) cameraByName(name string) *securityspy.Camera {
	cams := c.cameras()
	if cams == nil {
		return nil
	}

	return cams.ByName(name)
}

// noCamerasReply is shown when SecuritySpy has not loaded any cameras yet.
func (c *Chat) noCamerasReply() *Reply {
	return &Reply{
		Reply: "SecuritySpy isn't ready (no cameras loaded).\n\n" +
			"Check the [security_spy] config and connection, then try again.",
		Edit:  true,
		Toast: "Offline",
		Keyboard: [][]Button{
			{{Label: "Done", Data: cbCancel}},
		},
	}
}

// refreshCameras best-effort refreshes SecuritySpy; safe when SSpy is nil.
func (c *Chat) refreshCameras() {
	if c == nil || c.SSpy == nil {
		return
	}

	_ = c.SSpy.Refresh()
}
