package server

import (
	"fmt"
	"sort"
	"time"

	"github.com/jamesread/RepoManager/core/engine"
	"github.com/jamesread/RepoManager/core/helpers"
	repomanagerv1 "github.com/jamesread/RepoManager/service/gen/repomanager/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

func projectsToProto(projects []engine.Project) []*repomanagerv1.Project {
	out := make([]*repomanagerv1.Project, 0, len(projects))
	for _, p := range projects {
		out = append(out, projectToProto(p))
	}
	return out
}

func projectToProto(p engine.Project) *repomanagerv1.Project {
	meta := metaToProtoStruct(p.Meta)
	ann := p.Annotations
	if ann == nil {
		ann = map[string]string{}
	}
	return &repomanagerv1.Project{
		Path:        p.Path,
		Rel:         p.Rel,
		Tags:        append([]string{}, p.Tags...),
		Meta:        meta,
		Annotations: ann,
	}
}

func metaToProtoStruct(meta map[string]any) *structpb.Struct {
	if len(meta) == 0 {
		return &structpb.Struct{}
	}
	converted := make(map[string]any, len(meta))
	for k, v := range meta {
		if proto, ok := metaValueToProto(v); ok {
			converted[k] = proto
		}
	}
	if len(converted) == 0 {
		return &structpb.Struct{}
	}
	out, err := structpb.NewStruct(converted)
	if err != nil {
		return &structpb.Struct{}
	}
	return out
}

func metaValueToProto(v any) (any, bool) {
	switch t := v.(type) {
	case nil:
		return nil, false
	case string:
		if t == "" {
			return nil, false
		}
		return t, true
	case bool:
		return t, true
	case int:
		return t, true
	case int32:
		return int64(t), true
	case int64:
		return t, true
	case float32:
		return float64(t), true
	case float64:
		return t, true
	case time.Time:
		if t.IsZero() {
			return nil, false
		}
		return t.UTC().Format(time.RFC3339Nano), true
	case []string:
		if len(t) == 0 {
			return nil, false
		}
		out := make([]any, len(t))
		for i, s := range t {
			out[i] = s
		}
		return out, true
	case []any:
		if len(t) == 0 {
			return nil, false
		}
		out := make([]any, 0, len(t))
		for _, item := range t {
			if converted, ok := metaValueToProto(item); ok {
				out = append(out, converted)
			}
		}
		if len(out) == 0 {
			return nil, false
		}
		return out, true
	default:
		if s := fmt.Sprint(v); s != "" {
			return s, true
		}
		return nil, false
	}
}

func catalogFields(eng *engine.Core) []*repomanagerv1.Field {
	fields := eng.Fields()
	out := make([]*repomanagerv1.Field, 0, len(fields)+len(eng.Store().AnnotationKeys()))
	for _, f := range fields {
		out = append(out, fieldToProto(f))
	}
	for _, key := range eng.Store().AnnotationKeys() {
		out = append(out, &repomanagerv1.Field{
			Id:         "annotation." + key,
			Label:      "@" + key,
			Kind:       string(helpers.FieldString),
			Sortable:   true,
			Filterable: true,
		})
	}
	return out
}

func fieldToProto(f helpers.Field) *repomanagerv1.Field {
	return &repomanagerv1.Field{
		Id:         f.ID,
		Label:      f.Label,
		Kind:       string(f.Kind),
		Sortable:   f.Sortable,
		Filterable: f.Filterable,
	}
}

func knownTags(eng *engine.Core) []string {
	tags := eng.Store().AllKnownTags()
	if tags == nil {
		return []string{}
	}
	sort.Strings(tags)
	return tags
}

func (s *Server) tagColors() map[string]string {
	colors := s.eng.Store().EnsureTagPastels(knownTags(s.eng))
	if colors == nil {
		return map[string]string{}
	}
	return colors
}

func (s *Server) listProjectsBody() *repomanagerv1.ListProjectsResponse {
	return &repomanagerv1.ListProjectsResponse{
		Root:        s.eng.Root(),
		Scanning:    s.eng.Scanning(),
		Found:       int32(s.eng.Found()),
		Fields:      catalogFields(s.eng),
		Projects:    projectsToProto(s.eng.Snapshot()),
		RecentRoots: s.eng.Store().RecentRoots(),
		KnownTags:   knownTags(s.eng),
		TagColors:   s.tagColors(),
	}
}
