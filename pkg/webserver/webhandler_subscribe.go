package webserver

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/davidnewhall/motifini/pkg/chat"
	"github.com/davidnewhall/motifini/pkg/messenger"
	"github.com/gorilla/mux"
	"golift.io/subscribe"
)

const defaultPauseMinutes = 60

// /api/v1.0/sub/{cmd:subscribe|unsubscribe|pause|unpause}/{api}/{contact}/{event} handler.
// Optional query: minutes (pause only, default 60).
func (c *Config) subsHandler(writer http.ResponseWriter, request *http.Request) {
	reqID := messenger.ReqID(messenger.IDLength)
	vars := mux.Vars(request)
	cmd := strings.ToLower(vars["cmd"])
	api := vars["api"]
	contact := vars["contact"]
	event := vars["event"]

	sub, err := c.lookupSubscriber(api, contact)
	if err != nil {
		c.finishReq(writer, request, reqID, http.StatusNotFound,
			"ERROR: subscriber not found: "+contact+"\n", cmd)

		return
	}

	code, reply := c.applySubCmd(sub, cmd, event, request.FormValue("minutes"))
	if code == http.StatusOK {
		err := c.Subs.StateFileSave()
		if err != nil {
			c.Error.Printf("[%v] saving state after %s: %v", reqID, cmd, err)
			code, reply = http.StatusInternalServerError, "ERROR: save failed: "+err.Error()+"\n"
		}
	}

	c.finishReq(writer, request, reqID, code, reply, cmd)
}

func (c *Config) lookupSubscriber(api, contact string) (*subscribe.Subscriber, error) {
	id, err := strconv.ParseInt(contact, 10, 64)
	if err == nil && id != 0 {
		sub, byIDErr := c.Subs.GetSubscriberByID(id, api)
		if byIDErr == nil {
			return sub, nil
		}
	}

	sub, err := c.Subs.GetSubscriber(contact, api)
	if err != nil {
		return nil, fmt.Errorf("get subscriber %q: %w", contact, err)
	}

	return sub, nil
}

func (c *Config) applySubCmd(sub *subscribe.Subscriber, cmd, event, minutesStr string) (int, string) {
	switch cmd {
	case "subscribe":
		return applySubscribe(sub, event)
	case "unsubscribe":
		return applyUnsubscribe(sub, event)
	case "pause":
		return applyPause(sub, event, minutesStr)
	case "unpause":
		return applyUnpause(sub, event)
	default:
		return http.StatusBadRequest, "ERROR: unknown command\n"
	}
}

func applySubscribe(sub *subscribe.Subscriber, event string) (int, string) {
	err := sub.Subscribe(event)
	if err != nil {
		return http.StatusConflict, "ERROR: " + err.Error() + "\n"
	}

	return http.StatusOK, fmt.Sprintf("OK: subscribed %s to %s\n", subLabel(sub), event)
}

func applyUnsubscribe(sub *subscribe.Subscriber, event string) (int, string) {
	if name := sub.Events.Name(event); name != "" {
		event = name
	}

	sub.Events.Remove(event)

	return http.StatusOK, fmt.Sprintf("OK: unsubscribed %s from %s\n", subLabel(sub), event)
}

func applyPause(sub *subscribe.Subscriber, event, minutesStr string) (int, string) {
	mins := defaultPauseMinutes

	if minutesStr != "" {
		n, err := strconv.Atoi(minutesStr)
		if err != nil {
			return http.StatusBadRequest, "ERROR: invalid minutes\n"
		}

		mins = n
	}

	if mins < 0 || mins > chat.MaxPauseMinutes {
		return http.StatusBadRequest,
			fmt.Sprintf("ERROR: minutes must be 0–%d (24 hours)\n", chat.MaxPauseMinutes)
	}

	if name := sub.Events.Name(event); name != "" {
		event = name
	}

	err := sub.Events.Pause(event, time.Duration(mins)*time.Minute)
	if err != nil {
		return http.StatusNotFound, "ERROR: " + err.Error() + "\n"
	}

	return http.StatusOK, fmt.Sprintf("OK: paused %s for %s (%d minutes)\n", event, subLabel(sub), mins)
}

func applyUnpause(sub *subscribe.Subscriber, event string) (int, string) {
	if name := sub.Events.Name(event); name != "" {
		event = name
	}

	err := sub.Events.UnPause(event)
	if err != nil {
		return http.StatusNotFound, "ERROR: " + err.Error() + "\n"
	}

	return http.StatusOK, fmt.Sprintf("OK: unpaused %s for %s\n", event, subLabel(sub))
}

func subLabel(sub *subscribe.Subscriber) string {
	if sub == nil {
		return "?"
	}

	if strings.TrimSpace(sub.Contact) != "" {
		return sub.Contact
	}

	return strconv.FormatInt(sub.ID, 10)
}
