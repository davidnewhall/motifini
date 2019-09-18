package cli

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/davidnewhall/motifini/pkg/export"
	"github.com/davidnewhall/motifini/pkg/messenger"
	"github.com/davidnewhall/motifini/pkg/webserver"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"golift.io/imessage"
	"golift.io/securityspy"
	"golift.io/subscribe"
)

var (
	// DefaultRepeatDelay is the default delay for subscriber's event notifications.
	DefaultRepeatDelay = time.Minute
	// Version of the application. Injected at build time.
	Version = "development"
	// Binary is the app name.
	Binary = "motifini"
)

// Motifini is the main application struct.
type Motifini struct {
	Flag  *Flags
	Conf  *Config
	HTTP  *webserver.Config
	SSpy  *securityspy.Server
	Subs  *subscribe.Subscribe
	Msgs  *messenger.Messenger
	Info  *log.Logger
	Error *log.Logger
	Debug *log.Logger
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
		Port      uint     `toml:"port"`
		TempDir   string   `toml:"temp_dir"`
		StateFile string   `toml:"state_file"`
		AllowedTo []string `toml:"allowed_to"`
	} `toml:"motifini"`
	Imessage    *imessage.Config    `toml:"imessage"`
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
	export.Init(Binary) // Initialize the main expvar map.
	export.Map.Version.Set(Version)
	export.Map.ConfigFile.Set(m.Flag.ConfigFile)
	if err := m.ParseConfigFile(); err != nil {
		m.Flag.Usage()
		return err
	}
	export.Map.ListenPort.Set(int64(m.Conf.Global.Port))
	m.Info.Printf("Motifini %v Starting! (PID: %v)", Version, os.Getpid())
	defer m.Info.Printf("Exiting!")

	m.Conf.Validate()
	return m.Run()
}

func (m *Motifini) setLogging() {
	debugOut := ioutil.Discard
	flags := log.LstdFlags
	if m.Flag.Debug {
		debugOut = os.Stdout
		flags = log.LstdFlags | log.Lshortfile
	}
	m.Info = log.New(os.Stdout, "[INFO] ", flags)
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
	m.Info.Println("Connecting to SecuritySpy:", m.Conf.SecuritySpy.URL)
	if m.SSpy, err = securityspy.GetServer(m.Conf.SecuritySpy); err != nil {
		return err
	}
	m.ProcessEventStream()
	defer m.SSpy.Events.Stop(true)
	m.Info.Println("Opening Subscriber Database:", m.Conf.Global.StateFile)
	if m.Subs, err = subscribe.GetDB(m.Conf.Global.StateFile); err != nil {
		return errors.Wrap(err, "sub state")
	}

	m.Msgs = &messenger.Messenger{
		SSpy:    m.SSpy,
		Subs:    m.Subs,
		Conf:    m.Conf.Imessage,
		TempDir: m.Conf.Global.TempDir,
		Info:    log.New(os.Stdout, "[MSGS] ", m.Info.Flags()),
		Debug:   m.Debug,
		Error:   m.Error,
	}
	if err := messenger.New(m.Msgs); err != nil {
		return err
	}

	m.HTTP = &webserver.Config{
		SSpy:      m.SSpy,
		Subs:      m.Subs,
		Msgs:      m.Msgs,
		Info:      log.New(os.Stdout, "[HTTP] ", m.Info.Flags()),
		Debug:     m.Debug,
		Error:     m.Error,
		TempDir:   m.Conf.Global.TempDir,
		AllowedTo: m.Conf.Global.AllowedTo,
		Port:      m.Conf.Global.Port,
	}
	go m.waitForSignal()
	return webserver.Start(m.HTTP)
}

// waitForSignal runs things at an interval and looks for an exit signal
// then shuts down the http server and event stream watcher.
func (m *Motifini) waitForSignal() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	m.Info.Printf("Shutting down! Caught Signal: %v", <-sigChan)
	m.saveSubDB()
	m.HTTP.Stop()
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
