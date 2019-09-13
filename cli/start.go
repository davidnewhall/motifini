package cli

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"golift.io/imessage"
	"golift.io/securityspy"
	"golift.io/subscribe"
)

var (
	// Version of the application. Injected at build time.
	Version = "development"
	// Binary is the app name.
	Binary = "motifini"
)

const (
	// APIiMessage is just an identifier for an imessage contact type.
	APIiMessage = "imessage"
)

// Motifini is the main application struct.
type Motifini struct {
	Flags   *Flags
	Config  *Config
	Debug   *Log
	Server  *http.Server
	Spy     *securityspy.Server
	Subs    *subscribe.Subscribe
	Msgs    *imessage.Messages
	exports exportData
}

// Flags defines our application's CLI arguments.
type Flags struct {
	Debug      bool
	VersionReq bool
	ConfigFile string
	*pflag.FlagSet
}

// Config struct
type Config struct {
	Global struct {
		Port      int    `toml:"port"`
		TempDir   string `toml:"temp_dir"`
		StateFile string `toml:"state_file"`
	} `toml:"motifini"`
	Imessage struct {
		AllowedTo     []string          `toml:"allowed_to"`
		DBPath        string            `toml:"db_path"`
		QueueSize     int               `toml:"queue_size"`
		Retries       int               `toml:"retries"`
		Interval      imessage.Duration `toml:"interval"`
		ClearMessages bool              `toml:"clear_messages"`
	} `toml:"imessage"`
	SecuritySpy *securityspy.Config `toml:"security_spy"`
}

// ParseArgs runs the parser for CLI arguments.
func (flag *Flags) ParseArgs(args []string) {
	*flag = Flags{FlagSet: pflag.NewFlagSet(Binary, pflag.ExitOnError)}
	flag.Usage = func() {
		fmt.Printf("Usage: %s [--config=filepath] [--version] [--debug]", Binary)
		flag.PrintDefaults()
	}
	flag.StringVarP(&flag.ConfigFile, "config", "c", "/usr/local/etc/"+Binary+".conf", "Config File")
	flag.BoolVarP(&flag.Debug, "debug", "D", false, "Turn on the Spam.")
	flag.BoolVarP(&flag.VersionReq, "version", "v", false, "Print the version and exit")
	_ = flag.Parse(args) // flag.ExitOnError means this will never return != nil
}

// Start the daemon.
func Start() error {
	rand.Seed(time.Now().UnixNano())
	log.SetFlags(log.LstdFlags)
	m := &Motifini{Config: &Config{}, Flags: &Flags{}}
	if m.Flags.ParseArgs(os.Args[1:]); m.Flags.VersionReq {
		fmt.Printf("%s v%s\n", Binary, Version)
		return nil // don't run anything else w/ version request.
	}
	m.Debug = &Log{Muted: !m.Flags.Debug}
	if err := m.Config.ParseFile(m.Flags.ConfigFile); err != nil {
		m.Flags.Usage()
		return err
	}
	m.Config.Validate()
	log.Printf("[INFO] Motifini %v Starting! (PID: %v)", Version, os.Getpid())
	defer log.Printf("[WARN] Exiting!")
	return m.Run()
}

// ParseFile parses and returns our configuration data.
// Supports a few formats for config file: xml, json, toml
func (c *Config) ParseFile(configFile string) error {
	// Preload our defaults.
	*c = Config{}
	log.Printf("[INFO] Loading Configuration File: %s", configFile)
	switch buf, err := ioutil.ReadFile(configFile); {
	case err != nil:
		return err
	case strings.Contains(configFile, ".json"):
		return json.Unmarshal(buf, c)
	case strings.Contains(configFile, ".xml"):
		return xml.Unmarshal(buf, c)
	default:
		return toml.Unmarshal(buf, c)
	}
}

// Validate makes sure the data in the config file is valid.
func (c *Config) Validate() {
	if c.Global.Port == 0 {
		c.Global.Port = 8765
	}
	if c.Global.TempDir == "" {
		c.Global.TempDir = "/tmp/"
	} else if !strings.HasSuffix(c.Global.TempDir, "/") {
		c.Global.TempDir += "/"
	}
	if c.Imessage.QueueSize < 20 {
		c.Imessage.QueueSize = 20
	} else if c.Imessage.QueueSize > 500 {
		c.Imessage.QueueSize = 500
	}
	if c.SecuritySpy.URL != "" && !strings.HasSuffix(c.SecuritySpy.URL, "/") {
		c.SecuritySpy.URL += "/"
	}
}

// Run starts the app after all configs are collected.
func (m *Motifini) Run() error {
	var err error
	log.Println("[INFO] Connecting to SecuritySpy:", m.Config.SecuritySpy.URL)
	if m.Spy, err = securityspy.GetServer(m.Config.SecuritySpy); err != nil {
		return err
	}
	log.Println("[INFO] Opening Subscriber Database:", m.Config.Global.StateFile)
	if m.Subs, err = subscribe.GetDB(m.Config.Global.StateFile); err != nil {
		return errors.Wrap(err, "sub state")
	}
	log.Println("[INFO] Watching iMessage Database:", m.Config.Imessage.DBPath)
	if err := m.startiMessage(); err != nil {
		return err
	}

	m.exportData()
	m.processEventStream()
	defer m.Spy.Events.Stop()
	go m.waitForSignal()
	return m.StartWebServer()
}

// waitForSignal runs things at an interval and looks for an exit signal
// then shuts down the http server and event stream watcher.
func (m *Motifini) waitForSignal() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	log.Printf("[INFO] Shutting down! Caught Signal: %v", <-sigChan)
	m.saveSubDB()
	m.StopWebServer()
}

// saveSubDB just saves the state file/db and logs any error.
// called from a few places. SaveStateFile() provides the file lock.
func (m *Motifini) saveSubDB() {
	if err := m.Subs.StateFileSave(); err != nil {
		log.Printf("[ERROR] saving subscribers state file: %v", err)
		return
	}
	m.Debug.Print("[DEBUG] Saved state DB file")
}
