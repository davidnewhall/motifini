package main

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/davidnewhall/motifini/encode"
	"github.com/davidnewhall/motifini/messages"
	"github.com/gorilla/mux"
)

///api/v1.0/send/imessage/video/{to}/{camera}"
func (c *Config) sendVideoHandler(w http.ResponseWriter, r *http.Request) {
	c.export.httpVisits.Add(1)
	vars := mux.Vars(r)
	to, cam := vars["to"], vars["camera"]
	vals := map[string]string{
		"audio":  r.FormValue("audio"),
		"height": r.FormValue("height"),
		"width":  r.FormValue("width"),
		"crf":    r.FormValue("crf"),
		"time":   r.FormValue("time"),
		"rate":   r.FormValue("rate"),
		"size":   r.FormValue("size"),
		"level":  r.FormValue("level"),
		"prof":   r.FormValue("profile"),
	}
	id, code, reply := ReqID(4), 200, "OK"
	c.Lock()
	_, ok := c.Cameras[cam]
	c.Unlock()
	if !ok {
		Debugf(id, "Invalid 'cam' provided: %v", cam)
		code, reply = 500, "ERROR: Camera not found in configuration!"
	}
	for _, t := range strings.Split(to, ",") {
		if t == "" || !contains(c.AllowedTo, t) {
			Debugf(id, "Invalid 'to' provided: %v", t)
			code, reply = 500, "ERROR: Missing 'to' or 'cam'"
		}
	}
	if code == 200 {
		go c.processVideoRequest(id, cam, to, vals)
	}
	reply = "REQ ID: " + id + ", msg: " + reply + "\n"
	c.finishReq(w, r, id, code, reply, messages.Msg{}, "-")
}

// Since this runs in a go routine it sort of defeats the purpose of the queue. sorta?
func (c *Config) processVideoRequest(id, cam, to string, v map[string]string) {
	path := c.TempDir + "imessage_relay_" + id + "_" + cam + ".mov"
	camData := c.Cameras[cam]
	urlData := &url.Values{}
	var builtURL string
	c.Lock()
	defer c.Unlock()
	if camData.Copy {
		if strings.HasSuffix(camData.URL, "=") {
			camData.URL += url.PathEscape(camData.Number)
		}
		if v["height"] != "" {
			urlData.Set("height", v["height"])
		} else if camData.Height > 99 {
			urlData.Set("height", strconv.Itoa(camData.Height))
		}
		if v["width"] != "" {
			urlData.Set("width", v["width"])
		} else if camData.Width > 99 {
			urlData.Set("width", strconv.Itoa(camData.Width))
		}
		if v["rate"] != "" {
			urlData.Set("req_fps", v["rate"])
		}
		if v["crf"] != "" {
			urlData.Set("quality", v["crf"])
		}
	}
	encoder := encode.Get(&encode.VidOps{
		Level: v["level"],
		Prof:  v["profile"],
		Copy:  camData.Copy,
		Audio: camData.Audio,
	})
	if strings.Contains(camData.URL, "&") {
		builtURL = camData.URL + "&" + urlData.Encode()
	} else {
		builtURL = camData.URL + "?" + urlData.Encode()
	}
	encoder.SetAudio(v["audio"])
	encoder.SetHeight(v["height"])
	encoder.SetWidth(v["width"])
	encoder.SetCRF(v["crf"])
	encoder.SetTime(v["time"])
	encoder.SetRate(v["rate"])
	encoder.SetSize(v["size"])
	cmd, out, err := encoder.GetVideo(id, builtURL, path, cam)
	// This will probably put passwords in logs :(
	Debugf(id, "FFMPEG Command: %v", cmd)
	if err != nil {
		log.Printf("[ERROR] [%v] GetVideo: %v %v", id, err, out)
		return
	}
	// Input data OK, video grabbed, send an attachment to each recipient.
	for _, t := range strings.Split(to, ",") {
		c.msgs.Send(
			messages.Msg{ID: id, To: t, Text: path, File: true, Call: c.videoCallback})
	}
}

// videoCallback runs in a go routine after the video iMessage is processed.
func (c *Config) videoCallback(id, to, path string, err error) {
	var size int64
	if fi, errStat := os.Stat(path); errStat == nil {
		size = fi.Size()
	}
	if err != nil {
		c.export.errors.Add(1)
		log.Printf("[ERROR] [%v] msgs.Send '%v': %v", id, to, err)
	} else {
		c.export.videos.Add(1)
		log.Printf("[REPLY] [%v] Video '%v' (%.2fMb) sent to: %v", id, path, float32(size)/1024/1024, to)
	}
	// Might take a while to upload.
	time.Sleep(20 * time.Minute)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		log.Printf("[ERROR] [%v] Remove(path): %v", id, err)
		return
	}
	Debugf(id, "Deleted: %v", path)
}

// /api/v1.0/send/imessage/picture/{to}/{camera}
func (c *Config) sendPictureHandler(w http.ResponseWriter, r *http.Request) {
	c.export.httpVisits.Add(1)
	vars := mux.Vars(r)
	to, cam := strings.Split(vars["to"], ","), vars["camera"]
	id, code, reply := ReqID(4), 200, "OK"
	path := c.TempDir + "imessage_relay_" + id + "_" + cam + ".jpg"
	// Check input data.
	for _, t := range to {
		if t == "" || !contains(c.AllowedTo, t) {
			Debugf(id, "Invalid 'to' provided: %v", t)
			code = 500
			break
		}
	}
	if cam == "" || code == 500 {
		code, reply = 500, "ERROR: Missing 'to' or 'cam'"
		Debugf(id, "Invalid 'to' provided or 'cam' empty: %v", cam)
	} else if err := c.GetPicture(id, cam, path); err != nil {
		log.Printf("[ERROR] [%v] GetPicture: %v", id, err)
		code, reply = 500, "ERROR: "+err.Error()
	} else {
		// Give the file system time to sync
		time.Sleep(150 * time.Millisecond)
		// Input data OK, send a message to each recipient.
		for _, t := range to {
			c.msgs.Send(messages.Msg{ID: id, To: t, Text: path, File: true, Call: c.pictureCallback})
		}
		reply = "REQ ID: " + id + ", msg: " + reply + "\n"
	}
	c.finishReq(w, r, id, code, reply, messages.Msg{}, "-")
}

// This runs in a go routine after the iMessage is processed.
// Possibly more than once...
func (c *Config) pictureCallback(id, to, path string, err error) {
	if err != nil {
		c.export.errors.Add(1)
		log.Printf("[ERROR] [%v] msgs.Send '%v': %v", id, to, err)

	} else {
		c.export.pics.Add(1)
		log.Printf("[REPLY] [%v] Picture '%v' sent to: %v", id, path, to)
	}
	// Might take a while to upload.
	time.Sleep(5 * time.Second)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		log.Printf("[ERROR] [%v] Remove(path): %v", id, err)
	} else if err == nil {
		Debugf(id, "Deleted: %v", path)
	}
}

// /api/v1.0/send/imessage/msg
func (c *Config) sendMessageHandler(w http.ResponseWriter, r *http.Request) {
	c.export.httpVisits.Add(1)
	vars := mux.Vars(r)
	to, msg := strings.Split(vars["to"], ","), vars["msg"]
	id, code, reply := ReqID(4), 200, "OK"
	// Check input data.
	for _, t := range to {
		if t == "" || !contains(c.AllowedTo, t) {
			Debugf(id, "Invalid 'to' provided: %v", t)
			code = 500
			break
		}
	}
	callback := func(i, t, m string, err error) {
		if err != nil {
			c.export.errors.Add(1)
			log.Printf("[ERROR] [%v] msgs.Send '%v': %v", id, to, err)
			return
		}
		c.export.texts.Add(1)
		log.Printf("[REPLY] [%v] Message (%d chars) sent to: %v", id, len(m), to)
	}
	if code == 500 || msg == "" {
		Debugf(id, "Invalid 'to' provided or 'msg' empty: %v", msg)
		code, reply = 500, "ERROR: Missing 'to' or 'msg'"
	} else {
		// Input data OK, send a message to each recipient.
		for _, t := range to {
			c.msgs.Send(messages.Msg{ID: id, To: t, Text: msg, File: false, Call: callback})
		}
	}
	reply = "REQ ID: " + id + ", msg: " + reply + "\n"
	c.finishReq(w, r, id, code, reply, messages.Msg{}, "-")
}
