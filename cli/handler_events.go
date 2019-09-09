package cli

import (
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"golift.io/imessage"
	"golift.io/securityspy"
)

// /api/v1.0/event/{cmd:remove|update|add|notify}/{event}
func (m *Motifini) eventsHandler(w http.ResponseWriter, r *http.Request) {
	m.exports.httpVisits.Add(1)
	id, vars := ReqID(4), mux.Vars(r)
	switch cmd := strings.ToLower(vars["cmd"]); cmd {
	case "remove":
		//
	case "update":
		//
	case "add":
		//
	case "notify":
		m.notifyHandler(id, vars, w, r)
	}
}

func (m *Motifini) notifyHandler(id string, vars map[string]string, w http.ResponseWriter, r *http.Request) {
	code, reply := 200, "REQ ID: "+id+", msg: got notify\n"
	cam := m.Spy.Cameras.ByName(vars["event"])
	subs := m.Subs.GetSubscribers(vars["event"])
	path := m.Config.Global.TempDir + "imessage_relay_" + id + "_" + vars["event"] + ".jpg"
	if cam != nil && len(subs) > 0 {
		if err := cam.SaveJPEG(&securityspy.VidOps{}, path); err != nil {
			log.Printf("[ERROR] [%v] cam.SaveJPEG: %v", id, err)
			code, reply = 500, "ERROR: "+err.Error()
		}
	}
	msg := r.FormValue("msg")
	if msg == "" {
		if msg = m.Subs.GetEvents()[vars["event"]]["description"]; msg == "" {
			msg = vars["event"]
		}
	}
	for _, sub := range subs {
		switch sub.API {
		case APIiMessage:
			if cam != nil {
				m.Msgs.Send(imessage.Outgoing{ID: id, To: sub.Contact, Text: path, File: true, Call: m.pictureCallback})
			} else {
				m.Msgs.Send(imessage.Outgoing{ID: id, To: sub.Contact, Text: msg})
			}
		default:
			log.Printf("[%v] Unknown Notification API '%v' for contact: %v", id, sub.API, sub.Contact)
		}
	}
	m.finishReq(w, r, id, code, reply, msg)
}
