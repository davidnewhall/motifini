package webserver

import (
	"net/http"
	"strings"

	"github.com/davidnewhall/motifini/pkg/messenger"
	"github.com/gorilla/mux"
	"golift.io/securityspy/v2"
)

// /api/v1.0/event/{cmd:remove|update|add|notify}/{event} handler.
func (c *Config) eventsHandler(writer http.ResponseWriter, request *http.Request) {
	reqID, vars := messenger.ReqID(messenger.IDLength), mux.Vars(request)

	switch cmd := strings.ToLower(vars["cmd"]); cmd {
	case "remove":
		//
	case "update":
		//
	case "add":
		//
	case "notify":
		c.notifyHandler(reqID, vars, writer, request)
	}
}

func (c *Config) notifyHandler(
	reqID string, vars map[string]string, writer http.ResponseWriter, request *http.Request,
) {
	code, reply := http.StatusOK, "REQ ID: "+reqID+", msg: got notify\n"
	cam := c.SSpy.Cameras.ByName(vars["event"])
	subs := c.Subs.GetSubscribers(vars["event"])
	path := ""

	if cam != nil && len(subs) > 0 {
		path = c.TempDir + "motifini_relay_" + reqID + "_" + vars["event"] + ".jpg"

		err := cam.SaveJPEG(&securityspy.VidOps{}, path)
		if err != nil {
			c.Error.Printf("[%v] cam.SaveJPEG: %v", reqID, err)
			code, reply = http.StatusInternalServerError, "ERROR: "+err.Error()
		}
	}

	msg := request.FormValue("msg")
	c.Msgs.SendFileOrMsg(reqID, msg, path, subs)
	c.finishReq(writer, request, reqID, code, reply, msg)
}
