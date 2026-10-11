package iam

import (
	"net/http"
	"os"
	"strings"

	armlayer "github.com/jamesread/armature-iam/layer"
	"github.com/jamesread/armature-iam/store"
	"github.com/jamesread/RepoManager/service/gen/repomanager/v1/repomanagerv1connect"
	log "github.com/sirupsen/logrus"
)

const CookieName = "repomanager-sid"

// AuthLayer wraps armature-iam Connect auth for RepoManager.
type AuthLayer struct {
	*armlayer.Layer
	Store store.Store
}

// NewAuthLayer wires armature-iam Connect auth.
func NewAuthLayer(st store.Store, authYAML map[string]any) (*AuthLayer, error) {
	cfg := armlayer.Config{
		CookieName: CookieName,
		AuthYAML:   authYAML,
		Logger:     log.StandardLogger(),
		AllowUnauthenticated: []string{
			repomanagerv1connect.RepoManagerServiceInitProcedure,
			repomanagerv1connect.RepoManagerServiceLoginProcedure,
			repomanagerv1connect.RepoManagerServiceLogoutProcedure,
			repomanagerv1connect.RepoManagerServiceValidateTokenProcedure,
		},
		RequiredPermission: RequiredPermission,
		DevDisableAuth:     os.Getenv("REPOMANAGER_DEV_DISABLE_AUTH") == "true",
		SecureCookies:      os.Getenv("REPOMANAGER_SECURE_COOKIES") != "false",
	}
	inner, err := armlayer.New(st, cfg)
	if err != nil {
		return nil, err
	}
	return &AuthLayer{Layer: inner, Store: st}, nil
}

// WrapHandler injects Session-Token / non-API-key Bearer into the session cookie
// so armature-iam accepts the SPA token on local HTTP when cookies are absent.
func (a *AuthLayer) WrapHandler(in http.Handler) http.Handler {
	authed := a.Layer.WrapHandler(in)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authed.ServeHTTP(w, injectSessionCookieFromHeaders(r))
	})
}

func injectSessionCookieFromHeaders(r *http.Request) *http.Request {
	if cookie, err := r.Cookie(CookieName); err == nil && cookie.Value != "" {
		return r
	}
	sid := strings.TrimSpace(r.Header.Get("Session-Token"))
	if sid == "" {
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
			if token != "" && !strings.HasPrefix(token, "sk_") {
				sid = token
			}
		}
	}
	if sid == "" {
		return r
	}
	clone := r.Clone(r.Context())
	clone.AddCookie(&http.Cookie{Name: CookieName, Value: sid})
	return clone
}
