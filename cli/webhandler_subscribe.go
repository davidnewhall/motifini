package cli

import (
	"net/http"
)

// /api/v1.0/sub/{cmd:subscribe|unsubscribe|pause|unpause}/{api}/{contact}/{event}
func (m *Motifini) subsHandler(w http.ResponseWriter, r *http.Request) {
	m.exports.httpVisits.Add(1)
	id, code, reply := ReqID(4), 500, "ERROR\n"
	from, msg := r.FormValue("from"), r.FormValue("msg")
	if from != "" && msg != "" {
		code, reply = 200, "woo!\n"
	}
	m.finishReq(w, r, id, code, reply, "-")
}

// unfinished.
