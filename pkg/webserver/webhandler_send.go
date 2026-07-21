package webserver

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/davidnewhall/motifini/pkg/chat"
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

	cam := c.cameraByName(name)
	if !c.securitySpyReady() {
		c.Debug.Printf("[%v] SecuritySpy cameras not loaded yet", reqID)
		code, reply = http.StatusServiceUnavailable, "ERROR: SecuritySpy not ready (cameras not loaded)"
	} else if cam == nil {
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

func (c *Config) processVideoRequest(
	reqID string, cam *securityspy.Camera, recipients string, formVals, vars map[string]string,
) error {
	path := filepath.Join(c.TempDir, "motifini_relay_"+reqID+"_"+cam.Name+".mov")

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
	defer os.Remove(path) // SendTelegram no longer deletes; clean up after all recipients.

	// Input data OK, video grabbed, send an attachment to each recipient.
	for t := range strings.SplitSeq(recipients, ",") {
		if vars["app"] == messenger.APITelegram {
			dest, _ := strconv.ParseInt(t, 10, 64)
			c.Msgs.SendTelegram(reqID, chat.CameraCaption(cam.Name, chat.CaptionVideo), path, dest, c.telegramContact(dest))
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

	path := filepath.Join(c.TempDir, "motifini_relay_"+reqID+"_"+name+".jpg")

	// Check input data.
	for _, t := range recipients {
		if t == "" || !contains(c.AllowedTo, t) {
			c.Debug.Printf("[%v] Invalid 'to' provided: %v", reqID, t)

			code = http.StatusInternalServerError

			break
		}
	}

	cam := c.cameraByName(name)

	switch {
	case name == "" || code == http.StatusInternalServerError:
		code, reply = http.StatusInternalServerError, "ERROR: Missing 'to' or 'cam', name: "+name
		c.Debug.Printf("[%v] Invalid 'to' provided or 'cam' empty: %v", reqID, name)
	case !c.securitySpyReady():
		code, reply = http.StatusServiceUnavailable, "ERROR: SecuritySpy not ready (cameras not loaded)"
		c.Debug.Printf("[%v] SecuritySpy cameras not loaded yet", reqID)
	case cam == nil:
		code, reply = http.StatusInternalServerError, "ERROR: Camera not found: "+name
		c.Debug.Printf("[%v] Camera not found: %v", reqID, name)
	default:
		code, reply = c.sendPictureToRecipients(reqID, cam, path, recipients, vars)
	}

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
	defer os.Remove(path) // SendTelegram no longer deletes; clean up after all recipients.

	// Input data OK, send a message to each recipient.
	for _, t := range recipients {
		if vars["app"] == messenger.APITelegram {
			dest, _ := strconv.ParseInt(t, 10, 64)
			c.Msgs.SendTelegram(reqID, chat.CameraCaption(cam.Name, chat.CaptionPhoto), path, dest, c.telegramContact(dest))
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
				c.Msgs.SendTelegram(reqID, msg, "", dest, c.telegramContact(dest))
			}
		}
	}

	reply = "REQ ID: " + reqID + ", msg: " + reply + "\n"
	c.finishReq(writer, request, reqID, code, reply, "-")
}

func (c *Config) telegramContact(id int64) string {
	if c.Subs == nil {
		return ""
	}

	sub, err := c.Subs.GetSubscriberByID(id, messenger.APITelegram)
	if err != nil || sub == nil {
		return ""
	}

	return sub.Contact
}
