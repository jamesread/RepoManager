package server

import (
	"context"

	"connectrpc.com/connect"
	repomanagerv1 "github.com/jamesread/RepoManager/service/gen/repomanager/v1"
	"github.com/jamesread/RepoManager/service/buildinfo"
)

func (s *Server) Init(ctx context.Context, req *connect.Request[repomanagerv1.InitRequest]) (*connect.Response[repomanagerv1.InitResponse], error) {
	au, _ := s.resolveSessionUser(ctx, req)
	features := s.featureFlags(ctx)
	res := &repomanagerv1.InitResponse{
		Version:        buildinfo.Version,
		SiteTitle:      s.cvars.GetString(ctx, "site_title", s.cfg.SiteTitle),
		Features:       features,
		CurrentVersion: buildinfo.Version,
	}
	if features.GetShowFooter() {
		if s.cvars.GetBool(ctx, "show_version_number", true) {
			res.CurrentVersion = buildinfo.Version
		} else {
			res.CurrentVersion = ""
		}
	}
	if au != nil && au.User != nil {
		res.CurrentUsername, res.RbacPermissions, res.RbacIsSuperuser = iamUserToProto(au)
	}
	return connect.NewResponse(res), nil
}

func (s *Server) featureFlags(ctx context.Context) *repomanagerv1.Features {
	return &repomanagerv1.Features{
		ThemeColorSchemeSwitcherEnabled: s.cvars.GetBool(ctx, "theme_color_scheme_switcher_enabled", true),
		ThemeName:                       s.cvars.GetString(ctx, "theme_name", ""),
		ThemeControl:                    s.cvars.GetString(ctx, "theme_control", "user"),
		AvailableThemes:                 []string{"default"},
		ShowFooter:                      s.cvars.GetBool(ctx, "show_footer", true),
		ShowNewVersions:                 false,
		ShowVersionNumber:               s.cvars.GetBool(ctx, "show_version_number", true),
	}
}
