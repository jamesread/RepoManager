package engine

import "github.com/jamesread/RepoManager/core/store"

// Request is a UI → engine command. Concrete types below implement Kind().
type Request interface {
	Kind() string
}

const (
	KindScan             = "scan"
	KindRefresh          = "refresh"
	KindAddTag           = "add_tag"
	KindRemoveTag        = "remove_tag"
	KindSetTagOrder      = "set_tag_order"
	KindSetAnnotation    = "set_annotation"
	KindAddAnnotationKey = "add_annotation_key"
	KindSetLayout        = "set_layout"
	KindSetPathRelative  = "set_path_relative"
	KindSetPathTilde     = "set_path_tilde"
	KindSetTableMode     = "set_table_mode"
	KindSetSession       = "set_session"
	KindSaveView         = "save_view"
	KindSetRoot          = "set_root"
)

// ScanRequest starts (or restarts) a filesystem scan of the engine root.
type ScanRequest struct{}

func (ScanRequest) Kind() string { return KindScan }

// RefreshRequest force-refreshes helpers for the given project paths.
type RefreshRequest struct {
	Paths []string `json:"paths"`
}

func (RefreshRequest) Kind() string { return KindRefresh }

// AddTagRequest adds a tag to one or more projects.
type AddTagRequest struct {
	Paths []string `json:"paths"`
	Tag   string   `json:"tag"`
}

func (AddTagRequest) Kind() string { return KindAddTag }

// RemoveTagRequest removes a tag from one or more projects.
type RemoveTagRequest struct {
	Paths []string `json:"paths"`
	Tag   string   `json:"tag"`
}

func (RemoveTagRequest) Kind() string { return KindRemoveTag }

// SetTagOrderRequest replaces the global tag importance order.
type SetTagOrderRequest struct {
	Order []string `json:"order"`
}

func (SetTagOrderRequest) Kind() string { return KindSetTagOrder }

// SetAnnotationRequest sets (or clears, when Value is empty) an annotation.
type SetAnnotationRequest struct {
	Paths []string `json:"paths"`
	Key   string   `json:"key"`
	Value string   `json:"value"`
}

func (SetAnnotationRequest) Kind() string { return KindSetAnnotation }

// AddAnnotationKeyRequest registers a global annotation key.
type AddAnnotationKeyRequest struct {
	Key string `json:"key"`
}

func (AddAnnotationKeyRequest) Kind() string { return KindAddAnnotationKey }

// SetLayoutRequest persists the list layout name ("comfy" / "compact").
type SetLayoutRequest struct {
	Layout string `json:"layout"`
}

func (SetLayoutRequest) Kind() string { return KindSetLayout }

// SetPathRelativeRequest persists relative vs absolute path display.
type SetPathRelativeRequest struct {
	Relative bool `json:"relative"`
}

func (SetPathRelativeRequest) Kind() string { return KindSetPathRelative }

// SetPathTildeRequest persists whether $HOME is shown as ~.
type SetPathTildeRequest struct {
	Tilde bool `json:"tilde"`
}

func (SetPathTildeRequest) Kind() string { return KindSetPathTilde }

// SetTableModeRequest persists table vs list rendering.
type SetTableModeRequest struct {
	Enabled bool `json:"enabled"`
}

func (SetTableModeRequest) Kind() string { return KindSetTableMode }

// SetSessionRequest persists per-root UI session state.
type SetSessionRequest struct {
	Session store.RootSession `json:"session"`
}

func (SetSessionRequest) Kind() string { return KindSetSession }

// SaveViewRequest stores a named view for the engine root.
type SaveViewRequest struct {
	Name   string           `json:"name"`
	Config store.ViewConfig `json:"config"`
}

func (SaveViewRequest) Kind() string { return KindSaveView }

// SetRootRequest changes the filesystem directory being scanned.
type SetRootRequest struct {
	Root string `json:"root"`
}

func (SetRootRequest) Kind() string { return KindSetRoot }
