package engine

// Event is an engine → UI notification. Concrete types below implement Kind().
type Event interface {
	Kind() string
}

const (
	KindProjectFound   = "project_found"
	KindProjectUpdated = "project_updated"
	KindProjectRemoved = "project_removed"
	KindScanStarted    = "scan_started"
	KindScanFinished   = "scan_finished"
	KindGitHubPending  = "github_pending"
	KindPrefsChanged   = "prefs_changed"
	KindRootChanged    = "root_changed"
	KindError          = "error"
)

// ProjectFound is emitted when a project is discovered during a scan
// (or loaded from cache).
type ProjectFound struct {
	Project Project `json:"project"`
}

func (ProjectFound) Kind() string { return KindProjectFound }

// ProjectUpdated is emitted when a project's tags, meta or annotations change.
type ProjectUpdated struct {
	Project Project `json:"project"`
}

func (ProjectUpdated) Kind() string { return KindProjectUpdated }

// ProjectRemoved is emitted when a project disappears from disk.
type ProjectRemoved struct {
	Path string `json:"path"`
}

func (ProjectRemoved) Kind() string { return KindProjectRemoved }

// ScanStarted marks the beginning of a filesystem scan.
type ScanStarted struct{}

func (ScanStarted) Kind() string { return KindScanStarted }

// ScanFinished marks the end of a scan. Err is empty on success.
type ScanFinished struct {
	Err string `json:"err,omitempty"`
}

func (ScanFinished) Kind() string { return KindScanFinished }

// GitHubPending reports how many GitHub network queries are outstanding.
type GitHubPending struct {
	Count int `json:"count"`
}

func (GitHubPending) Kind() string { return KindGitHubPending }

// PrefsChanged is emitted after layout/path/table settings change so UIs
// can refresh display without re-reading the store on a timer.
type PrefsChanged struct{}

func (PrefsChanged) Kind() string { return KindPrefsChanged }

// RootChanged is emitted after the scan root directory changes.
type RootChanged struct {
	Root string `json:"root"`
}

func (RootChanged) Kind() string { return KindRootChanged }

// ErrorEvent reports a failed request or background failure.
type ErrorEvent struct {
	Message string `json:"message"`
}

func (ErrorEvent) Kind() string { return KindError }
