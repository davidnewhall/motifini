package cli

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"expvar"
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

	flag "github.com/spf13/pflag"
	yaml "gopkg.in/yaml.v2"

	"github.com/BurntSushi/toml"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	"code.golift.io/imessage"
	"code.golift.io/subscribe"
)

var (
	// Version of the aplication.
	Version = "development"
	// StopChan is how we exit. Can be used in tests.
	StopChan = make(chan os.Signal, 1)
	// Binary is the app name.
	Binary = "motifini"
)

const (
	// APIiMessage is just an identifer for an imessage contact type.
	APIiMessage = "imessage"
)

// Motifini is the main application struct.
type Motifini struct {
	*Flags
	*Config
	*http.Server
	*subscribe.Subscribe
	*imessage.Messages
	exportData
	flag *flag.FlagSet
}

// Flags defines our application's CLI arguments.
type Flags struct {
	Debug      bool
	VersionReq bool
	ConfigFile string
}

// Config struct
type Config struct {
	Cameras map[string]struct {
		URL    string `toml:"url"`
		Copy   bool   `toml:"copy"`
		Height int    `toml:"height"`
		Width  int    `toml:"width"`
		Audio  bool   `toml:"audio"`
		Number string `toml:"number"`
	} `toml:"cameras"`
	Imessage struct {
		AllowedTo     []string `toml:"allowed_to"`
		DBPath        string   `toml:"db_path"`
		QueueSize     int      `toml:"queue_size"`
		ClearMessages bool     `toml:"clear_messages"`
	} `toml:"imessage"`
	SecuritySpy struct {
		URL string `toml:"url"`
	} `toml:"security_spy"`
	Port      int    `toml:"port"`
	TempDir   string `toml:"temp_dir"`
	StateFile string `toml:"state_file"`
	sync.Mutex
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

// Start the daemon.
func Start() error {
	rand.Seed(time.Now().UnixNano())
	log.SetFlags(log.LstdFlags)
	m := &Motifini{Config: &Config{}, Flags: &Flags{}}
	if m.ParseFlags(os.Args[1:]); m.VersionReq {
		fmt.Printf("%s v%s\n", Binary, Version)
		return nil // don't run anything else w/ version request.
	}
	if err := m.GetConfig(); err != nil {
		m.flag.Usage()
		return err
	}
	if err := m.Validate(); err != nil {
		return errors.Wrap(err, "config invalid")
	}
	if strings.HasPrefix(m.Imessage.DBPath, "~") {
		m.Imessage.DBPath = os.Getenv("HOME") + strings.TrimPrefix(m.Imessage.DBPath, "~")
	}
	return m.Run()
}

// Run starts the app after all configs are collected.
func (m *Motifini) Run() error {
	log.Printf("iMessage Relay %v Starting! (PID: %v)", Version, os.Getpid())
	err := m.GetCamNumbers()
	if err != nil {
		return errors.Wrap(err, "securityspy")
	}
	if m.Subscribe, err = subscribe.GetDB(m.StateFile); err != nil {
		return errors.Wrap(err, "sub state")
	}
	m.Messages, err = imessage.Init(&imessage.Config{
		SQLPath:   m.Imessage.DBPath,
		QueueSize: m.Imessage.QueueSize,
		ClearMsgs: m.Imessage.ClearMessages,
		Retries:   3,
		Interval:  250 * time.Millisecond,
	})
	if err != nil {
		return errors.Wrap(err, "initializing imessage")
	}
	if m.Debug {
		m.DebugLog = log.Printf
	}
	m.Messages.ErrorLog = log.Printf
	if err = m.Start(); err != nil {
		return errors.Wrap(err, "starting imessage")
	}
	m.Exports(m.ConfigFile)
	go m.taskPoller()
	return m.StartServer()
}

// ParseFlags runs the parser for CLI arguments.
func (m *Motifini) ParseFlags(args []string) {
	m.flag = flag.NewFlagSet(Binary, flag.ExitOnError)
	m.flag.Usage = func() {
		fmt.Printf("Usage: %s [--config=filepath] [--version] [--debug]", Binary)
		m.flag.PrintDefaults()
	}
	m.flag.StringVarP(&m.ConfigFile, "config", "c", "/usr/local/etc/"+Binary+".conf", "Config File")
	m.flag.BoolVarP(&m.Debug, "debug", "D", false, "Turn on the Spam (default false).")
	m.flag.BoolVarP(&m.VersionReq, "version", "v", false, "Print the version and exit")
	_ = m.flag.Parse(args)
}

// GetConfig parses and returns our configuration data.
// Supports any format for config file: xml, yaml, json, toml
func (m *Motifini) GetConfig() error {
	// Preload our defaults.
	m.Config = &Config{}
	log.Printf("Loading Configuration File: %s", m.ConfigFile)
	switch buf, err := ioutil.ReadFile(m.ConfigFile); {
	case err != nil:
		return err
	case strings.Contains(m.ConfigFile, ".json"):
		return json.Unmarshal(buf, m.Config)
	case strings.Contains(m.ConfigFile, ".xml"):
		return xml.Unmarshal(buf, m.Config)
	case strings.Contains(m.ConfigFile, ".yaml"):
		return yaml.Unmarshal(buf, m.Config)
	default:
		return toml.Unmarshal(buf, m.Config)
	}
}

// Validate makes sure the data in the config file is valid.
func (c *Config) Validate() error {
	if len(c.Imessage.AllowedTo) < 1 {
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
	if c.Imessage.QueueSize < 20 {
		c.Imessage.QueueSize = 20
	} else if c.Imessage.QueueSize > 500 {
		c.Imessage.QueueSize = 500
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

	if c.SecuritySpy.URL != "" && !strings.Contains(c.SecuritySpy.URL, "://") {
		log.Printf("[WARN] Security Spy URL appears malformed. Ignoring it and using AppleScript!")
		c.SecuritySpy.URL = ""
	} else if c.SecuritySpy.URL != "" && !strings.HasSuffix(c.SecuritySpy.URL, "/") {
		c.SecuritySpy.URL += "/"
	}
	return nil
}

// Exports makes all the expvar data available.
func (m *Motifini) Exports(configFile string) {
	m.Map = GetPublishedMap("iMessageRelay")
	m.Set("app_started", &m.startAt)
	m.Set("app_version", &m.version)
	m.Set("config_file", &m.configFile)
	m.Set("listen_port", &m.listenPort)
	m.Set("http_visits", &m.httpVisits)
	m.Set("default_url", &m.defaultURL)
	m.Set("videos_sent", &m.videos)
	m.Set("photos_sent", &m.pics)
	m.Set("messge_sent", &m.texts)
	m.Set("error_count", &m.errors)
	// Set static data now.
	m.startAt.Set(time.Now().String())
	m.version.Set(Version)
	m.configFile.Set(configFile)
	m.listenPort.Set(int64(m.Port))
}

// StartServer creates the http routers and starts http server
func (m *Motifini) StartServer() error {
	// Listen to all incoming imessages, pass them to our handler.
	m.IncomingCall(".*", m.recvMessageHandler)
	log.Printf("Listening on port %d", m.Port)
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
		Addr:         fmt.Sprintf("127.0.0.1:%d", m.Port),
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      r, // *mux.Router
	}
	return m.ListenAndServe()
}

// taskPoller runs things at an interval and
// looks for an exit signal to shut down the http server.
func (m *Motifini) taskPoller() {
	ticker := time.NewTicker(10 * time.Minute)
	signal.Notify(StopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	for {
		select {
		case <-ticker.C:
			if err := m.GetCamNumbers(); err != nil {
				log.Printf("[ERROR] Unable to update camera names. Is SecuritySpy running? %v", err)
			}
			if err := m.SaveStateFile(); err != nil {
				log.Printf("[ERROR] Error saving subscribers state file. %v", err)
			}
		case sig := <-StopChan:
			log.Println("Exiting! Caught Signal:", sig)
			if err := m.SaveStateFile(); err != nil {
				log.Printf("[ERROR] Error saving subscribers state file. %v", err)
			}
			// Give the http server up to 3 seconds to finish any open requests.
			if m.Server != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				_ = m.Shutdown(ctx)
			}
			return
		}
	}
}
