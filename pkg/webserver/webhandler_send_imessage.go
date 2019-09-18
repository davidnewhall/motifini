package webserver

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/davidnewhall/motifini/pkg/export"
	"github.com/davidnewhall/motifini/pkg/messenger"
	"github.com/gorilla/mux"
	"golift.io/imessage"
	"golift.io/securityspy"
)

///api/v1.0/send/imessage/video/{to}/{camera}"
func (c *Config) sendVideoHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	to, name := vars["to"], vars["camera"]
	vals := map[string]string{
		"height":  r.FormValue("height"),
		"width":   r.FormValue("width"),
		"quality": r.FormValue("crf"),
		"time":    r.FormValue("time"),
		"rate":    r.FormValue("rate"),
		"size":    r.FormValue("size"),
	}
	id, code, reply := messenger.ReqID(4), 200, "OK"
	cam := c.SSpy.Cameras.ByName(name)
	if cam == nil {
		c.Debug.Printf("[%v] Invalid 'cam' provided: %v", id, name)
		code, reply = 500, "ERROR: Camera not found in configuration!"
	}
	for _, t := range strings.Split(to, ",") {
		if t == "" || !contains(c.AllowedTo, t) {
			c.Debug.Printf("[%v] Invalid 'to' provided: %v", id, t)
			code, reply = 500, "ERROR: Missing 'to' or 'cam'"
		}
	}
	if code == 200 {
		go c.processVideoRequest(id, cam, to, vals)
	}
	reply = "REQ ID: " + id + ", msg: " + reply + "\n"
	c.finishReq(w, r, id, code, reply, "-")
}

// Since this runs in a go routine it sort of defeats the purpose of the queue. sorta?
func (c *Config) processVideoRequest(id string, cam *securityspy.Camera, to string, v map[string]string) {
	path := c.TempDir + "imessage_relay_" + id + "_" + cam.Name + ".mov"
	toInt := func(s string) (i int) { i, _ = strconv.Atoi(s); return }
	ops := &securityspy.VidOps{
		Height:  toInt(v["height"]),
		Width:   toInt(v["width"]),
		Quality: toInt(v["crf"]),
		FPS:     toInt(v["rate"]),
	}
	time, _ := time.ParseDuration(v["time"])
	size, _ := strconv.ParseInt(v["size"], 10, 64)
	if err := cam.SaveVideo(ops, time, size, path); err != nil {
		c.Error.Printf("[%v] SaveVideo: %v", id, err)
	}
	// Input data OK, video grabbed, send an attachment to each recipient.
	for _, t := range strings.Split(to, ",") {
		c.Msgs.SendiMessage(imessage.Outgoing{ID: id, To: t, Text: path, File: true, Call: c.Msgs.FileCallback})
	}
}

// /api/v1.0/send/imessage/picture/{to}/{camera}
func (c *Config) sendPictureHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	to, name := strings.Split(vars["to"], ","), vars["camera"]
	id, code, reply := messenger.ReqID(4), 200, "OK"
	path := c.TempDir + "imessage_relay_" + id + "_" + name + ".jpg"
	// Check input data.
	for _, t := range to {
		if t == "" || !contains(c.AllowedTo, t) {
			c.Debug.Printf("[%v] Invalid 'to' provided: %v", id, t)
			code = 500
			break
		}
	}
	if name == "" || code == 500 {
		code, reply = 500, "ERROR: Missing 'to' or 'cam'"
		c.Debug.Printf("[%v] Invalid 'to' provided or 'cam' empty: %v", id, name)

	} else if cam := c.SSpy.Cameras.ByName(name); cam == nil {
		code, reply = 500, "ERROR: Camera not found: "+name
		c.Debug.Printf("[%v] Camera not found: %v", id, name)

	} else if err := cam.SaveJPEG(&securityspy.VidOps{}, path); err != nil {
		c.Error.Printf("[%v] cam.SaveJPEG: %v", id, err)
		code, reply = 500, "ERROR: "+err.Error()

	} else {
		// Input data OK, send a message to each recipient.
		for _, t := range to {
			c.Msgs.SendiMessage(imessage.Outgoing{ID: id, To: t, Text: path, File: true, Call: c.Msgs.FileCallback})
		}
		reply = "REQ ID: " + id + ", msg: " + reply + "\n"
	}
	// There's a better way to do this....
	c.finishReq(w, r, id, code, reply, "-")
}

// /api/v1.0/send/imessage/msg
func (c *Config) sendMessageHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	to, msg := strings.Split(vars["to"], ","), vars["msg"]
	id, code, reply := messenger.ReqID(4), 200, "OK"
	// Check input data.
	for _, t := range to {
		if t == "" || !contains(c.AllowedTo, t) {
			c.Debug.Printf("[%v] Invalid 'to' provided: %v", id, t)
			code = 500
			break
		}
	}
	callback := func(msg *imessage.Response) {
		if msg.Errs != nil {
			export.Map.Errors.Add(1)
			c.Error.Printf("[%v] msgs.Msgs.Send '%v': %v", msg.ID, msg.To, msg.Errs)
			return
		}
		c.Info.Printf("[%v] iMessage (%d chars) sent to: %v", msg.ID, len(msg.Text), msg.To)
	}
	if code == 500 || msg == "" {
		c.Debug.Printf("[%v] Invalid 'to' provided or 'msg' empty: %v", id, msg)
		code, reply = 500, "ERROR: Missing 'to' or 'msg'"
	} else {
		// Input data OK, send a message to each recipient.
		for _, t := range to {
			c.Msgs.SendiMessage(imessage.Outgoing{ID: id, To: t, Text: msg, File: false, Call: callback})
		}
	}
	reply = "REQ ID: " + id + ", msg: " + reply + "\n"
	c.finishReq(w, r, id, code, reply, "-")
}
