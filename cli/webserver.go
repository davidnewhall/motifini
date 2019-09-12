package cli

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// StartServer creates the http routers and starts http server
func (m *Motifini) StartServer() error {
	log.Printf("Webserver listening on port %d", m.Config.Global.Port)
	r := mux.NewRouter()
	r.Handle("/debug/vars", http.DefaultServeMux).Methods("GET")
	r.HandleFunc("/api/v1.0/send/imessage/video/{to}/{camera}", m.sendVideoHandler).Methods("GET")
	r.HandleFunc("/api/v1.0/send/imessage/picture/{to}/{camera}", m.sendPictureHandler).Methods("GET")
	r.HandleFunc("/api/v1.0/send/imessage/msg/{to}", m.sendMessageHandler).Methods("GET").Queries("msg", "{msg}")
	r.HandleFunc("/api/v1.0/event/{cmd:remove|update|add|notify}/{event}", m.eventsHandler).Methods("POST")
	// need to figure out what user interface will use these methods.
	r.HandleFunc("/api/v1.0/sub/{cmd:subscribe|unsubscribe|pause|unpause}/{api}/{contact}/{event}",
		m.subsHandler).Methods("GET")
	r.PathPrefix("/").HandlerFunc(m.handleAll)
	http.Handle("/", r)
	m.Server = &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", m.Config.Global.Port),
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      r, // *mux.Router
	}
	return m.Server.ListenAndServe()
}

func (m *Motifini) finishReq(w http.ResponseWriter, r *http.Request, id string, code int, reply string, cmd string) {
	log.Printf(`[REQST] [%v] %v %v "%v %v" %d %d "%v" "%v"`,
		id, r.RemoteAddr, r.Host, r.Method, r.URL.String(), code, len(reply), r.UserAgent(), cmd)
	w.WriteHeader(code)
	if _, err := w.Write([]byte(reply)); err != nil {
		log.Printf("[ERROR] [%v] Error Sending Reply: %v", id, err)
	}
}

// handle any unknown URIs.
func (m *Motifini) handleAll(w http.ResponseWriter, r *http.Request) {
	m.exports.httpVisits.Add(1)
	m.exports.defaultURL.Add(1)
	id, code, reply := ReqID(4), 405, "FAIL\n"
	m.finishReq(w, r, id, code, reply, "-")
}
