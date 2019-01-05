package main

import (
	"context"
	"expvar"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/naoina/toml"
	"github.com/pkg/errors"

	"github.com/davidnewhall/motifini/exp"
	"github.com/davidnewhall/motifini/messages"
	"github.com/davidnewhall/motifini/subscribe"
)

var (
	// Version of the aplication.
	Version = "0.1.1"
	// StopChan is how we exit. Can be used in tests.
	StopChan = make(chan os.Signal, 1)
)

const (
	// APIiMessage is just an identifer for an imessage contact type.
	APIiMessage = "imessage"
)

// Config struct
type Config struct {
	AllowedTo []string
	Cameras   map[string]struct {
		URL    string
		Copy   bool
		Height int
		Width  int
		Audio  bool
		Number string
	}
	ClearMessages  bool
	Port           int
	Queue          int
	SecuritySpyURL string
	TempDir        string
	StateFile      string
	subs           subscribe.SubDB
	msgs           messages.Messages
	export         exportData
	sync.Mutex
}

func main() {
	rand.Seed(time.Now().UnixNano())
	configFile := ParseFlags()
	log.Printf("iMessage Relay %v Starting! (PID: %v)", Version, os.Getpid())
	config, err := GetConfig(configFile)
	if err != nil {
		log.Fatalln("[ERROR] Config Error:", err.Error())
	} else if err := config.Validate(); err != nil {
		log.Fatalln("[ERROR] Validation Error:", err.Error())
	} else if err := config.GetCamNumbers(); err != nil {
		log.Fatalln("[ERROR] SecuritySpy Error:", err.Error())
	} else if config.subs, err = subscribe.GetDB([]string{"all"}, config.StateFile); err != nil {
		log.Fatalln("[ERROR] Subscription State Error:", err.Error())
	}
	signal.Notify(StopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	config.Exports(configFile)
	config.StartUp()
}

// ParseFlags turns CLI args into usable data.
func ParseFlags() string {
	flag.Usage = func() {
		fmt.Println("Usage: motifini [--config=<file>] [--debug] [--version]")
		flag.PrintDefaults()
	}
	config := flag.String("c", "/usr/local/etc/motifini.conf", "Path to config file.")
	flag.BoolVar(&DebugLog, "D", false, "Turn on the Spam (default false).")
	version := flag.Bool("v", false, "Print the version and exit.")
	if flag.Parse(); *version {
		fmt.Println("motifini version:", Version)
		os.Exit(0) // don't run anything else.
	} else if log.SetFlags(log.LstdFlags); DebugLog {
		log.SetFlags(log.Lshortfile | log.Lmicroseconds | log.Ldate)
	}
	return *config
}

// GetConfig parses and returns our configuration data.
func GetConfig(configFile string) (*Config, error) {
	// Preload our defaults.
	config := new(Config)
	Debugf("xxxx", "Reading Config File: %v", configFile)
	if buf, err := ioutil.ReadFile(configFile); err != nil {
		return config, err
	} else if err := toml.Unmarshal(buf, config); err != nil {
		return config, errors.Wrap(err, "invalid config")
	}
	return config, nil
}

// Validate makes sure the data in the config file is valid.
func (c *Config) Validate() error {
	if len(c.AllowedTo) < 1 {
		return errors.New("missing allowed_to list")
	}
	if c.Port == 0 {
		c.Port = 8765
	}
	if c.TempDir == "" {
		c.TempDir = "/tmp/"
	} else if !strings.HasSuffix(c.TempDir, "/") {
		c.TempDir += "/"
	}
	if c.Queue < 20 {
		c.Queue = 20
	} else if c.Queue > 500 {
		c.Queue = 500
	}
	var ignore []string
	c.Lock()
	defer c.Unlock()
	for cam, camData := range c.Cameras {
		if camData.URL == "" || !strings.Contains(camData.URL, "://") {
			log.Printf("[WARN] Ignoring Camera '%v' missing or invalid URL: %v", cam, camData.URL)
			ignore = append(ignore, cam)
		}
	}
	for _, cam := range ignore {
		delete(c.Cameras, cam)
	}
	if len(c.Cameras) == 0 {
		log.Printf("[WARN] No usable cameras defined in configuration!")
	}

	if c.SecuritySpyURL != "" && !strings.Contains(c.SecuritySpyURL, "://") {
		log.Printf("[WARN] Security Spy URL appears malformed. Ignoring it and using AppleScript!")
		c.SecuritySpyURL = ""
	} else if c.SecuritySpyURL != "" && !strings.HasSuffix(c.SecuritySpyURL, "/") {
		c.SecuritySpyURL += "/"
	}
	return nil
}

// Contains our expvar exports.
type exportData struct {
	*expvar.Map
	startAt    expvar.String
	version    expvar.String
	configFile expvar.String
	listenPort expvar.Int
	httpVisits expvar.Int
	defaultURL expvar.Int
	videos     expvar.Int
	pics       expvar.Int
	texts      expvar.Int
	errors     expvar.Int
}

// Exports makes all the expvar data available.
func (c *Config) Exports(configFile string) {
	c.export.Map = exp.GetPublishedMap("iMessageRelay")
	c.export.Set("app_started", &c.export.startAt)
	c.export.Set("app_version", &c.export.version)
	c.export.Set("config_file", &c.export.configFile)
	c.export.Set("listen_port", &c.export.listenPort)
	c.export.Set("http_visits", &c.export.httpVisits)
	c.export.Set("default_url", &c.export.defaultURL)
	c.export.Set("videos_sent", &c.export.videos)
	c.export.Set("photos_sent", &c.export.pics)
	c.export.Set("messge_sent", &c.export.texts)
	c.export.Set("error_count", &c.export.errors)
	// Set static data now.
	c.export.startAt.Set(time.Now().String())
	c.export.version.Set(Version)
	c.export.configFile.Set(configFile)
	c.export.listenPort.Set(int64(c.Port))
}

// StartUp creates the http routers, starts http server and kicks off a task poller.
func (c *Config) StartUp() {
	c.msgs = messages.Init(&messages.Config{QueueSize: c.Queue, ClearMsgs: c.ClearMessages, Debug: DebugLog})
	log.Printf("Listening on port %d", c.Port)
	r := mux.NewRouter()
	r.Handle("/debug/vars", http.DefaultServeMux).Methods("GET")
	r.HandleFunc("/api/v1.0/send/imessage/video/{to}/{camera}", c.sendVideoHandler).Methods("GET")
	r.HandleFunc("/api/v1.0/send/imessage/picture/{to}/{camera}", c.sendPictureHandler).Methods("GET")
	r.HandleFunc("/api/v1.0/send/imessage/msg/{to}", c.sendMessageHandler).Methods("GET").Queries("msg", "{msg}")
	r.HandleFunc("/api/v1.0/recv/imessage/msg/{from}", c.recvMessageHandler).Methods("POST")
	r.HandleFunc("/api/v1.0/event/{cmd:remove|update|add|notify}/{event}", c.eventsHandler).Methods("POST")
	// need to figure out what user interface will use these methods.
	r.HandleFunc("/api/v1.0/sub/{cmd:subscribe|unsubscribe|pause|unpause}/{api}/{contact}/{event}", c.subsHandler).Methods("GET")
	r.PathPrefix("/").HandlerFunc(c.handleAll)
	http.Handle("/", r)

	srv := &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", c.Port),
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      r, // *mux.Router
	}

	go c.taskPoller(srv)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalln("[ERROR] http.ListenAndServe:", err)
	}
}

// taskPoller runs things at an interval and
// looks for an exit signal to shut down the http server.
func (c *Config) taskPoller(srv *http.Server) {
	ticker := time.NewTicker(10 * time.Minute)
	for {
		select {
		case <-ticker.C:
			if err := c.GetCamNumbers(); err != nil {
				log.Printf("[ERROR] Unable to update camera names. Is SecuritySpy running? %v", err)
			}
			if err := c.subs.SaveStateFile(); err != nil {
				log.Printf("[ERROR] Error saving subscribers state file. %v", err)
			}
		case sig := <-StopChan:
			log.Println("Exiting! Caught Signal:", sig)
			if err := c.subs.SaveStateFile(); err != nil {
				log.Printf("[ERROR] Error saving subscribers state file. %v", err)
			}
			// Give the http server up to 3 seconds to finish any open requests.
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = srv.Shutdown(ctx)
			return
		}
	}
}
