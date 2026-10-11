package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

const (
	envPrefix          = "REPOMANAGER_"
	RequiredMigration  = "3.project-views.sql"
	defaultListenPort  = "8080"
	defaultSiteTitle   = "RepoManager"
	defaultDatabaseURL = "file:tmp/repomanager.db?_foreign_keys=on"
)

var configDir string

type Config struct {
	ListenPort     string
	SiteTitle      string
	DatabaseDriver string
	DatabaseURL    string
	ConfigDir      string
	Basedir        string
	Auth           map[string]any
}

func SetConfigDir(dir string) {
	configDir = dir
}

func GetConfigPath() string {
	if configDir == "" {
		return ""
	}
	return filepath.Join(configDir, "config.yaml")
}

func Load() (*Config, error) {
	configDirFlag := flag.String("configdir", "", "directory containing config.yaml")
	basedirFlag := flag.String("basedir", "", "scan root directory override")
	flag.Parse()

	dir := *configDirFlag
	if dir == "" {
		dir = configDir
	}
	if dir != "" {
		SetConfigDir(dir)
	}

	k := koanf.New(".")
	_ = k.Load(env.Provider(envPrefix, ".", func(s string) string {
		return strings.ToLower(strings.TrimPrefix(s, envPrefix))
	}), nil)

	if path := GetConfigPath(); path != "" {
		if _, err := os.Stat(path); err == nil {
			if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
				return nil, fmt.Errorf("load config: %w", err)
			}
		}
	}

	cfg := &Config{
		ListenPort: coalesce(
			os.Getenv("PORT"),
			k.String("listen_port"),
			defaultListenPort,
		),
		SiteTitle: coalesce(
			k.String("site_title"),
			defaultSiteTitle,
		),
		DatabaseDriver: coalesce(
			os.Getenv("REPOMANAGER_DATABASE_DRIVER"),
			k.String("database_driver"),
		),
		DatabaseURL: coalesce(
			os.Getenv("REPOMANAGER_DATABASE_URL"),
			k.String("database_url"),
			defaultDatabaseURL,
		),
		ConfigDir: dir,
		Basedir:   strings.TrimSpace(*basedirFlag),
	}
	if raw := k.Get("auth"); raw != nil {
		if m, ok := raw.(map[string]any); ok {
			cfg.Auth = m
		}
	}
	return cfg, nil
}

func ListenAddr(cfg *Config) string {
	port := strings.TrimSpace(cfg.ListenPort)
	if port == "" {
		port = defaultListenPort
	}
	if strings.Contains(port, ":") {
		return port
	}
	return ":" + port
}

func coalesce(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
