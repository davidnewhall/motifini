package motifini

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/davidnewhall/motifini/pkg/export"
	"github.com/davidnewhall/motifini/pkg/messenger"
	"github.com/davidnewhall/motifini/pkg/webserver"
	"github.com/spf13/pflag"
	"golift.io/cnfg"
	"golift.io/cnfg/cnfgfile"
	"golift.io/imessage"
	"golift.io/securityspy"
	"golift.io/securityspy/server"
	"golift.io/subscribe"
	"golift.io/version"
)

const (
	Binary             = "motifini"
	DefaultRepeatDelay = time.Minute
	DefaultEnvPrefix   = "MO"
)

const minQueueSize = 20

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
	EnvPrefix  string
	ConfigFile string
	VersionReq bool
	*pflag.FlagSet
}

// Configuration for Motifini.
type Config struct {
	Global struct {
		Port      uint     `toml:"port"`
		TempDir   string   `toml:"temp_dir"`
		StateFile string   `toml:"state_file"`
		AllowedTo []string `toml:"allowed_to"`
		Debug     bool
	} `toml:"motifini"`
	Imessage    *imessage.Config `toml:"imessage"`
	SecuritySpy *server.Config   `toml:"security_spy"`
}

// ParseArgs runs the parser for CLI arguments.
func (flag *Flags) ParseArgs(args []string) {
	*flag = Flags{FlagSet: pflag.NewFlagSet(Binary, pflag.ExitOnError)}

	flag.Usage = func() {
		fmt.Printf("Usage: %s [--config=filepath] [--version] [--debug]", Binary) //nolint:forbidigo
		flag.PrintDefaults()
	}

	flag.StringVarP(&flag.EnvPrefix, "prefix", "p", DefaultEnvPrefix, "Environment Variable Configuration Prefix")
	flag.StringVarP(&flag.ConfigFile, "config", "c", "/usr/local/etc/"+Binary+".conf", "Config File")
	flag.BoolVarP(&flag.VersionReq, "version", "v", false, "Print the version and exit")
	_ = flag.Parse(args) // flag.ExitOnError means this will never return != nil
}

// Start the daemon.
func Start() error {
	rand.Seed(time.Now().UnixNano())

	m := &Motifini{Flag: &Flags{}, Info: log.New(os.Stdout, "[INFO] ", log.LstdFlags)}
	m.Flag.ParseArgs(os.Args[1:])

	if m.Flag.VersionReq {
		fmt.Println(version.Print(Binary)) //nolint:forbidigo
		return nil                         // don't run anything else w/ version request.
	}

	if err := m.ParseConfigFile(); err != nil {
		m.Flag.Usage()
		return err
	}

	m.setLogging()
	export.Init(Binary) // Initialize the main expvar map.
	export.Map.ListenPort.Set(int64(m.Conf.Global.Port))
	export.Map.Version.Set(version.Version + "-" + version.Revision)
	export.Map.ConfigFile.Set(m.Flag.ConfigFile)
	m.Info.Printf("Motifini %v-%v Starting! (PID: %v)", version.Version, version.Revision, os.Getpid())
	m.Conf.Validate()

	defer m.Info.Printf("Exiting!")

	return m.Run()
}

func (m *Motifini) setLogging() {
	debugOut := ioutil.Discard
	flags := log.LstdFlags

	if m.Conf.Global.Debug {
		debugOut = os.Stdout
		flags = log.LstdFlags | log.Lshortfile
	}

	m.Info = log.New(os.Stdout, "[INFO] ", flags)
	m.Error = log.New(os.Stderr, "[ERROR] ", flags)
	m.Debug = log.New(debugOut, "[DEBUG] ", flags)
}

// ParseConfigFile parses and returns our configuration data.
// Supports a few formats for config file: xml, json, toml.
func (m *Motifini) ParseConfigFile() error {
	// Preload our defaults.
	m.Conf = &Config{}
	m.Info.Println("Loading Configuration File:", m.Flag.ConfigFile)

	err := cnfgfile.Unmarshal(m.Conf, m.Flag.ConfigFile)
	if err != nil {
		return fmt.Errorf("config file: %w", err)
	}

	_, err = cnfg.UnmarshalENV(m.Conf, m.Flag.EnvPrefix)
	if err != nil {
		return fmt.Errorf("env vars: %w", err)
	}

	return nil
}

// Validate makes sure the data in the config file is valid.
func (c *Config) Validate() {
	if c.Global.TempDir == "" {
		c.Global.TempDir = "/tmp/"
	} else if !strings.HasSuffix(c.Global.TempDir, "/") {
		c.Global.TempDir += "/"
	}

	if c.Imessage.QueueSize < minQueueSize {
		c.Imessage.QueueSize = minQueueSize
	}
}

// Run starts the app after all configs are collected.
func (m *Motifini) Run() (err error) {
	m.Info.Println("Opening Subscriber Database:", m.Conf.Global.StateFile)

	if m.Subs, err = subscribe.GetDB(m.Conf.Global.StateFile); err != nil {
		return fmt.Errorf("sub state: %w", err)
	}

	m.Info.Println("Connecting to SecuritySpy:", m.Conf.SecuritySpy.URL)

	if m.SSpy, err = securityspy.New(m.Conf.SecuritySpy); err != nil {
		return fmt.Errorf("connecting to securityspy: %w", err)
	}

	m.ProcessEventStream()
	defer m.SSpy.Events.Stop(true)

	m.Msgs = &messenger.Messenger{
		SSpy:    m.SSpy,
		Subs:    m.Subs,
		Conf:    m.Conf.Imessage,
		TempDir: m.Conf.Global.TempDir,
		Info:    log.New(os.Stdout, "[MSGS] ", m.Info.Flags()),
		Debug:   m.Debug,
		Error:   m.Error,
	}
	if err = messenger.New(m.Msgs); err != nil {
		return fmt.Errorf("connecting to messenger: %w", err)
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

	if err = webserver.Start(m.HTTP); err != nil {
		return fmt.Errorf("webserver problem: %w", err)
	}

	return m.waitForSignal()
}

// waitForSignal runs things at an interval and looks for an exit signal
// then shuts down the http server and event stream watcher.
func (m *Motifini) waitForSignal() error {
	sigChan := make(chan os.Signal, 1)

	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	m.Info.Printf("Shutting down! Caught Signal: %v", <-sigChan)
	m.saveSubDB()

	return m.HTTP.Stop()
}

// saveSubDB just saves the state file/db and logs any error.
// called from a few places. SaveStateFile() provides the file lock.
func (m *Motifini) saveSubDB() {
	if err := m.Subs.StateFileSave(); err != nil {
		m.Error.Printf("saving subscribers state file: %v", err)
		return
	}

	m.Debug.Print("Saved state DB file: " + m.Conf.Global.StateFile)
}
