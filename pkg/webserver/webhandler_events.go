package webserver

import (
	"net/http"
	"strings"

	"github.com/davidnewhall/motifini/pkg/messenger"
	"github.com/gorilla/mux"
	"golift.io/securityspy/v2"
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
	if msg == "" && cam != nil {
		msg = cam.Name
	}
	c.Msgs.SendFileOrMsg(reqID, msg, path, subs)
	c.finishReq(writer, request, reqID, code, reply, msg)
}
