package server

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"connectrpc.com/connect"
	repomanagerv1 "github.com/jamesread/RepoManager/service/gen/repomanager/v1"
	"github.com/jamesread/RepoManager/service/internal/store"
)

func (s *Server) ListCvars(ctx context.Context, _ *connect.Request[repomanagerv1.ListCvarsRequest]) (*connect.Response[repomanagerv1.ListCvarsResponse], error) {
	rows, err := s.cvars.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*repomanagerv1.Cvar, 0, len(rows))
	for _, row := range rows {
		out = append(out, cvarRowToProto(row))
	}
	return connect.NewResponse(&repomanagerv1.ListCvarsResponse{Cvars: out}), nil
}

func (s *Server) UpdateCvar(ctx context.Context, req *connect.Request[repomanagerv1.UpdateCvarRequest]) (*connect.Response[repomanagerv1.UpdateCvarResponse], error) {
	key := strings.TrimSpace(req.Msg.GetKey())
	if key == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("key is required"))
	}
	if err := s.cvars.Update(ctx, key, int(req.Msg.GetValueInt()), req.Msg.GetValueString()); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("cvar not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	rows, err := s.cvars.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for _, row := range rows {
		if row.Key == key {
			return connect.NewResponse(&repomanagerv1.UpdateCvarResponse{Cvar: cvarRowToProto(row)}), nil
		}
	}
	return nil, connect.NewError(connect.CodeNotFound, errors.New("cvar not found"))
}

func cvarRowToProto(row store.CvarRow) *repomanagerv1.Cvar {
	cv := &repomanagerv1.Cvar{
		Key:         row.Key,
		MainType:    row.MainType,
		Title:       row.Title,
		Description: row.Description,
		Category:    row.Category,
		Ordinal:     int32(row.Ordinal),
	}
	if row.ValueInt.Valid {
		cv.ValueInt = int32(row.ValueInt.Int64)
	}
	if row.ValueString.Valid {
		cv.ValueString = row.ValueString.String
	}
	return cv
}
