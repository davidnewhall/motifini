package webserver

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/davidnewhall/motifini/pkg/messenger"
	"github.com/gorilla/mux"
	"golift.io/securityspy"
)

// /api/v1.0/send/telegram/video/{to}/{camera}" handler.
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
	id, code, reply := messenger.ReqID(messenger.IDLength), http.StatusOK, "OK"

	cam := c.SSpy.Cameras.ByName(name)
	if cam == nil {
		c.Debug.Printf("[%v] Invalid 'cam' provided: %v", id, name)

		code, reply = http.StatusInternalServerError, "ERROR: Camera not found in configuration!"
	}

	for _, t := range strings.Split(to, ",") {
		if t == "" || !contains(c.AllowedTo, t) {
			c.Debug.Printf("[%v] Invalid 'to' provided: %v", id, t)

			code, reply = http.StatusInternalServerError, "ERROR: Missing 'to' or 'cam'"
		}
	}

	if code == http.StatusOK {
		// TODO: make a channel with a queue for these.
		if err := c.processVideoRequest(id, cam, to, vals, vars); err != nil {
			code = http.StatusInternalServerError
			reply = "ERROR: " + err.Error()
		}
	}

	reply = "REQ ID: " + id + ", msg: " + reply + "\n"

	c.finishReq(w, r, id, code, reply, "-")
}

func toInt(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

// Since this runs in a go routine it sort of defeats the purpose of the queue. sorta?
func (c *Config) processVideoRequest(id string, cam *securityspy.Camera, to string, v, vars map[string]string) error {
	path := c.TempDir + "motifini_relay_" + id + "_" + cam.Name + ".mov"
	ops := &securityspy.VidOps{
		Height:  toInt(v["height"]),
		Width:   toInt(v["width"]),
		Quality: toInt(v["quality"]),
		FPS:     toInt(v["rate"]),
	}
	time := parseVideoLength(v["time"])
	size, _ := strconv.ParseInt(v["size"], 10, 64)

	if err := cam.SaveVideo(ops, time, size, path); err != nil {
		c.Error.Printf("[%v] SaveVideo: %v", id, err)
		return err
	}

	// Input data OK, video grabbed, send an attachment to each recipient.
	for _, t := range strings.Split(to, ",") {
		switch vars["app"] {
		case messenger.APITelegram:
			to, _ := strconv.ParseInt(t, 10, 64) //nolint:gomnd
			c.Msgs.SendTelegram(id, "", path, to)
		}
	}

	return nil
}

func parseVideoLength(input string) time.Duration {
	if input == "" {
		return 0
	}

	dur, err := time.ParseDuration(input)
	if err == nil {
		return dur
	}

	// Accept plain seconds like "5" in addition to Go duration strings like "5s".
	seconds, err := strconv.Atoi(input)
	if err != nil {
		return 0
	}

	return time.Duration(seconds) * time.Second
}

// /api/v1.0/send/telegram/picture/{to}/{camera} handler.
func (c *Config) sendPictureHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	to, name := strings.Split(vars["to"], ","), vars["camera"]
	id, code, reply := messenger.ReqID(messenger.IDLength), http.StatusOK, "OK"
	path := c.TempDir + "motifini_relay_" + id + "_" + name + ".jpg"

	// Check input data.
	for _, t := range to {
		if t == "" || !contains(c.AllowedTo, t) {
			c.Debug.Printf("[%v] Invalid 'to' provided: %v", id, t)

			code = http.StatusInternalServerError

			break
		}
	}

	if name == "" || code == http.StatusInternalServerError {
		code, reply = http.StatusInternalServerError, "ERROR: Missing 'to' or 'cam'"
		c.Debug.Printf("[%v] Invalid 'to' provided or 'cam' empty: %v", id, name)
	} else if cam := c.SSpy.Cameras.ByName(name); cam == nil {
		code, reply = http.StatusInternalServerError, "ERROR: Camera not found: "+name
		c.Debug.Printf("[%v] Camera not found: %v", id, name)
	} else if err := cam.SaveJPEG(&securityspy.VidOps{}, path); err != nil {
		c.Error.Printf("[%v] cam.SaveJPEG: %v", id, err)
		code, reply = http.StatusInternalServerError, "ERROR: "+err.Error()
	} else {
		// Input data OK, send a message to each recipient.
		for _, t := range to {
			switch vars["app"] {
			case messenger.APITelegram:
				to, _ := strconv.ParseInt(t, 10, 64) //nolint:gomnd
				c.Msgs.SendTelegram(id, "", path, to)
			}
		}

		reply = "REQ ID: " + id + ", msg: " + reply + "\n"
	}

	// There's a better way to do this....
	c.finishReq(w, r, id, code, reply, "-")
}

// /api/v1.0/send/telegram/msg handler.
func (c *Config) sendMessageHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	to, msg := strings.Split(vars["to"], ","), vars["msg"]
	id, code, reply := messenger.ReqID(messenger.IDLength), http.StatusOK, "OK"

	// Check input data.
	for _, t := range to {
		if t == "" || !contains(c.AllowedTo, t) {
			c.Debug.Printf("[%v] Invalid 'to' provided: %v", id, t)

			code = http.StatusInternalServerError

			break
		}
	}

	if code == http.StatusInternalServerError || msg == "" {
		c.Debug.Printf("[%v] Invalid 'to' provided or 'msg' empty: %v", id, msg)

		code, reply = http.StatusInternalServerError, "ERROR: Missing 'to' or 'msg'"
	} else {
		// Input data OK, send a message to each recipient.
		for _, t := range to {
			switch vars["app"] {
			case messenger.APITelegram:
				to, _ := strconv.ParseInt(t, 10, 64) //nolint:gomnd
				c.Msgs.SendTelegram(id, msg, "", to)
			}
		}
	}

	reply = "REQ ID: " + id + ", msg: " + reply + "\n"
	c.finishReq(w, r, id, code, reply, "-")
}
