package helpers

import "sync"

// GitHubPendingMsg notifies the UI that in-flight GitHub network queries changed.
type GitHubPendingMsg struct{}

// GitHubQueryState tracks GitHub API calls that are waiting on network responses.
// Cache hits do not increment the counter.
type GitHubQueryState struct {
	mu      sync.Mutex
	pending int
}

func NewGitHubQueryState() *GitHubQueryState {
	return &GitHubQueryState{}
}

func (g *GitHubQueryState) Begin() {
	if g == nil {
		return
	}
	g.mu.Lock()
	g.pending++
	g.mu.Unlock()
}

func (g *GitHubQueryState) Done() {
	if g == nil {
		return
	}
	g.mu.Lock()
	if g.pending > 0 {
		g.pending--
	}
	g.mu.Unlock()
}

func (g *GitHubQueryState) Count() int {
	if g == nil {
		return 0
	}
	g.mu.Lock()
	n := g.pending
	g.mu.Unlock()
	return n
}
