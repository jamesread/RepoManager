package server

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"github.com/jamesread/RepoManager/core/engine"
	"github.com/jamesread/RepoManager/core/helpers"
	repomanagerv1 "github.com/jamesread/RepoManager/service/gen/repomanager/v1"
)

func (s *Server) ListProjects(ctx context.Context, _ *connect.Request[repomanagerv1.ListProjectsRequest]) (*connect.Response[repomanagerv1.ListProjectsResponse], error) {
	return connect.NewResponse(s.listProjectsBody()), nil
}

func (s *Server) GetProject(ctx context.Context, req *connect.Request[repomanagerv1.GetProjectRequest]) (*connect.Response[repomanagerv1.GetProjectResponse], error) {
	path := strings.TrimSpace(req.Msg.GetPath())
	if path == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("path is required"))
	}
	p, ok := findProject(s.eng.Snapshot(), path)
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("project not found"))
	}
	return connect.NewResponse(&repomanagerv1.GetProjectResponse{
		Root:      s.eng.Root(),
		Fields:    catalogFields(s.eng),
		Project:   projectToProto(p),
		KnownTags: knownTags(s.eng),
		TagColors: s.tagColors(),
	}), nil
}

func (s *Server) RefreshProjects(ctx context.Context, req *connect.Request[repomanagerv1.RefreshProjectsRequest]) (*connect.Response[repomanagerv1.ListProjectsResponse], error) {
	paths := refreshPaths(req.Msg.GetPaths())
	if len(paths) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("paths are required"))
	}
	if !projectsContain(s.eng.Snapshot(), paths) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("project not found"))
	}
	if err := s.eng.Send(ctx, engine.RefreshRequest{Paths: paths}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(s.listProjectsBody()), nil
}

func (s *Server) DeleteProject(ctx context.Context, req *connect.Request[repomanagerv1.DeleteProjectRequest]) (*connect.Response[repomanagerv1.ListProjectsResponse], error) {
	path := strings.TrimSpace(req.Msg.GetPath())
	if path == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("path is required"))
	}
	if err := s.eng.DeleteLocalProject(ctx, path); err != nil {
		switch {
		case errors.Is(err, engine.ErrProjectNotFound):
			return nil, connect.NewError(connect.CodeNotFound, err)
		case errors.Is(err, engine.ErrLocalChanges):
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		default:
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
	}
	return connect.NewResponse(s.listProjectsBody()), nil
}

func (s *Server) SetRoot(ctx context.Context, req *connect.Request[repomanagerv1.SetRootRequest]) (*connect.Response[repomanagerv1.ListProjectsResponse], error) {
	root := strings.TrimSpace(req.Msg.GetRoot())
	if root == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("root is required"))
	}
	if err := s.eng.Send(ctx, engine.SetRootRequest{Root: root}); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	return connect.NewResponse(s.listProjectsBody()), nil
}

func (s *Server) Rescan(ctx context.Context, _ *connect.Request[repomanagerv1.RescanRequest]) (*connect.Response[repomanagerv1.ListProjectsResponse], error) {
	if err := s.eng.Rescan(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(s.listProjectsBody()), nil
}

func (s *Server) MutateTags(ctx context.Context, req *connect.Request[repomanagerv1.MutateTagsRequest]) (*connect.Response[repomanagerv1.ListProjectsResponse], error) {
	mutation, code, msg := tagMutation(s.eng.Snapshot(), req.Msg.GetPaths(), req.Msg.GetTag(), req.Msg.GetOp())
	if code != 0 {
		return nil, connect.NewError(httpToConnectCode(code), errors.New(msg))
	}
	if err := s.eng.Send(ctx, mutation); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(s.listProjectsBody()), nil
}

func (s *Server) SetAnnotation(ctx context.Context, req *connect.Request[repomanagerv1.SetAnnotationRequest]) (*connect.Response[repomanagerv1.GetProjectResponse], error) {
	path := strings.TrimSpace(req.Msg.GetPath())
	key := strings.TrimSpace(req.Msg.GetKey())
	if path == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("path is required"))
	}
	if key == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("key is required"))
	}
	if _, ok := findProject(s.eng.Snapshot(), path); !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("project not found"))
	}
	if err := s.eng.Send(ctx, engine.SetAnnotationRequest{
		Paths: []string{path},
		Key:   key,
		Value: req.Msg.GetValue(),
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	p, ok := findProject(s.eng.Snapshot(), path)
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("project not found"))
	}
	return connect.NewResponse(&repomanagerv1.GetProjectResponse{
		Root:      s.eng.Root(),
		Fields:    catalogFields(s.eng),
		Project:   projectToProto(p),
		KnownTags: knownTags(s.eng),
		TagColors: s.tagColors(),
	}), nil
}

func refreshPaths(paths []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func tagMutation(projects []engine.Project, paths []string, tag, op string) (engine.Request, int, string) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil, http.StatusBadRequest, "tag is required"
	}
	if helpers.IsGitStatusTag(tag) {
		return nil, http.StatusBadRequest, "tag is set by a helper"
	}
	if len(paths) == 0 {
		return nil, http.StatusBadRequest, "paths are required"
	}
	if !projectsContain(projects, paths) {
		return nil, http.StatusNotFound, "project not found"
	}
	switch op {
	case "add":
		return engine.AddTagRequest{Paths: paths, Tag: tag}, 0, ""
	case "remove":
		return engine.RemoveTagRequest{Paths: paths, Tag: tag}, 0, ""
	default:
		return nil, http.StatusBadRequest, "op must be add or remove"
	}
}

func projectsContain(projects []engine.Project, paths []string) bool {
	known := make(map[string]struct{}, len(projects))
	for _, p := range projects {
		known[p.Path] = struct{}{}
	}
	for _, path := range paths {
		if _, ok := known[path]; !ok {
			return false
		}
	}
	return true
}

func findProject(projects []engine.Project, path string) (engine.Project, bool) {
	for _, p := range projects {
		if p.Path == path {
			return p, true
		}
	}
	return engine.Project{}, false
}

func httpToConnectCode(status int) connect.Code {
	switch status {
	case http.StatusBadRequest:
		return connect.CodeInvalidArgument
	case http.StatusNotFound:
		return connect.CodeNotFound
	case http.StatusConflict:
		return connect.CodeFailedPrecondition
	default:
		return connect.CodeInternal
	}
}
