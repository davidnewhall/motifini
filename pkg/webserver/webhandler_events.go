package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/davidnewhall/motifini/pkg/chat"
	"github.com/davidnewhall/motifini/pkg/messenger"
	"github.com/gorilla/mux"
	"golift.io/securityspy/v2"
	"golift.io/subscribe"
)

// Media kinds accepted by the event notify endpoint (media= form field).
const (
	mediaNone  = "none"
	mediaPhoto = "photo"
	mediaVideo = "video"
)

// /api/v1.0/event/{cmd:remove|notify}/{event} handler.
func (c *Config) eventsHandler(writer http.ResponseWriter, request *http.Request) {
	reqID, vars := messenger.ReqID(messenger.IDLength), mux.Vars(request)

	switch cmd := strings.ToLower(vars["cmd"]); cmd {
	case "remove":
		c.Subs.EventRemove(vars["event"])
		err := c.Subs.StateFileSave()
		if err != nil {
			c.finishReq(writer, request, reqID, http.StatusInternalServerError,
				"ERROR: "+err.Error()+"\n", cmd)

			return
		}
		c.finishReq(writer, request, reqID, http.StatusOK,
			"OK: removed event "+vars["event"]+"\n", cmd)
	case "notify":
		c.notifyHandler(reqID, vars, writer, request)
	default:
		c.finishReq(writer, request, reqID, http.StatusNotImplemented,
			"ERROR: unsupported event command\n", cmd)
	}
}

// eventUpsertHandler handles PUT /api/v1.0/event/{event}: register or update a
// catalog event so it appears in the Telegram subscribe menus. Accepts a
// "description" form field or a JSON body like {"description": "..."}.
func (c *Config) eventUpsertHandler(writer http.ResponseWriter, request *http.Request) {
	reqID, event := messenger.ReqID(messenger.IDLength), mux.Vars(request)["event"]

	if event == "" || chat.IsCamSettingsKey(event) {
		c.finishReq(writer, request, reqID, http.StatusBadRequest,
			"ERROR: invalid event name\n", "register")

		return
	}

	description, err := eventDescription(request)
	if err != nil {
		c.finishReq(writer, request, reqID, http.StatusBadRequest,
			"ERROR: "+err.Error()+"\n", "register")

		return
	}

	created := !c.Subs.Events.Exists(event)
	if c.upsertCatalogEvent(event, description) {
		err := c.Subs.StateFileSave()
		if err != nil {
			c.finishReq(writer, request, reqID, http.StatusInternalServerError,
				"ERROR: "+err.Error()+"\n", "register")

			return
		}
	}

	verb := "updated"
	if created {
		verb = "registered"
	}

	c.finishReq(writer, request, reqID, http.StatusOK,
		"OK: "+verb+" event "+event+"\n", "register")
}

// eventDescription reads the event description from a form field or a JSON body.
func eventDescription(request *http.Request) (string, error) {
	if !strings.HasPrefix(request.Header.Get("Content-Type"), "application/json") {
		return request.FormValue("description"), nil
	}

	var body struct {
		Description string `json:"description"`
	}

	err := json.NewDecoder(request.Body).Decode(&body)
	if err != nil {
		return "", fmt.Errorf("decoding JSON body: %w", err)
	}

	return body.Description, nil
}

// upsertCatalogEvent registers or updates a catalog event from the HTTP API.
// New events are tagged source=ha so the Telegram menus group them under Home
// Assistant; existing events keep their rules and get the description updated.
// Returns true when the catalog changed and should be saved.
func (c *Config) upsertCatalogEvent(event, description string) bool {
	if !c.Subs.Events.Exists(event) {
		if description == "" {
			description = event
		}

		_ = c.Subs.Events.New(event, &subscribe.Rules{
			S: map[string]string{"description": description, "source": chat.EventSourceHA},
		})

		return true
	}

	changed := false

	if description != "" {
		if current, _ := c.Subs.Events.RuleGetS(event, "description"); current != description {
			c.Subs.Events.RuleSetS(event, "description", description)
			changed = true
		}
	}

	// Claim legacy catalog events that predate the source rule.
	if _, ok := c.Subs.Events.RuleGetS(event, "source"); !ok {
		c.Subs.Events.RuleSetS(event, "source", chat.EventSourceHA)
		changed = true
	}

	return changed
}

// eventListEntry is one catalog event in the GET /api/v1.0/events response.
type eventListEntry struct {
	Event       string `json:"event"`
	Description string `json:"description"`
	Source      string `json:"source"`
	Subscribers int    `json:"subscribers"`
}

// eventsListHandler handles GET /api/v1.0/events: list the subscribable event
// catalog as JSON. Reserved camera clip settings keys are excluded.
func (c *Config) eventsListHandler(writer http.ResponseWriter, request *http.Request) {
	reqID, names := messenger.ReqID(messenger.IDLength), chat.CatalogEventNames(c.Subs.Events)
	events := make([]eventListEntry, 0, len(names))

	for _, name := range names {
		description, _ := c.Subs.Events.RuleGetS(name, "description")
		source, _ := c.Subs.Events.RuleGetS(name, "source")

		events = append(events, eventListEntry{
			Event:       name,
			Description: description,
			Source:      source,
			Subscribers: len(c.Subs.GetSubscribers(name)),
		})
	}

	reply, err := json.Marshal(events)
	if err != nil {
		c.finishReq(writer, request, reqID, http.StatusInternalServerError,
			"ERROR: "+err.Error()+"\n", "list")

		return
	}

	c.finishReqJSON(writer, request, reqID, http.StatusOK, reply, "list")
}

// notifyRequest holds the validated parameters of an event notify call.
type notifyRequest struct {
	event string
	msg   string
	media string
	cam   *securityspy.Camera
}

// notifyHandler handles POST /api/v1.0/event/notify/{event}.
//
// Form fields:
//
//	msg — optional text / caption (empty allowed)
//	camera — SecuritySpy camera name or number; required when media is photo/video
//	media — none | photo | video (default: photo when camera is set, else none)
//	description — optional catalog description, used only when the event is new
//
// A request with neither msg nor camera media is rejected (400). Unknown events
// are registered in the catalog (source=ha) so Telegram menus pick them up.
func (c *Config) notifyHandler(
	reqID string, vars map[string]string, writer http.ResponseWriter, request *http.Request,
) {
	req, code, reply := c.parseNotifyRequest(vars, request)
	if code != http.StatusOK {
		c.finishReq(writer, request, reqID, code, reply, "notify")

		return
	}

	// Register unknown events so they appear in the Telegram subscribe menus.
	if !chat.IsCamSettingsKey(req.event) && !c.Subs.Events.Exists(req.event) {
		c.upsertCatalogEvent(req.event, request.FormValue("description"))

		err := c.Subs.StateFileSave()
		if err != nil {
			c.Error.Printf("[%v] StateFileSave: %v", reqID, err)
		}
	}

	subs := c.Subs.GetSubscribers(req.event)
	path, code, reply := c.captureNotifyMedia(reqID, req, len(subs))

	c.Msgs.SendFileOrMsg(reqID, req.msg, path, subs)
	c.finishReq(writer, request, reqID, code, reply, req.msg)
}

// parseNotifyRequest validates the msg / media / camera combination and
// resolves the camera when media was requested.
func (c *Config) parseNotifyRequest(
	vars map[string]string, request *http.Request,
) (*notifyRequest, int, string) {
	req := &notifyRequest{
		event: vars["event"],
		msg:   request.FormValue("msg"),
		media: strings.ToLower(strings.TrimSpace(request.FormValue("media"))),
	}
	camera := strings.TrimSpace(request.FormValue("camera"))

	if req.media == "" {
		req.media = mediaNone
		if camera != "" {
			req.media = mediaPhoto
		}
	}

	switch req.media {
	case mediaNone:
		// camera is ignored on purpose: explicit none means text-only.
		if req.msg == "" {
			return nil, http.StatusBadRequest,
				"ERROR: provide msg, or camera with media=photo|video\n"
		}
	case mediaPhoto, mediaVideo:
		if camera == "" {
			return nil, http.StatusBadRequest,
				"ERROR: camera is required when media is " + req.media + "\n"
		}
	default:
		return nil, http.StatusBadRequest,
			"ERROR: media must be none, photo, or video\n"
	}

	if req.media == mediaNone {
		return req, http.StatusOK, ""
	}

	if !c.securitySpyReady() {
		return nil, http.StatusServiceUnavailable,
			"ERROR: SecuritySpy not ready (cameras not loaded)\n"
	}

	req.cam = c.cameraByNameOrNum(camera)
	if req.cam == nil {
		return nil, http.StatusBadRequest, "ERROR: camera not found: " + camera + "\n"
	}

	return req, http.StatusOK, ""
}

// captureNotifyMedia grabs the requested photo or video clip for a notify.
// Returns an empty path when no media was requested or no one is subscribed.
// On capture failure the notify still goes out (text-only) and the caller gets
// a 500 so the automation knows the media is missing.
func (c *Config) captureNotifyMedia(reqID string, req *notifyRequest, subCount int) (string, int, string) {
	if req.media == mediaNone || subCount == 0 {
		return "", http.StatusOK, "REQ ID: " + reqID + ", msg: got notify\n"
	}

	ext := ".jpg"
	if req.media == mediaVideo {
		ext = ".mp4"
	}

	path := filepath.Join(c.TempDir, "motifini_relay_"+reqID+"_"+req.cam.Name+ext)

	var err error
	if req.media == mediaVideo {
		settings := chat.GetCameraClipSettings(c.Subs, req.cam.Name)
		err = req.cam.SaveVideo(chat.VideoClipOps(req.cam, settings), settings.Length, int64(settings.Size), path)
	} else {
		err = req.cam.SaveJPEG(&securityspy.VidOps{}, path)
	}

	if err != nil {
		c.Error.Printf("[%v] capture %s for %s: %v", reqID, req.media, req.event, err)
		// Text-only fallback; never attach a missing/partial file.
		return "", http.StatusInternalServerError, "ERROR: " + err.Error() + "\n"
	}

	return path, http.StatusOK, "REQ ID: " + reqID + ", msg: got notify\n"
}
