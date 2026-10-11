package server

import (
	"context"

	"connectrpc.com/connect"
	repomanagerv1 "github.com/jamesread/RepoManager/service/gen/repomanager/v1"
	"github.com/jamesread/RepoManager/service/internal/config"
	"github.com/jamesread/RepoManager/service/internal/store"
)

func (s *Server) GetDatabaseStatus(ctx context.Context, _ *connect.Request[repomanagerv1.DatabaseStatusRequest]) (*connect.Response[repomanagerv1.DatabaseStatusResponse], error) {
	st := store.BuildStatus(ctx, s.db, config.RequiredMigration)
	return connect.NewResponse(databaseStatusToProto(st)), nil
}

func databaseStatusToProto(st store.Status) *repomanagerv1.DatabaseStatusResponse {
	return &repomanagerv1.DatabaseStatusResponse{
		Driver:            st.Driver,
		DsnRedacted:       st.DSNRedacted,
		Host:              st.Host,
		Database:          st.Database,
		Connected:         st.Connected,
		ConnectionError:   st.ConnectionError,
		RequiredMigration: st.RequiredMigration,
		LatestMigration:   st.LatestMigration,
		MigrationsOk:      st.MigrationsOK,
		AppliedMigrations: append([]string{}, st.AppliedMigrations...),
	}
}
