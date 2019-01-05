package main

import (
	"net/http"

	"github.com/davidnewhall/motifini/messages"
)

// /api/v1.0/sub/{cmd:subscribe|unsubscribe|pause|unpause}/{api}/{contact}/{event}
func (c *Config) subsHandler(w http.ResponseWriter, r *http.Request) {
	c.export.httpVisits.Add(1)
	id, code, reply := ReqID(4), 500, "ERROR\n"
	from, msg := r.FormValue("from"), r.FormValue("msg")
	if from != "" && msg != "" {
		code, reply = 200, "woo!\n"
	}
	c.finishReq(w, r, id, code, reply, messages.Msg{}, "-")
}
