package main

import (
	"testing"

	"github.com/jamesread/RepoManager/core/engine"
	"github.com/jamesread/RepoManager/core/store"
)

// attachTestEngine wires a real in-process engine (cache loaded, scan not
// started) so UI tests can resolve fields and Send mutations.
func attachTestEngine(t *testing.T, m *model, st *store.Store, root string) {
	t.Helper()
	if root == "" {
		root = t.TempDir()
	}
	if st == nil {
		st = m.store
	}
	eng := engine.New(st, root)
	t.Cleanup(eng.Close)
	m.eng = eng
	m.store = st
	m.root = root
	m.events = eng.Subscribe()
	for _, p := range eng.Snapshot() {
		m.upsertProjectData(projectFromEngine(p))
	}
	m.found = len(m.all)
}
