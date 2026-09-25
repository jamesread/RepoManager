package helpers

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jamesread/RepoManager/core/jsonfile"
)

const FieldIcon = "icon"

// FieldIconRev changes when the cached logo bytes change, so clients can
// drop a cached image without treating the logo path as new metadata.
const FieldIconRev = "icon.rev"

const maxIconBytes int64 = 2 << 20

// logoExtOrder is the preference when several logo files share a directory.
var logoExtOrder = []string{".svg", ".png", ".webp", ".avif", ".jpg", ".jpeg", ".gif", ".ico"}

var iconContentTypes = map[string]string{
	".svg":  "image/svg+xml",
	".png":  "image/png",
	".webp": "image/webp",
	".avif": "image/avif",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".ico":  "image/x-icon",
}

const maxLogoDepth = 4

var skipIconDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "dist": true,
	"build": true, "target": true, ".cache": true, "coverage": true,
}

type iconCacheRecord struct {
	Rel     string `json:"rel"`
	Ext     string `json:"ext"`
	Size    int64  `json:"size"`
	ModUnix int64  `json:"mod_unix_nano"`
	Rev     string `json:"rev"`
}

// iconMu serializes cache reads and writes. Refresh and a background collect
// can touch the same project at once.
var iconMu sync.Mutex

// IconHelper copies a project logo into the cache so the web UI can serve it
// without reading the repo on every request.
type IconHelper struct {
	cacheDir string
}

func NewIconHelper(cacheDir string) *IconHelper {
	return &IconHelper{cacheDir: filepath.Join(cacheDir, "icons")}
}

func (h *IconHelper) Name() string { return "icon" }

func (h *IconHelper) Fields() []Field {
	return []Field{{
		ID:         FieldIcon,
		Label:      "Icon",
		Kind:       FieldString,
		Sortable:   false,
		Filterable: false,
	}}
}

func (h *IconHelper) Collect(ctx context.Context, projectPath string) (map[string]any, error) {
	return h.collect(ctx, projectPath, false)
}

func (h *IconHelper) CollectForced(ctx context.Context, projectPath string) (map[string]any, error) {
	return h.collect(ctx, projectPath, true)
}

func (h *IconHelper) collect(ctx context.Context, projectPath string, force bool) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	iconMu.Lock()
	defer iconMu.Unlock()
	abs, info, ok := findProjectLogo(projectPath)
	if !ok {
		h.removeCache(projectPath)
		return map[string]any{FieldIcon: "", FieldIconRev: ""}, nil
	}
	rel, err := filepath.Rel(projectPath, abs)
	if err != nil {
		return nil, err
	}
	rel = filepath.ToSlash(rel)
	rev, err := h.ensureCached(projectPath, abs, rel, info, force)
	if err != nil {
		return nil, err
	}
	return map[string]any{FieldIcon: rel, FieldIconRev: rev}, nil
}

func (h *IconHelper) ensureCached(projectPath, abs, rel string, info os.FileInfo, force bool) (string, error) {
	ext := strings.ToLower(filepath.Ext(rel))
	if !force {
		if rev, ok := h.cacheFresh(projectPath, rel, ext, info); ok {
			return rev, nil
		}
	}
	return h.writeCache(projectPath, abs, rel, ext, info)
}

func (h *IconHelper) cacheFresh(projectPath, rel, ext string, info os.FileInfo) (string, bool) {
	var rec iconCacheRecord
	if err := jsonfile.Read(h.recordPath(projectPath), &rec); err != nil {
		return "", false
	}
	if rec.Rev == "" || rec.Rel != rel || rec.Ext != ext || rec.Size != info.Size() || rec.ModUnix != info.ModTime().UnixNano() {
		return "", false
	}
	if _, err := os.Stat(h.dataPath(projectPath, ext)); err != nil {
		return "", false
	}
	return rec.Rev, true
}

func (h *IconHelper) writeCache(projectPath, abs, rel, ext string, info os.FileInfo) (string, error) {
	if err := os.MkdirAll(h.cacheDir, 0o755); err != nil {
		return "", err
	}
	h.dropPreviousExt(projectPath, ext)
	dst := h.dataPath(projectPath, ext)
	if err := copyRegularFile(abs, dst); err != nil {
		return "", err
	}
	rev, err := hashFile(dst)
	if err != nil {
		return "", err
	}
	rec := iconCacheRecord{
		Rel:     rel,
		Ext:     ext,
		Size:    info.Size(),
		ModUnix: info.ModTime().UnixNano(),
		Rev:     rev,
	}
	if err := jsonfile.Write(h.recordPath(projectPath), rec); err != nil {
		return "", err
	}
	return rev, nil
}

func (h *IconHelper) dropPreviousExt(projectPath, ext string) {
	var prev iconCacheRecord
	if err := jsonfile.Read(h.recordPath(projectPath), &prev); err != nil {
		return
	}
	if prev.Ext == "" || prev.Ext == ext {
		return
	}
	_ = os.Remove(h.dataPath(projectPath, prev.Ext))
}

func (h *IconHelper) removeCache(projectPath string) {
	var prev iconCacheRecord
	if err := jsonfile.Read(h.recordPath(projectPath), &prev); err == nil && prev.Ext != "" {
		_ = os.Remove(h.dataPath(projectPath, prev.Ext))
	}
	_ = os.Remove(h.recordPath(projectPath))
}

func (h *IconHelper) dataPath(projectPath, ext string) string {
	return filepath.Join(h.cacheDir, iconCacheID(projectPath)+ext)
}

func (h *IconHelper) recordPath(projectPath string) string {
	return filepath.Join(h.cacheDir, iconCacheID(projectPath)+".json")
}

func iconCacheID(projectPath string) string {
	sum := sha1.Sum([]byte(projectPath))
	return hex.EncodeToString(sum[:])
}

// OpenCachedIcon opens the cached logo for projectPath. cacheRoot is the
// store cache directory (the helper appends "icons").
func OpenCachedIcon(cacheRoot, projectPath string) (*os.File, string, error) {
	iconMu.Lock()
	defer iconMu.Unlock()
	h := NewIconHelper(cacheRoot)
	var rec iconCacheRecord
	if err := jsonfile.Read(h.recordPath(projectPath), &rec); err != nil {
		return nil, "", err
	}
	ctype, ok := iconContentTypes[rec.Ext]
	if !ok {
		return nil, "", os.ErrNotExist
	}
	f, err := os.Open(h.dataPath(projectPath, rec.Ext))
	if err != nil {
		return nil, "", err
	}
	return f, ctype, nil
}

func findProjectLogo(projectPath string) (string, os.FileInfo, bool) {
	bestScore := int(^uint(0) >> 1)
	var best string
	var bestInfo os.FileInfo
	_ = filepath.WalkDir(projectPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return skipWalkErr(d)
		}
		if d.IsDir() {
			return skipLogoDir(projectPath, path, d.Name())
		}
		score, info, ok := logoCandidate(projectPath, path, d)
		if !ok || score >= bestScore {
			return nil
		}
		bestScore = score
		best = path
		bestInfo = info
		return nil
	})
	return best, bestInfo, best != ""
}

func skipWalkErr(d fs.DirEntry) error {
	if d != nil && d.IsDir() {
		return fs.SkipDir
	}
	return nil
}

func skipLogoDir(projectPath, path, name string) error {
	if path != projectPath && skipIconDirs[name] {
		return fs.SkipDir
	}
	rel, err := filepath.Rel(projectPath, path)
	if err != nil || rel == "." {
		return nil
	}
	if strings.Count(rel, string(os.PathSeparator)) > maxLogoDepth {
		return fs.SkipDir
	}
	return nil
}

func logoCandidate(projectPath, path string, d fs.DirEntry) (int, os.FileInfo, bool) {
	if !d.Type().IsRegular() || logoRank(d.Name()) < 0 {
		return 0, nil, false
	}
	info, err := d.Info()
	if err != nil || info.Size() == 0 || info.Size() > maxIconBytes {
		return 0, nil, false
	}
	rel, err := filepath.Rel(projectPath, path)
	if err != nil {
		return 0, nil, false
	}
	depth := strings.Count(rel, string(os.PathSeparator))
	return depth*len(logoExtOrder) + logoRank(d.Name()), info, true
}

func logoRank(name string) int {
	lower := strings.ToLower(name)
	ext := filepath.Ext(lower)
	if lower != "logo"+ext {
		return -1
	}
	for i, want := range logoExtOrder {
		if ext == want {
			return i
		}
	}
	return -1
}

func copyRegularFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	n, copyErr := io.Copy(out, io.LimitReader(in, maxIconBytes+1))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil || n > maxIconBytes {
		_ = os.Remove(tmp)
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		return os.ErrInvalid
	}
	return os.Rename(tmp, dst)
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sum := sha1.New()
	if _, err := io.Copy(sum, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(sum.Sum(nil))[:16], nil
}
