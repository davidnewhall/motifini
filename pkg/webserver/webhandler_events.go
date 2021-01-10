package webserver

import (
	"net/http"
	"strings"

	"github.com/davidnewhall/motifini/pkg/messenger"
	"github.com/gorilla/mux"
	"golift.io/securityspy"
)

// /api/v1.0/event/{cmd:remove|update|add|notify}/{event} handler.
func (c *Config) eventsHandler(w http.ResponseWriter, r *http.Request) {
	id, vars := messenger.ReqID(messenger.IDLength), mux.Vars(r)

	switch cmd := strings.ToLower(vars["cmd"]); cmd {
	case "remove":
		//
	case "update":
		//
	case "add":
		//
	case "notify":
		c.notifyHandler(id, vars, w, r)
	}
}

func (c *Config) notifyHandler(id string, vars map[string]string, w http.ResponseWriter, r *http.Request) {
	code, reply := http.StatusOK, "REQ ID: "+id+", msg: got notify\n"
	cam := c.SSpy.Cameras.ByName(vars["event"])
	subs := c.Subs.GetSubscribers(vars["event"])
	path := ""

	if cam != nil && len(subs) > 0 {
		path = c.TempDir + "imessage_relay_" + id + "_" + vars["event"] + ".jpg"

		if err := cam.SaveJPEG(&securityspy.VidOps{}, path); err != nil {
			c.Error.Printf("[%v] cam.SaveJPEG: %v", id, err)
			code, reply = http.StatusInternalServerError, "ERROR: "+err.Error()
		}
	}

	msg := r.FormValue("msg")
	c.Msgs.SendFileOrMsg(id, msg, path, subs)
	c.finishReq(w, r, id, code, reply, msg)
}
