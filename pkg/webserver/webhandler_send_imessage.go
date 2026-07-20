package webserver

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/davidnewhall/motifini/pkg/messenger"
	"github.com/gorilla/mux"
	"golift.io/securityspy/v2"
)

// /api/v1.0/send/telegram/video/{to}/{camera}" handler.
func (c *Config) sendVideoHandler(writer http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)
	recipients, name := vars["to"], vars["camera"]
	vals := map[string]string{
		"height":  request.FormValue("height"),
		"width":   request.FormValue("width"),
		"quality": request.FormValue("crf"),
		"time":    request.FormValue("time"),
		"rate":    request.FormValue("rate"),
		"size":    request.FormValue("size"),
		"acodec":  request.FormValue("acodec"),
		"vcodec":  request.FormValue("vcodec"),
	}
	reqID, code, reply := messenger.ReqID(messenger.IDLength), http.StatusOK, "OK"

	cam := c.SSpy.Cameras.ByName(name)
	if cam == nil {
		c.Debug.Printf("[%v] Invalid 'cam' provided: %v", reqID, name)

		code, reply = http.StatusInternalServerError, "ERROR: Camera not found in configuration!"
	}

	for t := range strings.SplitSeq(recipients, ",") {
		if t == "" || !contains(c.AllowedTo, t) {
			c.Debug.Printf("[%v] Invalid 'to' provided: %v", reqID, t)

			code, reply = http.StatusInternalServerError, "ERROR: Missing 'to' or 'cam'"
		}
	}

	if code == http.StatusOK {
		// TODO: make a channel with a queue for these.
		err := c.processVideoRequest(reqID, cam, recipients, vals, vars)
		if err != nil {
			code = http.StatusInternalServerError
			reply = "ERROR: " + err.Error()
		}
	}

	reply = "REQ ID: " + reqID + ", msg: " + reply + "\n"

	c.finishReq(writer, request, reqID, code, reply, "-")
}

func toInt(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

// Since this runs in a go routine it sort of defeats the purpose of the queue. sorta?
func (c *Config) processVideoRequest(
	reqID string, cam *securityspy.Camera, recipients string, formVals, vars map[string]string,
) error {
	path := c.TempDir + "motifini_relay_" + reqID + "_" + cam.Name + ".mov"

	audioCodec := strings.TrimSpace(formVals["acodec"])
	if audioCodec == "" {
		audioCodec = "aac"
	}

	videoCodec := strings.TrimSpace(formVals["vcodec"])
	if videoCodec == "" {
		videoCodec = cam.PreferredVCodec()
	}

	ops := &securityspy.VidOps{
		Height:  toInt(formVals["height"]),
		Width:   toInt(formVals["width"]),
		Quality: toInt(formVals["quality"]),
		FPS:     toInt(formVals["rate"]),
		ACodec:  audioCodec,
		VCodec:  videoCodec,
	}
	timeLength := parseVideoLength(formVals["time"])
	size, _ := strconv.ParseInt(formVals["size"], 10, 64)

	err := cam.SaveVideo(ops, timeLength, size, path)
	if err != nil {
		c.Error.Printf("[%v] SaveVideo: %v", reqID, err)
		return fmt.Errorf("SaveVideo: %w", err)
	}

	// Input data OK, video grabbed, send an attachment to each recipient.
	for t := range strings.SplitSeq(recipients, ",") {
		if vars["app"] == messenger.APITelegram {
			dest, _ := strconv.ParseInt(t, 10, 64)
			c.Msgs.SendTelegram(reqID, "", path, dest)
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
func (c *Config) sendPictureHandler(writer http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)
	recipients, name := strings.Split(vars["to"], ","), vars["camera"]
	reqID, code := messenger.ReqID(messenger.IDLength), http.StatusOK

	var reply string

	path := c.TempDir + "motifini_relay_" + reqID + "_" + name + ".jpg"

	// Check input data.
	for _, t := range recipients {
		if t == "" || !contains(c.AllowedTo, t) {
			c.Debug.Printf("[%v] Invalid 'to' provided: %v", reqID, t)

			code = http.StatusInternalServerError

			break
		}
	}

	cam := c.SSpy.Cameras.ByName(name)

	switch {
	case name == "" || code == http.StatusInternalServerError:
		code, reply = http.StatusInternalServerError, "ERROR: Missing 'to' or 'cam', name: "+name
		c.Debug.Printf("[%v] Invalid 'to' provided or 'cam' empty: %v", reqID, name)
	case cam == nil:
		code, reply = http.StatusInternalServerError, "ERROR: Camera not found: "+name
		c.Debug.Printf("[%v] Camera not found: %v", reqID, name)
	default:
		code, reply = c.sendPictureToRecipients(reqID, cam, path, recipients, vars)
	}

	// There's a better way to do this....
	c.finishReq(writer, request, reqID, code, reply, "-")
}

// sendPictureToRecipients saves a snapshot and delivers it to each recipient, returning
// the HTTP status code and reply message to send back to the caller.
func (c *Config) sendPictureToRecipients(
	reqID string, cam *securityspy.Camera, path string, recipients []string, vars map[string]string,
) (int, string) {
	err := cam.SaveJPEG(&securityspy.VidOps{}, path)
	if err != nil {
		c.Error.Printf("[%v] cam.SaveJPEG: %v", reqID, err)
		return http.StatusInternalServerError, "ERROR: " + err.Error()
	}

	// Input data OK, send a message to each recipient.
	for _, t := range recipients {
		if vars["app"] == messenger.APITelegram {
			dest, _ := strconv.ParseInt(t, 10, 64)
			c.Msgs.SendTelegram(reqID, "", path, dest)
		}
	}

	return http.StatusOK, "REQ ID: " + reqID + ", msg: OK\n"
}

// /api/v1.0/send/telegram/msg handler.
func (c *Config) sendMessageHandler(writer http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)
	recipients, msg := strings.Split(vars["to"], ","), vars["msg"]
	reqID, code, reply := messenger.ReqID(messenger.IDLength), http.StatusOK, "OK"

	// Check input data.
	for _, t := range recipients {
		if t == "" || !contains(c.AllowedTo, t) {
			c.Debug.Printf("[%v] Invalid 'to' provided: %v", reqID, t)

			code = http.StatusInternalServerError

			break
		}
	}

	if code == http.StatusInternalServerError || msg == "" {
		c.Debug.Printf("[%v] Invalid 'to' provided or 'msg' empty: %v", reqID, msg)

		code, reply = http.StatusInternalServerError, "ERROR: Missing 'to' or 'msg'"
	} else {
		// Input data OK, send a message to each recipient.
		for _, t := range recipients {
			if vars["app"] == messenger.APITelegram {
				dest, _ := strconv.ParseInt(t, 10, 64)
				c.Msgs.SendTelegram(reqID, msg, "", dest)
			}
		}
	}

	reply = "REQ ID: " + reqID + ", msg: " + reply + "\n"
	c.finishReq(writer, request, reqID, code, reply, "-")
}
