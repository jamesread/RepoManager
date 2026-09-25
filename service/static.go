package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func findStaticDir() string {
	if dir := strings.TrimSpace(os.Getenv("PLIST_STATIC_DIR")); dir != "" {
		if abs, err := filepath.Abs(dir); err == nil {
			return abs
		}
		return dir
	}
	for _, c := range staticDirCandidates() {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			abs, err := filepath.Abs(c)
			if err != nil {
				return c
			}
			return abs
		}
	}
	return ""
}

func staticDirCandidates() []string {
	var out []string
	if exe, err := os.Executable(); err == nil {
		binDir := filepath.Dir(exe)
		if resolved, err := filepath.EvalSymlinks(binDir); err == nil {
			binDir = resolved
		}
		out = append(out,
			filepath.Join(binDir, "frontend", "dist"),
			filepath.Join(binDir, "..", "frontend", "dist"),
		)
	}
	out = append(out, "frontend/dist", "./frontend/dist", "../frontend/dist")
	return out
}

func spaFileServer(root string) http.Handler {
	fs := http.FileServer(http.Dir(root))
	indexPath := filepath.Join(root, "index.html")
	rootAbs, _ := filepath.Abs(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		target := spaTarget(root, rootAbs, r.URL.Path)
		if target == indexPath {
			http.ServeFile(w, r, indexPath)
			return
		}
		fs.ServeHTTP(w, r)
	})
}

func spaTarget(root, rootAbs, urlPath string) string {
	indexPath := filepath.Join(root, "index.html")
	path := filepath.Clean(urlPath)
	if path == "/" || path == "." {
		return indexPath
	}
	fpath := filepath.Join(root, path)
	if !isSafeRegularFile(fpath, rootAbs) {
		return indexPath
	}
	return fpath
}

func isSafeRegularFile(fpath, rootAbs string) bool {
	abs, err := filepath.Abs(fpath)
	if err != nil {
		return false
	}
	if !strings.HasPrefix(abs, rootAbs+string(os.PathSeparator)) && abs != rootAbs {
		return false
	}
	fi, err := os.Stat(abs)
	return err == nil && !fi.IsDir()
}
