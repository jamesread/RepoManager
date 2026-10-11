// Command plist-wui serves the RepoManager web UI and Connect API.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	armpassword "github.com/jamesread/armature-iam/password"
	armstore "github.com/jamesread/armature-iam/store"
	"github.com/jamesread/RepoManager/core/cli"
	"github.com/jamesread/RepoManager/core/engine"
	"github.com/jamesread/RepoManager/core/store"
	"github.com/jamesread/RepoManager/service/buildinfo"
	"github.com/jamesread/RepoManager/service/internal/config"
	"github.com/jamesread/RepoManager/service/internal/httpserver"
	"github.com/jamesread/RepoManager/service/internal/iam"
	"github.com/jamesread/RepoManager/service/internal/iamstore"
	"github.com/jamesread/RepoManager/service/internal/server"
	appstore "github.com/jamesread/RepoManager/service/internal/store"
	"github.com/sirupsen/logrus"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "repomanager-wui: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if path := config.GetConfigPath(); path != "" {
		logrus.Infof("Config loaded from %s", path)
	}

	dsn := config.ResolveDatabaseURL(cfg)
	driver := config.ResolveDatabaseDriver(cfg, dsn)
	database, err := appstore.Open(driver, dsn)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer func() { _ = database.DB.Close() }()

	ctx := context.Background()
	if err := appstore.AssertMigration(ctx, database.DB, config.RequiredMigration); err != nil {
		return err
	}

	iamStore := iamstore.New(database.Driver, database.DB)
	if err := iamStore.EnsureRBACBootstrap(ctx); err != nil {
		return fmt.Errorf("iam bootstrap: %w", err)
	}
	if err := bootstrapAdmin(ctx, iamStore); err != nil {
		return fmt.Errorf("admin bootstrap: %w", err)
	}

	cvarStore := appstore.NewCvarStore(database.DB, database.Driver)
	if err := cvarStore.EnsureDefaults(ctx, cfg.SiteTitle); err != nil {
		return fmt.Errorf("cvars: %w", err)
	}
	projectViewStore := appstore.NewProjectViewStore(database.DB, database.Driver)

	st, err := store.Load()
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}
	root, err := resolveServiceRoot(st, cfg.Basedir)
	if err != nil {
		return err
	}

	eng := engine.New(st, root)
	defer eng.Close()
	_ = eng.Subscribe()
	eng.Start()

	authLayer, err := iam.NewAuthLayer(iamStore, cfg.Auth)
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	srv := server.New(cfg, eng, authLayer, cvarStore, projectViewStore, database)
	mux := httpserver.NewMux(srv, authLayer, eng)

	addr := config.ListenAddr(cfg)
	logrus.Infof("RepoManager %s listening on %s (root %s, db %s)", buildinfo.Version, addr, root, database.Driver)
	return http.ListenAndServe(addr, mux)
}

func resolveServiceRoot(st *store.Store, basedir string) (string, error) {
	if strings.TrimSpace(basedir) == "" {
		if saved, ok := st.StartupRoot(); ok {
			return saved, st.RememberRoot(saved)
		}
	}
	root, err := cli.ResolveRoot(basedir)
	if err != nil {
		return "", err
	}
	if err := st.RememberRoot(root); err != nil {
		return "", err
	}
	return root, nil
}

func bootstrapAdmin(ctx context.Context, iamStore armstore.Store) error {
	count, err := iamStore.CountUserAccounts(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := armpassword.Hash("admin")
	if err != nil {
		return err
	}
	id, err := iamStore.CreateUserAccount(ctx, "admin", hash, armstore.UserCreatedByAdmin)
	if err != nil {
		return err
	}
	if err := iamStore.EnsureUserInGroup(ctx, id, "Administrators"); err != nil {
		return err
	}
	if err := iamStore.EnsureUserInEveryoneGroup(ctx, id); err != nil {
		return err
	}
	logrus.Warn("Created default admin user (username: admin, password: admin)")
	return nil
}
