package web

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/drumandbytes/delistd/internal/broker"
	"github.com/drumandbytes/delistd/internal/config"
	"github.com/drumandbytes/delistd/internal/datadir"
	"github.com/drumandbytes/delistd/internal/history"
	"github.com/drumandbytes/delistd/internal/schedule"
	emailtmpl "github.com/drumandbytes/delistd/internal/template"
)

const (
	defaultRateLimit  = 30
	defaultRateWindow = time.Minute
	defaultSessionTTL = 30 * time.Minute
)

// Version is the build version shown in the footer. main sets it from its
// -ldflags value at startup; it stays "dev" otherwise.
var Version = "dev"

// Server is the local web UI.
type Server struct {
	// Settings the caller sets after NewServer.
	BrokerOverride string   // serve's --brokers, so a reload resolves the list as startup did
	ListenHost     string   // serve's --listen; empty binds 127.0.0.1, anything else is for containers
	AllowedHosts   []string // serve's --allowed-hosts / DELISTD_ALLOWED_HOSTS; "*" allows any Host

	// State shared with the handlers. config and brokerDB are swapped whole,
	// so read them through getConfig and brokers.
	config       atomic.Pointer[config.Config]
	brokerDB     atomic.Pointer[broker.BrokerDatabase]
	configPath   string
	dataDir      string // the config directory: schedule lock and state live here
	reloadMu     sync.Mutex
	cfgStamp     string // version of config.yaml the current config came from (guarded by reloadMu)
	historyStore *history.Store
	tmplEngine   *emailtmpl.Engine
	templates    map[string]*template.Template
	sessions     *SessionStore
	rateLimiter  *RateLimiter
	jobManager   *JobManager
	port         int

	brokerUpdateURL string // broker.DefaultUpdateURL; tests point it at a local server

	// Lifecycle, guarded by lifeMu.
	lifeMu        sync.Mutex
	httpServer    *http.Server
	stopScheduler context.CancelFunc
	shutDown      bool

	// The in-app scheduler (scheduler.go). The OS hooks are fields so tests
	// never touch the real launchd or systemd.
	cycleMu      sync.Mutex
	cycleRunning bool
	osInstalled  func() bool
	installOS    func(schedule.Job) error
	removeOS     func() error
}

// NewServer wires a server around an already loaded config, broker list,
// history store and template engine. configPath is empty only in tests.
func NewServer(port int, cfg *config.Config, configPath string, brokerDB *broker.BrokerDatabase, store *history.Store, engine *emailtmpl.Engine) (*Server, error) {
	pages, err := loadTemplates()
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	s := &Server{
		configPath:      configPath,
		dataDir:         dataDirFor(configPath),
		historyStore:    store,
		tmplEngine:      engine,
		templates:       pages,
		sessions:        NewSessionStore(defaultSessionTTL),
		rateLimiter:     NewRateLimiter(defaultRateLimit, defaultRateWindow),
		jobManager:      NewJobManager(),
		port:            port,
		brokerUpdateURL: broker.DefaultUpdateURL,
		osInstalled:     schedule.Installed,
		installOS:       schedule.Install,
		removeOS:        schedule.Remove,
	}
	s.config.Store(cfg)
	s.brokerDB.Store(brokerDB)

	if configPath != "" {
		// "none" (no file yet) differs from every real stamp, so a config.yaml
		// the CLI creates later is picked up.
		if s.cfgStamp = fileStamp(configPath); s.cfgStamp == "" {
			s.cfgStamp = "none"
		}
		removeStaleJobFiles(s.dataDir)
	}
	return s, nil
}

// dataDirFor is the config's directory, or the default data directory when
// there is no config path (tests).
func dataDirFor(configPath string) string {
	if configPath != "" {
		return filepath.Dir(configPath)
	}
	home, _ := os.UserHomeDir()
	return datadir.Of(home)
}

// removeStaleJobFiles drops the files paused send jobs were once saved to for
// resuming after a restart; nothing writes or reads them any more.
func removeStaleJobFiles(dir string) {
	stale, _ := filepath.Glob(filepath.Join(dir, "pending_job*.json"))
	for _, f := range stale {
		_ = os.Remove(f)
	}
}

// brokers is the current broker list.
func (s *Server) brokers() *broker.BrokerDatabase { return s.brokerDB.Load() }

// getConfig is the current config. Settings handlers write it while other
// handlers and the send job read it, so writers load, copy, mutate and store.
func (s *Server) getConfig() *config.Config { return s.config.Load() }

// Start serves until Shutdown. It returns at once if Shutdown already ran.
func (s *Server) Start() error {
	router := s.setupRouter()

	s.lifeMu.Lock()
	if s.shutDown {
		s.lifeMu.Unlock()
		return nil
	}
	srv := &http.Server{
		Addr:         listenAddr(s.ListenHost, s.port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	s.httpServer = srv
	schedCtx, stopSched := context.WithCancel(context.Background())
	s.stopScheduler = stopSched
	s.lifeMu.Unlock()

	// Listen before opening the browser, so there is a server to load.
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		stopSched()
		return fmt.Errorf("server error: %w", err)
	}

	go s.runScheduler(schedCtx)
	if IsLoopbackListen(s.ListenHost) { // a non-loopback bind is a container: no browser there
		go OpenBrowser(fmt.Sprintf("http://localhost:%d", s.port))
	}
	fmt.Printf("Starting Delistd web UI at http://localhost:%d\n", s.port)
	fmt.Println("Press Ctrl+C to stop")

	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

// Shutdown stops the scheduler and the server. Called before Start, it makes
// Start return immediately.
func (s *Server) Shutdown(ctx context.Context) error {
	s.lifeMu.Lock()
	defer s.lifeMu.Unlock()
	s.shutDown = true
	if s.stopScheduler != nil {
		s.stopScheduler()
	}
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}
