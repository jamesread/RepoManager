package server

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"connectrpc.com/connect"
	armlayer "github.com/jamesread/armature-iam/layer"
	"github.com/jamesread/RepoManager/service/internal/iam"
)

func (s *Server) authUser(ctx context.Context) *armlayer.AuthenticatedUser {
	return armlayer.UserFromContext(ctx)
}

func sessionIDFromRequest(req connect.AnyRequest) string {
	if h := req.Header().Get("Session-Token"); h != "" {
		return h
	}
	if auth := req.Header().Get("Authorization"); auth != "" {
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" && parts[1] != "" && !strings.HasPrefix(parts[1], "sk_") {
			return parts[1]
		}
	}
	if cookieHeader := req.Header().Get("Cookie"); cookieHeader != "" {
		prefix := iam.CookieName + "="
		for _, part := range strings.Split(cookieHeader, ";") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, prefix) {
				return strings.TrimPrefix(part, prefix)
			}
		}
	}
	return ""
}

func (s *Server) resolveSessionUser(ctx context.Context, req connect.AnyRequest) (*armlayer.AuthenticatedUser, error) {
	sid := sessionIDFromRequest(req)
	if sid == "" {
		return nil, nil
	}
	sess, err := s.auth.Store.GetSessionBySID(ctx, sid)
	if err != nil || sess == nil {
		return nil, err
	}
	user, err := s.auth.Store.GetUserByID(ctx, sess.UserAccountID)
	if err != nil || user == nil {
		return nil, err
	}
	rb, err := s.auth.Store.LoadEffectiveRBAC(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	return &armlayer.AuthenticatedUser{User: user, RBAC: rb}, nil
}

func rbacPermissionNames(au *armlayer.AuthenticatedUser) []string {
	if au == nil || au.RBAC == nil {
		return nil
	}
	if au.RBAC.IsSuperuser {
		names := make([]string, 0, len(au.RBAC.Permissions))
		for n := range au.RBAC.Permissions {
			names = append(names, n)
		}
		sort.Strings(names)
		return names
	}
	names := make([]string, 0, len(au.RBAC.Permissions))
	for n, ok := range au.RBAC.Permissions {
		if ok {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names
}

func (s *Server) getUserIDFromContext(ctx context.Context) (int, error) {
	au := s.authUser(ctx)
	if au == nil || au.User == nil {
		return 0, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	return au.User.ID, nil
}

func sessionExpiresAt() int64 {
	return time.Now().Add(10 * 365 * 24 * time.Hour).Unix()
}

func iamUserToProto(u *armlayer.AuthenticatedUser) (username string, perms []string, superuser bool) {
	if u == nil || u.User == nil {
		return "", nil, false
	}
	return u.User.Username, rbacPermissionNames(u), u.RBAC != nil && u.RBAC.IsSuperuser
}

func int32SliceToInt(in []int32) []int {
	out := make([]int, 0, len(in))
	for _, v := range in {
		out = append(out, int(v))
	}
	return out
}
