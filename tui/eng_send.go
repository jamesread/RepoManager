package main

import (
	"context"

	"github.com/jamesread/RepoManager/core/engine"
	"github.com/jamesread/RepoManager/core/helpers"
	"github.com/jamesread/RepoManager/core/store"
)

func (m *model) engSend(r engine.Request) error {
	if m.eng != nil {
		return m.eng.Send(context.Background(), r)
	}
	// Test fixtures without an engine fall back to the store directly.
	switch req := r.(type) {
	case engine.SetLayoutRequest:
		return m.store.SetLayout(req.Layout)
	case engine.SetPathRelativeRequest:
		return m.store.SetPathRelative(req.Relative)
	case engine.SetPathTildeRequest:
		return m.store.SetPathTilde(req.Tilde)
	case engine.SetTableModeRequest:
		return m.store.SetTableMode(req.Enabled)
	case engine.SetSessionRequest:
		return m.store.SetSession(m.root, req.Session)
	case engine.SaveViewRequest:
		return m.store.SaveView(m.root, req.Name, req.Config)
	case engine.SetTagOrderRequest:
		return m.store.SetTagOrder(req.Order)
	case engine.AddAnnotationKeyRequest:
		_, err := m.store.AddAnnotationKey(req.Key)
		return err
	case engine.AddTagRequest:
		if helpers.IsGitStatusTag(req.Tag) {
			return engine.ErrReservedTag
		}
		var err error
		for _, path := range req.Paths {
			if _, e := m.store.AddTag(path, req.Tag); e != nil {
				err = e
			}
		}
		return err
	case engine.RemoveTagRequest:
		if helpers.IsGitStatusTag(req.Tag) {
			return engine.ErrReservedTag
		}
		var err error
		for _, path := range req.Paths {
			if _, e := m.store.RemoveTag(path, req.Tag); e != nil {
				err = e
			}
		}
		return err
	case engine.SetAnnotationRequest:
		var err error
		for _, path := range req.Paths {
			if e := m.store.SetAnnotation(path, req.Key, req.Value); e != nil {
				err = e
			}
		}
		return err
	default:
		return nil
	}
}

func (m *model) persistSessionSnap(snap store.RootSession) {
	_ = m.engSend(engine.SetSessionRequest{Session: snap})
}
