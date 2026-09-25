// Package engine is the UI-agnostic core of RepoManager: it owns the project list,
// runs scans and helper enrichment, and persists tags, annotations, sessions
// and views via store.
//
// UIs talk to an Engine through two channels:
//
//   - Requests (UI → engine): Send(ctx, Request) mutates state or starts work.
//   - Events (engine → UI): Subscribe() delivers ProjectFound, ProjectUpdated,
//     ScanFinished, GitHubPending, and related notifications.
//
// Sort, layout, table column fitting and bulk selection stay UI-local; they
// recompute from a local mirror of Snapshot() updated by Events. Session and
// display prefs are persisted through Requests so every UI shares them.
//
// Request and Event types carry JSON tags so a future web transport can
// encode the same values over WebSocket or SSE without changing this API:
// marshal Request/Event to JSON on the wire, demux by Kind(), and call
// Send / forward Events to the browser.
package engine

import (
	"context"

	"github.com/jamesread/RepoManager/core/helpers"
	"github.com/jamesread/RepoManager/core/store"
)

// Engine is the contract every UI (TUI, web) uses.
type Engine interface {
	// Subscribe returns a fan-out channel of Events. The caller must drain
	// it; Close closes all subscriber channels.
	Subscribe() <-chan Event
	// Send delivers a Request. Scan and Refresh return once work is
	// scheduled; mutations apply synchronously and emit events before returning.
	Send(ctx context.Context, r Request) error
	// Snapshot returns a copy of the current project list.
	Snapshot() []Project
	// Fields returns the helper field catalog for sort/filter menus.
	Fields() []helpers.Field
	// HelperCount is how many helpers run per Enrich/Refresh.
	HelperCount() int
	// GitHubPendingCount is outstanding GitHub network queries.
	GitHubPendingCount() int
	// Store is the backing store. UIs may read display data (tag colours,
	// prefs); mutations must go through Send.
	Store() *store.Store
	// Root is the scan directory.
	Root() string
	// Scanning reports whether a scan is in progress.
	Scanning() bool
	// Found is the number of projects currently known.
	Found() int
	// Start begins the initial filesystem scan. Call after Subscribe so
	// early events are not missed. Idempotent with Send(ScanRequest{}).
	Start()
	// Close cancels background work, waits for in-flight scan/helpers to
	// finish, then closes subscriber channels.
	Close()
}
