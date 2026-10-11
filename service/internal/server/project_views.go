package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"connectrpc.com/connect"
	repomanagerv1 "github.com/jamesread/RepoManager/service/gen/repomanager/v1"
	"github.com/jamesread/RepoManager/service/internal/store"
)

const maxProjectViewStateBytes = 256 * 1024

func (s *Server) ListProjectViews(ctx context.Context, _ *connect.Request[repomanagerv1.ListProjectViewsRequest]) (*connect.Response[repomanagerv1.ListProjectViewsResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	rows, defaultID, err := s.projectViews.ListByUser(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*repomanagerv1.ProjectView, 0, len(rows))
	for _, row := range rows {
		out = append(out, projectViewToProto(row))
	}
	return connect.NewResponse(&repomanagerv1.ListProjectViewsResponse{
		Views:         out,
		DefaultViewId: int32(defaultID),
	}), nil
}

func (s *Server) SaveProjectView(ctx context.Context, req *connect.Request[repomanagerv1.SaveProjectViewRequest]) (*connect.Response[repomanagerv1.SaveProjectViewResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Msg.GetName())
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	stateJSON, err := validateProjectViewStateJSON(req.Msg.GetStateJson())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	row, defaultID, err := s.projectViews.Create(ctx, userID, name, stateJSON, req.Msg.GetSetAsDefault())
	if err != nil {
		return nil, projectViewStoreError(err)
	}
	return connect.NewResponse(&repomanagerv1.SaveProjectViewResponse{
		View:          projectViewToProto(row),
		DefaultViewId: int32(defaultID),
	}), nil
}

func (s *Server) DeleteProjectView(ctx context.Context, req *connect.Request[repomanagerv1.DeleteProjectViewRequest]) (*connect.Response[repomanagerv1.DeleteProjectViewResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	defaultID, err := s.projectViews.Delete(ctx, userID, int(req.Msg.GetViewId()))
	if err != nil {
		return nil, projectViewStoreError(err)
	}
	return connect.NewResponse(&repomanagerv1.DeleteProjectViewResponse{
		DefaultViewId: int32(defaultID),
	}), nil
}

func (s *Server) SetDefaultProjectView(ctx context.Context, req *connect.Request[repomanagerv1.SetDefaultProjectViewRequest]) (*connect.Response[repomanagerv1.SetDefaultProjectViewResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	defaultID, err := s.projectViews.SetDefault(ctx, userID, int(req.Msg.GetViewId()))
	if err != nil {
		return nil, projectViewStoreError(err)
	}
	return connect.NewResponse(&repomanagerv1.SetDefaultProjectViewResponse{
		DefaultViewId: int32(defaultID),
	}), nil
}

func validateProjectViewStateJSON(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("state is required")
	}
	if len(raw) > maxProjectViewStateBytes {
		return "", errors.New("state is too large")
	}
	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return "", errors.New("state must be valid JSON")
	}
	return raw, nil
}

func projectViewToProto(row store.ProjectViewRow) *repomanagerv1.ProjectView {
	return &repomanagerv1.ProjectView{
		Id:        int32(row.ID),
		Name:      row.Name,
		StateJson: row.StateJSON,
		IsDefault: row.IsDefault,
		CreatedAt: row.CreatedAt.Unix(),
		UpdatedAt: row.UpdatedAt.Unix(),
	}
}

func projectViewStoreError(err error) error {
	switch {
	case errors.Is(err, store.ErrProjectViewNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, store.ErrProjectViewNameUsed):
		return connect.NewError(connect.CodeAlreadyExists, err)
	case errors.Is(err, store.ErrProjectViewLimit):
		return connect.NewError(connect.CodeResourceExhausted, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}
