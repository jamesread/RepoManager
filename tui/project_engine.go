package main

import (
	"github.com/jamesread/RepoManager/core/engine"
	"github.com/jamesread/RepoManager/core/helpers"
)

func projectFromEngine(p engine.Project) projectItem {
	return projectItem{
		path:        p.Path,
		rel:         p.Rel,
		tags:        append([]string{}, p.Tags...),
		meta:        cloneMeta(p.Meta),
		annotations: cloneAnnotations(p.Annotations),
	}
}

func (i projectItem) toEngine() engine.Project {
	return engine.Project{
		Path:        i.path,
		Rel:         i.rel,
		Tags:        append([]string{}, i.tags...),
		Meta:        cloneMeta(i.meta),
		Annotations: cloneAnnotations(i.annotations),
	}
}

func cloneMeta(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func cloneAnnotations(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func (m *model) helperFields() []helpers.Field {
	if m.eng == nil {
		return nil
	}
	return m.eng.Fields()
}
