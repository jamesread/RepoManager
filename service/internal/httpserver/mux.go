package httpserver

import (
	"net/http"
	"strings"

	"github.com/jamesread/RepoManager/core/engine"
	"github.com/jamesread/RepoManager/core/helpers"
	"github.com/jamesread/RepoManager/service/gen/repomanager/v1/repomanagerv1connect"
	"github.com/jamesread/RepoManager/service/internal/iam"
	"github.com/jamesread/RepoManager/service/internal/server"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

func NewMux(srv *server.Server, auth *iam.AuthLayer, eng *engine.Core) *http.ServeMux {
	mux := http.NewServeMux()
	path, handler := repomanagerv1connect.NewRepoManagerServiceHandler(srv)
	handler = auth.WrapHandler(handler)
	mux.Handle(path, handler)
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/api/project/icon", iconHandler(eng))

	staticDir := findStaticDir()
	if staticDir == "" {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "frontend not built; run: make frontend", http.StatusServiceUnavailable)
		})
		logrus.Warn("frontend/dist not found; API only until the SPA is built")
		return mux
	}
	logrus.Infof("serving webui from %s", staticDir)
	mux.Handle("/", spaFileServer(staticDir))
	return mux
}

func iconHandler(eng *engine.Core) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := r.URL.Query().Get("path")
		if path == "" {
			http.Error(w, "path is required", http.StatusBadRequest)
			return
		}
		p, ok := findProject(eng.Snapshot(), path)
		if !ok || projectIcon(p) == "" {
			http.NotFound(w, r)
			return
		}
		f, ctype, err := helpers.OpenCachedIcon(eng.Store().CacheDir(), path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			http.Error(w, "icon unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", ctype)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "private, max-age=60")
		http.ServeContent(w, r, projectIcon(p), info.ModTime(), f)
	}
}

func projectIcon(p engine.Project) string {
	if p.Meta == nil {
		return ""
	}
	icon, _ := p.Meta[helpers.FieldIcon].(string)
	return strings.TrimSpace(icon)
}

func findProject(projects []engine.Project, path string) (engine.Project, bool) {
	for _, p := range projects {
		if p.Path == path {
			return p, true
		}
	}
	return engine.Project{}, false
}
