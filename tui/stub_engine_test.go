package main

import (
	"context"
	"sync"

	"github.com/jamesread/RepoManager/core/engine"
	"github.com/jamesread/RepoManager/core/helpers"
	"github.com/jamesread/RepoManager/core/store"
)

// stubEngine is a minimal Engine for UI unit tests that do not need scanning.
type stubEngine struct {
	store     *store.Store
	root      string
	ghPending int

	once   sync.Once
	fields []helpers.Field
	events chan engine.Event
}

func newStubEngine(st *store.Store, root string) *stubEngine {
	return &stubEngine{
		store:  st,
		root:   root,
		events: make(chan engine.Event, 8),
	}
}

func (s *stubEngine) Subscribe() <-chan engine.Event { return s.events }
func (s *stubEngine) Send(_ context.Context, r engine.Request) error {
	return (&model{store: s.store, root: s.root}).engSend(r)
}
func (s *stubEngine) Snapshot() []engine.Project { return nil }
func (s *stubEngine) Fields() []helpers.Field {
	s.once.Do(func() {
		ch := make(chan any, 1)
		s.fields = helpers.NewEngine(context.Background(), ch, helpers.Default(s.store.CacheDir(), s.store.GitHubCacheTTL(), nil, nil)).Fields()
	})
	return s.fields
}
func (s *stubEngine) HelperCount() int        { return 5 }
func (s *stubEngine) GitHubPendingCount() int { return s.ghPending }
func (s *stubEngine) Store() *store.Store     { return s.store }
func (s *stubEngine) Root() string            { return s.root }
func (s *stubEngine) Scanning() bool          { return false }
func (s *stubEngine) Found() int              { return 0 }
func (s *stubEngine) Start()                  {}
func (s *stubEngine) Close() {
	select {
	case <-s.events:
	default:
		close(s.events)
	}
}
