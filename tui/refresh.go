package main

import "sync"

// refreshState tracks projects with an in-flight manual refresh (F5). The
// list delegate reads it to draw an indicator next to the project name; the
// model updates it as helper results arrive. It is shared by pointer between
// the model and the delegate (which lives inside list.Model), and guarded by
// a mutex only because delegates render on the UI goroutine while the model
// is a value type that is copied around by Bubble Tea.
type refreshState struct {
	mu      sync.Mutex
	pending map[string]int // path -> outstanding helper results
}

// refreshGlyph is drawn beside a project name while its refresh is running.
const refreshGlyph = "↻"

func newRefreshState() *refreshState {
	return &refreshState{pending: map[string]int{}}
}

// begin marks path as refreshing, expecting n helper results.
func (r *refreshState) begin(path string, n int) {
	if r == nil || path == "" || n <= 0 {
		return
	}
	r.mu.Lock()
	r.pending[path] += n
	r.mu.Unlock()
}

// done records one helper result for path; returns true when the refresh
// just finished (all expected results arrived).
func (r *refreshState) done(path string) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	n, ok := r.pending[path]
	if !ok {
		return false
	}
	n--
	if n <= 0 {
		delete(r.pending, path)
		return true
	}
	r.pending[path] = n
	return false
}

func (r *refreshState) active(path string) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.pending[path]
	return ok
}

func (r *refreshState) count() int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.pending)
}
