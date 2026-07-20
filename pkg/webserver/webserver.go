// Package webserver provides all the methods for our web server. There aren't
// very many things going on here yet, and it will likely get split into more
// packages as it gets built out. Right now, it basically has an event handler
// that allows external home automation tools to trigger event notifications.
// It also provides a nice API to send Telegram messages. You can use this to program in
// your home automation apps to "curl" an interface to send a msg to a Telegram user.
// We also have hooks for SecuritySpy, to capture and send videos or pictures to
// Telegram users. This code needs a re-think now that we have an event stream.
package webserver

import (
	"context"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"slices"
	"time"

	"github.com/davidnewhall/motifini/pkg/export"
	"github.com/davidnewhall/motifini/pkg/messenger"
	"github.com/gorilla/mux"
	"golift.io/securityspy/v2"
	"golift.io/subscribe"
)

// HTTP server defaults.
const (
	DefaultListenPort = 8765
	Timeout           = 30 * time.Second
)

// Config holds HTTP server dependencies and listen settings.
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
func Start(cfg *Config) error {
	if cfg.SSpy == nil {
		return fmt.Errorf("%w: securityspy is nil", messenger.ErrNillConfigItem)
	}

	if cfg.Subs == nil {
		return fmt.Errorf("%w: subscribe is nil", messenger.ErrNillConfigItem)
	}

	if cfg.Msgs == nil {
		return fmt.Errorf("%w: messenger is nil", messenger.ErrNillConfigItem)
	}

	if cfg.Info == nil {
		cfg.Info = log.New(io.Discard, "", 0)
	}

	if cfg.Debug == nil {
		cfg.Debug = log.New(io.Discard, "", 0)
	}

	if cfg.Error == nil {
		cfg.Error = log.New(io.Discard, "", 0)
	}

	if cfg.TempDir == "" {
		cfg.TempDir = "/tmp/"
	}

	if cfg.Port == 0 {
		cfg.Port = DefaultListenPort
	}

	cfg.Start()

	return nil
}

// Start creates the http routers and starts http server
// This code block shows all the routes, for now.
func (c *Config) Start() {
	router := mux.NewRouter()
	router.Handle("/debug/vars", http.DefaultServeMux).Methods("GET")
	router.HandleFunc("/api/v1.0/send/{app:telegram}/video/{to}/{camera}", c.sendVideoHandler).Methods("GET")
	router.HandleFunc("/api/v1.0/send/{app:telegram}/picture/{to}/{camera}", c.sendPictureHandler).Methods("GET")
	router.HandleFunc("/api/v1.0/send/{app:telegram}/msg/{to}", c.sendMessageHandler).
		Methods("GET").Queries("msg", "{msg}")
	router.HandleFunc("/api/v1.0/event/{cmd:remove|update|add|notify}/{event}", c.eventsHandler).Methods("POST")
	// need to figure out what user interface will use these methods.
	router.HandleFunc("/api/v1.0/sub/{cmd:subscribe|unsubscribe|pause|unpause}/{api}/{contact}/{event}",
		c.subsHandler).Methods("GET")
	router.PathPrefix("/").HandlerFunc(c.handleAll)

	c.http = &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", c.Port),
		WriteTimeout: Timeout,
		ReadTimeout:  Timeout,
		IdleTimeout:  time.Minute,
		Handler:      router, // *mux.Router
	}

	c.Info.Print("Web server listening at http://", c.http.Addr)

	go func() {
		err := c.http.ListenAndServe()
		if err != nil {
			c.Error.Println("Web Server Stopped:", err)
		}
	}()
}

// Stop shuts down the HTTP listener.
func (c *Config) Stop() error {
	// Give the http server up to 3 seconds to finish any open requests.
	if c.http == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()

	err := c.http.Shutdown(ctx)
	if err != nil {
		return fmt.Errorf("shutting down web server: %w", err)
	}

	return nil
}

func (c *Config) finishReq(
	writer http.ResponseWriter, request *http.Request, reqID string, code int, reply, cmd string,
) {
	export.Map.HTTPVisits.Add(1)
	c.Info.Printf(`[%v] %v %v "%v %v" %d %d "%v" "%v"`,
		reqID, request.RemoteAddr, request.Host, request.Method, request.URL.String(),
		code, len(reply), request.UserAgent(), cmd)
	// Force plain-text rendering and escape the body so a browser can never
	// interpret the reply (which may echo back request input) as HTML/script.
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	writer.WriteHeader(code)

	_, err := writer.Write([]byte(html.EscapeString(reply)))
	if err != nil {
		c.Error.Printf("[%v] Error Sending Reply: %v", reqID, err)
	}
}

// handle any unknown URIs.
func (c *Config) handleAll(writer http.ResponseWriter, request *http.Request) {
	export.Map.HTTPVisits.Add(1)
	export.Map.DefaultURL.Add(1)

	reqID, code, reply := messenger.ReqID(messenger.IDLength), http.StatusMethodNotAllowed, "FAIL\n"
	c.finishReq(writer, request, reqID, code, reply, "-")
}

// check for a thing in a thing.
func contains(s []string, e string) bool {
	return slices.Contains(s, e)
}
