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
	log.Printf("Listening on port %d", m.Config.Global.Port)
	r := mux.NewRouter()
	r.Handle("/debug/vars", http.DefaultServeMux).Methods("GET")
	r.HandleFunc("/api/v1.0/send/imessage/video/{to}/{camera}", m.sendVideoHandler).Methods("GET")
	r.HandleFunc("/api/v1.0/send/imessage/picture/{to}/{camera}", m.sendPictureHandler).Methods("GET")
	r.HandleFunc("/api/v1.0/send/imessage/msg/{to}", m.sendMessageHandler).Methods("GET").Queries("msg", "{msg}")
	r.HandleFunc("/api/v1.0/event/{cmd:remove|update|add|notify}/{event}", m.eventsHandler).Methods("POST")
	// need to figure out what user interface will use these methods.
	r.HandleFunc("/api/v1.0/sub/{cmd:subscribe|unsubscribe|pause|unpause}/{api}/{contact}/{event}", m.subsHandler).Methods("GET")
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
