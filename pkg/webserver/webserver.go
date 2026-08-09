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
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net"
	"net/http"
	"slices"
	"strconv"
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
	DefaultListenAddr = "127.0.0.1"
	Timeout           = 30 * time.Second
)

// ErrAPIKeyRequired is returned when the web server is configured to listen on
// a non-localhost address without an api_key.
var ErrAPIKeyRequired = errors.New("api_key is required when listen_addr is not localhost")

// Config holds HTTP server dependencies and listen settings.
type Config struct {
	http             *http.Server
	SSpy             *securityspy.Server
	Subs             *subscribe.Subscribe
	Msgs             *messenger.Messenger
	Info             *log.Logger
	Debug            *log.Logger
	Error            *log.Logger
	TempDir          string
	AllowedTo        []string
	ListenAddr       string
	APIKey           string
	Port             uint
	AllowSubscribers bool
}

// Start validates the config and returns any errors.
// If all goes well, this will not return until the server shuts down.
func Start(cfg *Config) error {
	// SSpy may be nil when [security_spy] is missing; camera routes return 503 via securitySpyReady.

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

	if cfg.ListenAddr == "" {
		cfg.ListenAddr = DefaultListenAddr
	}

	if cfg.Port == 0 {
		cfg.Port = DefaultListenPort
	}

	if cfg.APIKey == "" && !isLoopbackAddr(cfg.ListenAddr) {
		return fmt.Errorf("%w (listen_addr=%s)", ErrAPIKeyRequired, cfg.ListenAddr)
	}

	cfg.Start()

	return nil
}

// handler builds the routed handler stack. When an API key is configured,
// every route (including /debug/vars) requires it.
func (c *Config) handler() http.Handler {
	router := mux.NewRouter()
	router.Handle("/debug/vars", http.DefaultServeMux).Methods("GET")
	router.HandleFunc("/api/v1.0/send/{app:telegram}/video/{to}/{camera}", c.sendVideoHandler).Methods("GET")
	router.HandleFunc("/api/v1.0/send/{app:telegram}/picture/{to}/{camera}", c.sendPictureHandler).Methods("GET")
	router.HandleFunc("/api/v1.0/send/{app:telegram}/msg/{to}", c.sendMessageHandler).
		Methods("GET").Queries("msg", "{msg}")
	router.HandleFunc("/api/v1.0/events", c.eventsListHandler).Methods("GET")
	router.HandleFunc("/api/v1.0/event/{event}", c.eventUpsertHandler).Methods("PUT")
	router.HandleFunc("/api/v1.0/event/{cmd:remove|notify}/{event}", c.eventsHandler).Methods("POST")
	router.HandleFunc("/api/v1.0/sub/{cmd:subscribe|unsubscribe|pause|unpause}/{api}/{contact}/{event}",
		c.subsHandler).Methods("GET")
	router.PathPrefix("/").HandlerFunc(c.handleAll)

	return c.requireAPIKey(router)
}

// Start creates the http routers and starts http server
// This code block shows all the routes, for now.
func (c *Config) Start() {
	c.http = &http.Server{
		Addr:         net.JoinHostPort(c.ListenAddr, strconv.Itoa(int(c.Port))),
		WriteTimeout: Timeout,
		ReadTimeout:  Timeout,
		IdleTimeout:  time.Minute,
		Handler:      c.handler(),
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
		reqID, request.RemoteAddr, request.Host, request.Method, redactAPIKey(request),
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

// finishReqJSON is finishReq for JSON payloads: no HTML escaping, JSON content type.
func (c *Config) finishReqJSON(
	writer http.ResponseWriter, request *http.Request, reqID string, code int, reply []byte, cmd string,
) {
	export.Map.HTTPVisits.Add(1)
	c.Info.Printf(`[%v] %v %v "%v %v" %d %d "%v" "%v"`,
		reqID, request.RemoteAddr, request.Host, request.Method, redactAPIKey(request),
		code, len(reply), request.UserAgent(), cmd)
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(code)

	_, err := writer.Write(reply)
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

// recipientAllowed reports whether a Telegram chat id may be a "to" recipient:
// either listed in allowed_to, or — when allow_subscribers is set — any
// authenticated, non-ignored Telegram subscriber in the state DB.
func (c *Config) recipientAllowed(idStr string) bool {
	if contains(c.AllowedTo, idStr) {
		return true
	}

	if !c.AllowSubscribers || c.Subs == nil {
		return false
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return false
	}

	sub, err := c.Subs.GetSubscriberByID(id, messenger.APITelegram)
	if err != nil || sub == nil {
		return false
	}

	// Same auth marker the Telegram message path checks (set by /id or admin Allow).
	authed, _ := sub.Meta["hasAuth"].(bool)

	return authed && !sub.Ignored
}

// securitySpyReady is false until the first successful Refresh() loads cameras.
func (c *Config) securitySpyReady() bool {
	return c != nil && c.SSpy != nil && c.SSpy.Cameras != nil
}

// cameraByName looks up a camera, or nil when SecuritySpy has no camera list yet.
func (c *Config) cameraByName(name string) *securityspy.Camera {
	if !c.securitySpyReady() {
		return nil
	}

	return c.SSpy.Cameras.ByName(name)
}

// cameraByNameOrNum looks up a camera by name, falling back to the SecuritySpy
// camera number when the value parses as an integer.
func (c *Config) cameraByNameOrNum(nameOrNum string) *securityspy.Camera {
	if !c.securitySpyReady() {
		return nil
	}

	if cam := c.SSpy.Cameras.ByName(nameOrNum); cam != nil {
		return cam
	}

	num, err := strconv.Atoi(nameOrNum)
	if err != nil {
		return nil
	}

	return c.SSpy.Cameras.ByNum(num)
}
