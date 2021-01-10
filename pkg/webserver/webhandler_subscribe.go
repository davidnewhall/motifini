package webserver

import (
	"net/http"

	"github.com/davidnewhall/motifini/pkg/messenger"
)

// /api/v1.0/sub/{cmd:subscribe|unsubscribe|pause|unpause}/{api}/{contact}/{event} handler.
func (c *Config) subsHandler(w http.ResponseWriter, r *http.Request) {
	id, code, reply := messenger.ReqID(messenger.IDLength), http.StatusInternalServerError, "ERROR\n"
	from, msg := r.FormValue("from"), r.FormValue("msg")

	if from != "" && msg != "" {
		code, reply = http.StatusOK, "woo!\n"
	}

	c.finishReq(w, r, id, code, reply, "-")
}

// unfinished.
