package motifini

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/davidnewhall/motifini/pkg/chat"
	"github.com/davidnewhall/motifini/pkg/export"
	"github.com/davidnewhall/motifini/pkg/messenger"
	"github.com/davidnewhall/motifini/pkg/webserver"
	"github.com/spf13/pflag"
	"golift.io/cnfg"
	"golift.io/cnfgfile"
	"golift.io/rotatorr"
	"golift.io/rotatorr/timerotator"
	"golift.io/securityspy/v2"
	"golift.io/securityspy/v2/server"
	"golift.io/subscribe"
	"golift.io/version"
)

// Application identity and default timing values.
const (
	Binary           = "motifini"
	DefaultEnvPrefix = "MO"

	defaultLogFileMb        = 5
	defaultLogFiles         = 10
	defaultSecuritySpyRetry = 5 * time.Second
	megabyte                = 1024 * 1024
)

// DefaultRepeatDelay mirrors chat.DefaultRepeatDelay for callers outside pkg/chat.
const DefaultRepeatDelay = chat.DefaultRepeatDelay

// Motifini is the main application struct.
type Motifini struct {
	Flag          *Flags
	Conf          *Config
	HTTP          *webserver.Config
	SSpy          *securityspy.Server
	Subs          *subscribe.Subscribe
	Msgs          *messenger.Messenger
	Info          *log.Logger
	Error         *log.Logger
	Debug         *log.Logger
	Event         *log.Logger // SecuritySpy event stream (optional rotating file)
	logWriter     io.Writer   // Info/MSGS/HTTP sink (stdout and/or rotating log_file)
	appLog        io.Closer
	eventLog      io.Closer
	streamLive    bool // true between EventStreamConnect and Disconnect
	streamSawDown bool // true after a real disconnect (so "back up" is meaningful)
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
		TempDir          string        `toml:"temp_dir"`
		StateFile        string        `toml:"state_file"`
		LogFile          string        `toml:"log_file"`           // optional rotating app log (info/error/debug)
		EventLog         string        `toml:"event_log"`          // optional rotating SecuritySpy event stream log
		LogFiles         int           `toml:"log_files"`          // rotated log file count (default 10)
		LogFileMb        int           `toml:"log_file_mb"`        // rotated log size in MB (default 5)
		SecuritySpyRetry cnfg.Duration `toml:"security_spy_retry"` // reconnect interval when SS is down (default 5s)
		Debug            bool          `toml:"debug"`
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
	flag.StringVarP(&flag.ConfigFile, "config", "c", "/opt/homebrew/etc/"+Binary+".conf", "Config File")
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

	app.Conf.Validate()
	// Print log paths to stdout before setLogging may redirect away from the console.
	// Config path was already printed by ParseConfigFile ("Loading Configuration File: ...").
	if path := strings.TrimSpace(app.Conf.Global.LogFile); path != "" {
		app.Info.Println("App log file:", path)
	}

	if path := strings.TrimSpace(app.Conf.Global.EventLog); path != "" {
		app.Info.Println("Event log file:", path)
	}

	app.setLogging()
	defer app.closeLogging()

	export.Init(Binary)                                       // Initialize the main expvar map.
	export.Map.ListenPort.Set(int64(app.Conf.Webserver.Port)) //nolint:gosec // caught above.
	export.Map.Version.Set(version.Version + "-" + version.Revision)
	export.Map.ConfigFile.Set(app.Flag.ConfigFile)
	app.Info.Printf("Motifini %v-%v Starting! (PID: %v)", version.Version, version.Revision, os.Getpid())

	defer app.Info.Printf("Exiting!")

	return app.Run()
}

func (m *Motifini) setLogging() {
	flags := log.LstdFlags
	infoOut := io.Writer(os.Stdout)
	errOut := io.Writer(os.Stderr)
	debugOut := io.Discard

	if path := strings.TrimSpace(m.Conf.Global.LogFile); path != "" {
		rotator, err := m.newRotator(path)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "[ERROR] Opening log file %q: %v (logging to stdout/stderr only)\n", path, err)
		} else {
			m.appLog = rotator
			infoOut = rotator
			errOut = rotator
		}
	}

	if m.Conf.Global.Debug {
		debugOut = infoOut
		flags = log.LstdFlags | log.Lshortfile
	}

	m.logWriter = infoOut
	m.Info = log.New(infoOut, "[INFO] ", flags)
	m.Error = log.New(errOut, "[ERROR] ", flags)
	m.Debug = log.New(debugOut, "[DEBUG] ", flags)
	m.Event = log.New(io.Discard, "[EVENT] ", flags)

	if path := strings.TrimSpace(m.Conf.Global.EventLog); path != "" {
		m.openEventLog(path, flags)
	}
}

func (m *Motifini) newRotator(path string) (*rotatorr.Logger, error) {
	logger, err := rotatorr.New(&rotatorr.Config{
		Filepath: path,
		FileSize: int64(m.Conf.Global.LogFileMb) * megabyte,
		Rotatorr: &timerotator.Layout{FileCount: m.Conf.Global.LogFiles},
	})
	if err != nil {
		return nil, fmt.Errorf("rotatorr: %w", err)
	}

	return logger, nil
}

func (m *Motifini) openEventLog(path string, flags int) {
	if logPath := strings.TrimSpace(m.Conf.Global.LogFile); logPath != "" &&
		filepath.Clean(path) == filepath.Clean(logPath) {
		m.Error.Printf("event_log %q matches log_file; event stream logging disabled", path)
		return
	}

	rotator, err := m.newRotator(path)
	if err != nil {
		m.Error.Printf("Opening event log %q: %v (event stream logging disabled)", path, err)
		return
	}

	m.eventLog = rotator
	m.Event = log.New(rotator, "[EVENT] ", flags)
	m.Info.Printf("SecuritySpy event stream logging to %s (%d files @ %dMB)",
		path, m.Conf.Global.LogFiles, m.Conf.Global.LogFileMb)
}

func (m *Motifini) closeLogging() {
	if m.eventLog != nil {
		_ = m.eventLog.Close()
		m.eventLog = nil
	}

	if m.appLog != nil {
		_ = m.appLog.Close()
		m.appLog = nil
	}
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

	if c.Global.LogFileMb < 1 {
		c.Global.LogFileMb = defaultLogFileMb
	}

	if c.Global.LogFiles < 1 {
		c.Global.LogFiles = defaultLogFiles
	}

	if c.Global.SecuritySpyRetry.Duration <= 0 {
		c.Global.SecuritySpyRetry.Duration = defaultSecuritySpyRetry
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

	if m.connectSecuritySpy() {
		m.ProcessEventStream()
		defer m.SSpy.Events.Stop(true)
	}

	m.publishDebugStats()

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

	m.notifySystemEvent(chat.EventStarted, fmt.Sprintf(
		"Motifini %s-%s started at %s (PID %d).",
		version.Version, version.Revision,
		time.Now().Format("2006-01-02 15:04:05 MST"),
		os.Getpid()))

	return m.waitForSignal()
}

// connectSecuritySpy builds the client and refreshes once. Startup continues even when
// SecuritySpy is down; a background loop retries until Refresh succeeds.
// NewMust only builds the client (no network); connectivity failures come from Refresh.
// Returns false when [security_spy] is missing so the event stream is not started.
func (m *Motifini) connectSecuritySpy() bool {
	if m.Conf.SecuritySpy == nil || m.Conf.SecuritySpy.URL == "" {
		m.Error.Println("SecuritySpy config missing — camera features disabled")
		m.SSpy = nil

		return false
	}

	m.Info.Println("Connecting to SecuritySpy:", m.Conf.SecuritySpy.URL)

	m.SSpy = securityspy.NewMust(m.Conf.SecuritySpy)

	err := m.SSpy.Refresh()
	if err == nil {
		m.Info.Printf("Connected to SecuritySpy (%d cameras)", len(m.SSpy.Cameras.All()))
		return true
	}

	retry := m.Conf.Global.SecuritySpyRetry.Duration
	m.Error.Printf("SecuritySpy unavailable: %v — will retry every %s", err, retry)
	go m.retrySecuritySpy(retry)

	return true
}

func (m *Motifini) retrySecuritySpy(interval time.Duration) {
	for {
		time.Sleep(interval)

		err := m.SSpy.Refresh()
		if err != nil {
			m.Debug.Printf("SecuritySpy still unavailable: %v — retrying in %s", err, interval)
			continue
		}

		m.Info.Printf("Connected to SecuritySpy (%d cameras)", len(m.SSpy.Cameras.All()))

		return
	}
}

// startMessenger builds and connects the chat/messenger stack.
func (m *Motifini) startMessenger() error {
	m.Msgs = &messenger.Messenger{
		Chat: chat.New(&chat.Chat{
			TempDir: m.Conf.Global.TempDir,
			Subs:    m.Subs,
			SSpy:    m.SSpy,
			Info:    log.New(m.logWriter, "[CHAT] ", m.Info.Flags()),
			Debug:   m.Debug,
			Error:   m.Error,
		}),
		Subs:     m.Subs,
		Telegram: m.Conf.Telegram,
		TempDir:  m.Conf.Global.TempDir,
		Info:     log.New(m.logWriter, "[MSGS] ", m.Info.Flags()),
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
		Info:      log.New(m.logWriter, "[HTTP] ", m.Info.Flags()),
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
		if m.SSpy == nil || m.SSpy.Cameras == nil {
			return 0
		}

		return int64(len(m.SSpy.Cameras.All()))
	})
	export.PublishCount("cameras_online", func() int64 {
		if m.SSpy == nil || m.SSpy.Cameras == nil {
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
