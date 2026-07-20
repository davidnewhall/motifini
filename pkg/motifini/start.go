package motifini

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/davidnewhall/motifini/pkg/chat"
	"github.com/davidnewhall/motifini/pkg/export"
	"github.com/davidnewhall/motifini/pkg/messenger"
	"github.com/davidnewhall/motifini/pkg/webserver"
	"github.com/spf13/pflag"
	"golift.io/cnfg"
	"golift.io/cnfgfile"
	"golift.io/securityspy/v2"
	"golift.io/securityspy/v2/server"
	"golift.io/subscribe"
	"golift.io/version"
)

// Application identity and default timing values.
const (
	Binary           = "motifini"
	DefaultEnvPrefix = "MO"
)

// DefaultRepeatDelay mirrors chat.DefaultRepeatDelay for callers outside pkg/chat.
const DefaultRepeatDelay = chat.DefaultRepeatDelay

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
	*pflag.FlagSet

	EnvPrefix  string
	ConfigFile string
	VersionReq bool
}

// Config is the configuration for Motifini.
type Config struct {
	Global struct {
		TempDir   string `toml:"temp_dir"`
		StateFile string `toml:"state_file"`
		Debug     bool   `toml:"debug"`
	} `toml:"motifini"`
	Webserver struct {
		Port      uint     `toml:"port"`
		AllowedTo []string `toml:"allowed_to"`
		Enable    bool     `toml:"enable"`
	} `toml:"webserver"`
	Telegram    *messenger.TelegramConfig `toml:"telegram"`
	SecuritySpy *server.Config            `toml:"security_spy"`
}

// ParseArgs runs the parser for CLI arguments.
func (flag *Flags) ParseArgs(args []string) {
	*flag = Flags{FlagSet: pflag.NewFlagSet(Binary, pflag.ExitOnError)}

	flag.Usage = func() {
		fmt.Printf("Usage: %s [--config=filepath] [--version] [--debug]", Binary) //nolint:forbidigo // cli usage
		flag.PrintDefaults()
	}

	flag.StringVarP(&flag.EnvPrefix, "prefix", "p", DefaultEnvPrefix, "Environment Variable Configuration Prefix")
	flag.StringVarP(&flag.ConfigFile, "config", "c", "/usr/local/etc/"+Binary+".conf", "Config File")
	flag.BoolVarP(&flag.VersionReq, "version", "v", false, "Print the version and exit")
	_ = flag.Parse(args) // flag.ExitOnError means this will never return != nil
}

// Start the daemon.
func Start() error {
	app := &Motifini{Flag: &Flags{}, Info: log.New(os.Stdout, "[INFO] ", log.LstdFlags)}
	app.Flag.ParseArgs(os.Args[1:])

	if app.Flag.VersionReq {
		fmt.Println(version.Print(Binary)) //nolint:forbidigo // version request
		return nil                         // don't run anything else w/ version request.
	}

	err := app.ParseConfigFile()
	if err != nil {
		app.Flag.Usage()
		return err
	}

	const maxPort = 65535
	if app.Conf.Webserver.Port > maxPort {
		app.Conf.Webserver.Port = maxPort
	}

	app.setLogging()
	export.Init(Binary)                                       // Initialize the main expvar map.
	export.Map.ListenPort.Set(int64(app.Conf.Webserver.Port)) //nolint:gosec // caught above.
	export.Map.Version.Set(version.Version + "-" + version.Revision)
	export.Map.ConfigFile.Set(app.Flag.ConfigFile)
	app.Info.Printf("Motifini %v-%v Starting! (PID: %v)", version.Version, version.Revision, os.Getpid())
	app.Conf.Validate()

	defer app.Info.Printf("Exiting!")

	return app.Run()
}

func (m *Motifini) setLogging() {
	debugOut := io.Discard
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
}

// Run starts the app after all configs are collected.
func (m *Motifini) Run() error {
	m.Info.Println("Opening Subscriber Database:", m.Conf.Global.StateFile)

	var err error

	m.Subs, err = subscribe.GetDB(m.Conf.Global.StateFile)
	if err != nil {
		return fmt.Errorf("sub state: %w", err)
	}

	chat.EnsureBuiltInEvents(m.Subs)

	m.Info.Println("Connecting to SecuritySpy:", m.Conf.SecuritySpy.URL)

	m.SSpy, err = securityspy.New(m.Conf.SecuritySpy)
	if err != nil {
		return fmt.Errorf("connecting to securityspy: %w", err)
	}

	m.publishDebugStats()
	m.ProcessEventStream()
	defer m.SSpy.Events.Stop(true)

	err = m.startMessenger()
	if err != nil {
		return err
	}

	if m.Conf.Webserver.Enable {
		err = m.startWebserver()
		if err != nil {
			return err
		}
	}

	return m.waitForSignal()
}

// startMessenger builds and connects the chat/messenger stack.
func (m *Motifini) startMessenger() error {
	m.Msgs = &messenger.Messenger{
		Chat:     chat.New(&chat.Chat{TempDir: m.Conf.Global.TempDir, Subs: m.Subs, SSpy: m.SSpy}),
		Subs:     m.Subs,
		Telegram: m.Conf.Telegram,
		TempDir:  m.Conf.Global.TempDir,
		Info:     log.New(os.Stdout, "[MSGS] ", m.Info.Flags()),
		Debug:    m.Debug,
		Error:    m.Error,
	}

	err := messenger.New(m.Msgs)
	if err != nil {
		return fmt.Errorf("connecting to messenger: %w", err)
	}

	return nil
}

// startWebserver builds and starts the HTTP API.
func (m *Motifini) startWebserver() error {
	m.HTTP = &webserver.Config{
		SSpy:      m.SSpy,
		Subs:      m.Subs,
		Msgs:      m.Msgs,
		Info:      log.New(os.Stdout, "[HTTP] ", m.Info.Flags()),
		Debug:     m.Debug,
		Error:     m.Error,
		TempDir:   m.Conf.Global.TempDir,
		AllowedTo: m.Conf.Webserver.AllowedTo,
		Port:      m.Conf.Webserver.Port,
	}

	err := webserver.Start(m.HTTP)
	if err != nil {
		return fmt.Errorf("webserver problem: %w", err)
	}

	return nil
}

// publishDebugStats exposes live gauges on /debug/vars.
func (m *Motifini) publishDebugStats() {
	export.PublishCount("subscribers", func() int64 {
		if m.Subs == nil {
			return 0
		}

		return int64(len(m.Subs.Subscribers))
	})
	export.PublishCount("admins", func() int64 {
		if m.Subs == nil {
			return 0
		}

		return int64(len(m.Subs.GetAdmins()))
	})
	export.PublishCount("cameras", func() int64 {
		if m.SSpy == nil {
			return 0
		}

		return int64(len(m.SSpy.Cameras.All()))
	})
	export.PublishCount("cameras_online", func() int64 {
		if m.SSpy == nil {
			return 0
		}

		var online int64
		for _, cam := range m.SSpy.Cameras.All() {
			if cam != nil && cam.Connected.Val {
				online++
			}
		}

		return online
	})
}

// waitForSignal runs things at an interval and looks for an exit signal
// then shuts down the http server and event stream watcher.
func (m *Motifini) waitForSignal() error {
	sigChan := make(chan os.Signal, 1)

	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	m.Info.Printf("Shutting down! Caught Signal: %v", <-sigChan)
	m.saveSubDB()

	if m.HTTP != nil {
		err := m.HTTP.Stop()
		if err != nil {
			return fmt.Errorf("stopping web server: %w", err)
		}
	}

	return nil
}

// saveSubDB just saves the state file/db and logs any error.
// called from a few places. SaveStateFile() provides the file lock.
func (m *Motifini) saveSubDB() {
	err := m.Subs.StateFileSave()
	if err != nil {
		m.Error.Printf("saving subscribers state file: %v", err)
		return
	}

	m.Debug.Print("Saved state DB file: " + m.Conf.Global.StateFile)
}
