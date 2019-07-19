package cli

import (
	"log"
	"net/http"
	"strings"

	"code.golift.io/imessage"
	"github.com/gorilla/mux"
)

// /api/v1.0/event/{cmd:remove|update|add|notify}/{event}
func (m *Motifini) eventsHandler(w http.ResponseWriter, r *http.Request) {
	m.exports.httpVisits.Add(1)
	vars := mux.Vars(r)
	id, code, reply := ReqID(4), 500, "3RROR\n"
	msg := ""
	switch cmd := strings.ToLower(vars["cmd"]); cmd {
	case "remove":
		//
	case "update":
		//
	case "add":
		//
	case "notify":
		code, reply = 200, "REQ ID: "+id+", msg: got notify\n"
		_, isCam := m.Cameras[vars["event"]]
		subs := m.GetSubscribers(vars["event"])
		path := m.TempDir + "imessage_relay_" + id + "_" + vars["event"] + ".jpg"
		if isCam && len(subs) > 0 {
			if err := m.GetPicture(id, vars["event"], path); err != nil {
				log.Printf("[ERROR] [%v] GetPicture: %v", id, err)
				code, reply = 500, "ERROR: "+err[0].Error()
			}
		}
		msg = r.FormValue("msg")
		if msg == "" {
			if msg = m.GetEvents()[vars["event"]]["description"]; msg == "" {
				msg = vars["event"]
			}
		}
		for _, sub := range subs {
			switch sub.API {
			case APIiMessage:
				if isCam {
					m.Send(imessage.Outgoing{ID: id, To: sub.Contact, Text: path, File: true, Call: m.pictureCallback})
				} else {
					m.Send(imessage.Outgoing{ID: id, To: sub.Contact, Text: msg})
				}
			default:
				log.Printf("[%v] Unknown Notification API '%v' for contact: %v", id, sub.API, sub.Contact)
			}
		}
	}
	m.finishReq(w, r, id, code, reply, imessage.Outgoing{}, msg)
}
