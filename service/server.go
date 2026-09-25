package main

import (
	"net/http"

	"github.com/jamesread/RepoManager/core/engine"
	"github.com/jamesread/RepoManager/core/store"
	"github.com/sirupsen/logrus"
)

// Server hosts the RepoManager web UI and JSON API over a single HTTP port.
type Server struct {
	eng *engine.Core
	mux *http.ServeMux
}

// Run starts the web UI for root and blocks until the process exits.
func Run(root string, st *store.Store) error {
	eng := engine.New(st, root)
	defer eng.Close()
	_ = eng.Subscribe()
	eng.Start()

	s := &Server{eng: eng, mux: http.NewServeMux()}
	s.routes()

	addr := ListenAddr()
	logrus.Infof("RepoManager webui listening on %s (root %s)", addr, root)
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/projects", s.handleProjects)
	s.mux.HandleFunc("/api/project", s.handleProject)
	s.mux.HandleFunc("/api/project/icon", s.handleProjectIcon)
	s.mux.HandleFunc("/api/project/refresh", s.handleProjectRefresh)
	s.mux.HandleFunc("/api/project/delete", s.handleProjectDelete)
	s.mux.HandleFunc("/api/root", s.handleRoot)
	s.mux.HandleFunc("/api/scan", s.handleScan)
	s.mux.HandleFunc("/api/tags", s.handleTags)
	s.mux.HandleFunc("/api/annotations", s.handleAnnotations)

	staticDir := findStaticDir()
	if staticDir == "" {
		s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "frontend not built; run: make -C frontend build", http.StatusServiceUnavailable)
		})
		logrus.Warn("frontend/dist not found; API only until the SPA is built")
		return
	}
	logrus.Infof("serving webui from %s", staticDir)
	s.mux.Handle("/", spaFileServer(staticDir))
}

// Handler exposes the mux for tests.
func (s *Server) Handler() http.Handler {
	return s.mux
}

// NewForTest builds a Server around an existing engine (tests).
func NewForTest(eng *engine.Core) *Server {
	s := &Server{eng: eng, mux: http.NewServeMux()}
	s.routes()
	return s
}

func init() {
	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
}
