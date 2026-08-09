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
		c.catalog.Lock()
		c.Subs.EventRemove(vars["event"])
		err := chat.SaveState(c.Subs)
		c.catalog.Unlock()

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

	if event == "" || chat.IsCamSettingsKey(event) || len(event) > chat.MaxEventNameLen {
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

	c.catalog.Lock()
	created := !c.Subs.Events.Exists(event)
	c.upsertCatalogEvent(event, description)

	// Save on every accepted PUT: if a previous save failed, a retry carrying
	// the same payload would otherwise skip the save and stay stale on disk.
	err = chat.SaveState(c.Subs)
	c.catalog.Unlock()

	if err != nil {
		c.finishReq(writer, request, reqID, http.StatusInternalServerError,
			"ERROR: "+err.Error()+"\n", "register")

		return
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
func (c *Config) upsertCatalogEvent(event, description string) {
	if !c.Subs.Events.Exists(event) {
		if description == "" {
			description = event
		}

		_ = c.Subs.Events.New(event, &subscribe.Rules{
			S: map[string]string{"description": description, "source": chat.EventSourceHA},
		})

		return
	}

	if description != "" {
		if current, _ := c.Subs.Events.RuleGetS(event, "description"); current != description {
			c.Subs.Events.RuleSetS(event, "description", description)
		}
	}

	// Claim legacy catalog events that predate the source rule.
	if _, ok := c.Subs.Events.RuleGetS(event, "source"); !ok {
		c.Subs.Events.RuleSetS(event, "source", chat.EventSourceHA)
	}
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
	if !chat.IsCamSettingsKey(req.event) {
		err := c.registerNotifyEvent(req.event, request.FormValue("description"))
		if err != nil {
			c.finishReq(writer, request, reqID, http.StatusInternalServerError,
				"ERROR: "+err.Error()+"\n", "notify")

			return
		}
	}

	subs := c.Subs.GetSubscribers(req.event)
	msg, path, code, reply := c.captureNotifyMedia(reqID, req, len(subs))

	c.Msgs.SendFileOrMsg(reqID, msg, path, subs)
	c.finishReq(writer, request, reqID, code, reply, msg)
}

// registerNotifyEvent adds an event the catalog has never seen, so the Telegram
// subscribe menus pick it up. The check, create, save and rollback all run under
// one lock: a save failure rolls the new event back, and without the lock that
// rollback could delete an event another request just created and saved.
func (c *Config) registerNotifyEvent(event, description string) error {
	c.catalog.Lock()
	defer c.catalog.Unlock()

	if c.Subs.Events.Exists(event) {
		return nil
	}

	c.upsertCatalogEvent(event, description)

	err := chat.SaveState(c.Subs)
	if err != nil {
		// Roll back so a later notify retries the registration instead of the
		// event living in memory only and disappearing on restart.
		c.Subs.Events.Remove(event)

		return fmt.Errorf("saving new event: %w", err)
	}

	return nil
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

	if req.event == "" || len(req.event) > chat.MaxEventNameLen {
		return nil, http.StatusBadRequest, "ERROR: invalid event name\n"
	}

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

// captureNotifyMedia grabs the requested photo or video clip for a notify and
// returns the message to deliver plus the media path (empty for text-only).
// A camera known to be offline skips the capture attempt entirely; the message
// still goes out with an offline note appended (200 — delivery succeeded,
// degraded). A capture failure on an online camera also appends a note and
// falls back to text-only, but returns 500 so the automation knows.
func (c *Config) captureNotifyMedia(
	reqID string, req *notifyRequest, subCount int,
) (string, string, int, string) {
	if req.media == mediaNone || subCount == 0 {
		return req.msg, "", http.StatusOK, "REQ ID: " + reqID + ", msg: got notify\n"
	}

	if !req.cam.Connected.Val {
		c.Error.Printf("[%v] %s is offline; skipping %s capture for %s",
			reqID, req.cam.Name, req.media, req.event)
		note := "⚠ " + req.cam.Name + " is offline — no " + req.media + " attached."

		return appendNote(req.msg, note), "", http.StatusOK,
			"REQ ID: " + reqID + ", msg: got notify (camera offline, text only)\n"
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
		note := "⚠ Couldn't capture " + req.media + " from " + req.cam.Name + "."

		return appendNote(req.msg, note), "", http.StatusInternalServerError, "ERROR: " + err.Error() + "\n"
	}

	return req.msg, path, http.StatusOK, "REQ ID: " + reqID + ", msg: got notify\n"
}

// appendNote adds a delivery note to a notify message (the note becomes the
// message when the notify carried no text).
func appendNote(msg, note string) string {
	if msg == "" {
		return note
	}

	return msg + "\n\n" + note
}
