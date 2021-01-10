// Package webserver provides all the methods for our web server. There aren't
// very many things going on here yet, and it will likely get split into more
// packages as it gets built out. Right now, it basically has an event handler
// that allows external home automation tools to trigger event notifications.
// It also provides a nice API to send iMessages. You can use this to program in
// your home automation apps to "curl" an interface to send a msg to an iPhone, etc.
// We also have hooks for SecuritySpy, to capture and send videos or pictures to
// messenger users. This code needs a re-think now that we have an event stream.
package webserver

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/davidnewhall/motifini/pkg/export"
	"github.com/davidnewhall/motifini/pkg/messenger"
	"github.com/gorilla/mux"
	"golift.io/securityspy"
	"golift.io/subscribe"
)

const (
	DefaultListenPort = 8765
	Timeout           = 30 * time.Second
)

type Config struct {
	http      *http.Server
	SSpy      *securityspy.Server
	Subs      *subscribe.Subscribe
	Msgs      *messenger.Messenger
	Info      *log.Logger
	Debug     *log.Logger
	Error     *log.Logger
	TempDir   string
	AllowedTo []string
	Port      uint
}

// Start validates the config and returns any errors.
// If all goes well, this will not return until the server shuts down.
func Start(s *Config) error {
	if s.SSpy == nil {
		return fmt.Errorf("%w: securityspy is nil", messenger.ErrNillConfigItem)
	}

	if s.Subs == nil {
		return fmt.Errorf("%w: subscribe is nil", messenger.ErrNillConfigItem)
	}

	if s.Msgs == nil {
		return fmt.Errorf("%w: messenger is nil", messenger.ErrNillConfigItem)
	}

	if s.Info == nil {
		s.Info = log.New(ioutil.Discard, "", 0)
	}

	if s.Debug == nil {
		s.Debug = log.New(ioutil.Discard, "", 0)
	}

	if s.Error == nil {
		s.Error = log.New(ioutil.Discard, "", 0)
	}

	if s.TempDir == "" {
		s.TempDir = "/tmp/"
	}

	if s.Port == 0 {
		s.Port = DefaultListenPort
	}

	return s.StartWebServer()
}

// StartWebServer creates the http routers and starts http server
// This code block shows all the routes, for now.
func (c *Config) StartWebServer() error {
	r := mux.NewRouter()
	r.Handle("/debug/vars", http.DefaultServeMux).Methods("GET")
	r.HandleFunc("/api/v1.0/send/imessage/video/{to}/{camera}", c.sendVideoHandler).Methods("GET")
	r.HandleFunc("/api/v1.0/send/imessage/picture/{to}/{camera}", c.sendPictureHandler).Methods("GET")
	r.HandleFunc("/api/v1.0/send/imessage/msg/{to}", c.sendMessageHandler).Methods("GET").Queries("msg", "{msg}")
	r.HandleFunc("/api/v1.0/event/{cmd:remove|update|add|notify}/{event}", c.eventsHandler).Methods("POST")
	// need to figure out what user interface will use these methods.
	r.HandleFunc("/api/v1.0/sub/{cmd:subscribe|unsubscribe|pause|unpause}/{api}/{contact}/{event}",
		c.subsHandler).Methods("GET")
	r.PathPrefix("/").HandlerFunc(c.handleAll)
	http.Handle("/", r)

	c.http = &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", c.Port),
		WriteTimeout: Timeout,
		ReadTimeout:  Timeout,
		IdleTimeout:  time.Minute,
		Handler:      r, // *mux.Router
	}
	c.Info.Print("Web server listening at http://", c.http.Addr)

	return c.http.ListenAndServe()
}

// Stop shuts down the HTTP listener.
func (c *Config) Stop() {
	// Give the http server up to 3 seconds to finish any open requests.
	if c.http == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()

	if err := c.http.Shutdown(ctx); err != nil {
		c.Error.Println("Shutting down web server:", err)
	}
}

func (c *Config) finishReq(w http.ResponseWriter, r *http.Request, id string, code int, reply string, cmd string) {
	export.Map.HTTPVisits.Add(1)
	c.Info.Printf(`[%v] %v %v "%v %v" %d %d "%v" "%v"`,
		id, r.RemoteAddr, r.Host, r.Method, r.URL.String(), code, len(reply), r.UserAgent(), cmd)
	w.WriteHeader(code)

	if _, err := w.Write([]byte(reply)); err != nil {
		c.Error.Printf("[%v] Error Sending Reply: %v", id, err)
	}
}

// handle any unknown URIs.
func (c *Config) handleAll(w http.ResponseWriter, r *http.Request) {
	export.Map.HTTPVisits.Add(1)
	export.Map.DefaultURL.Add(1)

	id, code, reply := messenger.ReqID(messenger.IDLength), http.StatusMethodNotAllowed, "FAIL\n"
	c.finishReq(w, r, id, code, reply, "-")
}

// check for a thing in a thing.
func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}

	return false
}
