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
	"github.com/davidnewhall/motifini/chat"
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
	Flag    *Flags
	Conf    *Config
	HTTP    *http.Server
	SSpy    *securityspy.Server
	Subs    *subscribe.Subscribe
	Imsg    *imessage.Messages
	Chat    *chat.Chat
	WReq    *log.Logger // web request
	MReq    *log.Logger // msg request
	Info    *log.Logger
	Warn    *log.Logger
	Error   *log.Logger
	Debug   *log.Logger
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
		AllowedTo     []string `toml:"allowed_to"`
		DBPath        string   `toml:"db_path"`
		QueueSize     int      `toml:"queue_size"`
		Retries       int      `toml:"retries"`
		ClearMessages bool     `toml:"clear_messages"`
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
	m := &Motifini{Flag: &Flags{}}
	if m.Flag.ParseArgs(os.Args[1:]); m.Flag.VersionReq {
		fmt.Printf("%s v%s\n", Binary, Version)
		return nil // don't run anything else w/ version request.
	}

	m.setLogging()
	if err := m.ParseConfigFile(); err != nil {
		m.Flag.Usage()
		return err
	}
	m.Conf.Validate()
	m.Info.Printf("Motifini %v Starting! (PID: %v)", Version, os.Getpid())
	defer m.Warn.Printf("Exiting!")
	return m.Run()
}

func (m *Motifini) setLogging() {
	debugOut := ioutil.Discard
	flags := log.LstdFlags
	if m.Flag.Debug {
		debugOut = os.Stdout
		flags = log.LstdFlags | log.Lshortfile
	}
	// TODO: figure out a better way to structure these or set them.
	m.Info = log.New(os.Stdout, "[INFO] ", flags)
	m.MReq = log.New(os.Stdout, "[MSG] ", flags)
	m.WReq = log.New(os.Stdout, "[HTTP] ", flags)
	m.Warn = log.New(os.Stderr, "[WARN] ", flags)
	m.Error = log.New(os.Stderr, "[ERROR] ", flags)
	m.Debug = log.New(debugOut, "[DEBUG] ", flags)
}

// ParseConfigFile parses and returns our configuration data.
// Supports a few formats for config file: xml, json, toml
func (m *Motifini) ParseConfigFile() error {
	// Preload our defaults.
	m.Conf = &Config{}
	m.Info.Printf("Loading Configuration File: %s", m.Flag.ConfigFile)
	switch buf, err := ioutil.ReadFile(m.Flag.ConfigFile); {
	case err != nil:
		return err
	case strings.Contains(m.Flag.ConfigFile, ".json"):
		return json.Unmarshal(buf, &m.Conf)
	case strings.Contains(m.Flag.ConfigFile, ".xml"):
		return xml.Unmarshal(buf, &m.Conf)
	default:
		return toml.Unmarshal(buf, &m.Conf)
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
	if m.Conf.SecuritySpy.URL != "" {
		m.Info.Println("Connecting to SecuritySpy:", m.Conf.SecuritySpy.URL)
		if m.SSpy, err = securityspy.GetServer(m.Conf.SecuritySpy); err != nil {
			return err
		}
		m.processEventStream()
		defer m.SSpy.Events.Stop()
	}
	m.Info.Println("Opening Subscriber Database:", m.Conf.Global.StateFile)
	if m.Subs, err = subscribe.GetDB(m.Conf.Global.StateFile); err != nil {
		return errors.Wrap(err, "sub state")
	}
	// Configure chat library.
	m.Chat = chat.New(&chat.Chat{TempDir: m.Conf.Global.TempDir, Subs: m.Subs, SSpy: m.SSpy})
	m.Info.Println("Watching iMessage Database:", m.Conf.Imessage.DBPath)
	if err := m.startiMessage(); err != nil {
		return err
	}

	m.exportData()
	go m.waitForSignal()
	return m.StartWebServer()
}

// waitForSignal runs things at an interval and looks for an exit signal
// then shuts down the http server and event stream watcher.
func (m *Motifini) waitForSignal() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	m.Info.Printf("Shutting down! Caught Signal: %v", <-sigChan)
	m.saveSubDB()
	m.StopWebServer()
}

// saveSubDB just saves the state file/db and logs any error.
// called from a few places. SaveStateFile() provides the file lock.
func (m *Motifini) saveSubDB() {
	if err := m.Subs.StateFileSave(); err != nil {
		m.Error.Printf("saving subscribers state file: %v", err)
		return
	}
	m.Debug.Print("Saved state DB file")
}
