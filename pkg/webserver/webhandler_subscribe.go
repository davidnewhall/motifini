package webserver

import (
	"net/http"

	"github.com/davidnewhall/motifini/pkg/messenger"
)

// TODO: add more to this. allow pulling subscriber list, admins, etc.

// /api/v1.0/sub/{cmd:subscribe|unsubscribe|pause|unpause}/{api}/{contact}/{event} handler.
func (c *Config) subsHandler(writer http.ResponseWriter, request *http.Request) {
	reqID, code, reply := messenger.ReqID(messenger.IDLength), http.StatusInternalServerError, "ERROR\n"
	from, msg := request.FormValue("from"), request.FormValue("msg")

	if from != "" && msg != "" {
		code, reply = http.StatusOK, "woo!\n"
	}

	c.finishReq(writer, request, reqID, code, reply, "-")
}

// unfinished.
