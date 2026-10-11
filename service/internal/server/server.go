package server

import (
	"github.com/jamesread/RepoManager/core/engine"
	"github.com/jamesread/RepoManager/service/gen/repomanager/v1/repomanagerv1connect"
	"github.com/jamesread/RepoManager/service/internal/config"
	"github.com/jamesread/RepoManager/service/internal/iam"
	"github.com/jamesread/RepoManager/service/internal/store"
)

// Server implements RepoManager Connect RPCs.
type Server struct {
	repomanagerv1connect.UnimplementedRepoManagerServiceHandler
	cfg   *config.Config
	eng   *engine.Core
	auth  *iam.AuthLayer
	cvars        *store.CvarStore
	projectViews *store.ProjectViewStore
	db           *store.Database
}

func New(cfg *config.Config, eng *engine.Core, auth *iam.AuthLayer, cvars *store.CvarStore, projectViews *store.ProjectViewStore, db *store.Database) *Server {
	return &Server{cfg: cfg, eng: eng, auth: auth, cvars: cvars, projectViews: projectViews, db: db}
}
