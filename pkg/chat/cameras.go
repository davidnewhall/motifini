package chat

import (
	"golift.io/securityspy/v2"
)

// allCameras returns SecuritySpy cameras, or nil when SS is unavailable.
func (c *Chat) allCameras() []*securityspy.Camera {
	if c == nil || c.SSpy == nil || c.SSpy.Cameras == nil {
		return nil
	}

	return c.SSpy.Cameras.All()
}

// cameraByName looks up a camera, or nil when SecuritySpy has no camera list yet.
func (c *Chat) cameraByName(name string) *securityspy.Camera {
	if c == nil || c.SSpy == nil || c.SSpy.Cameras == nil {
		return nil
	}

	return c.SSpy.Cameras.ByName(name)
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
