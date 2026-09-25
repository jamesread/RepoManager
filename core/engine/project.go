package engine

// Project is the UI-agnostic project DTO. TUI wraps it for list.Item;
// a web UI can serialize it as JSON as-is.
type Project struct {
	Path        string            `json:"path"`
	Rel         string            `json:"rel"`
	Tags        []string          `json:"tags,omitempty"`
	Meta        map[string]any    `json:"meta,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

func (p Project) clone() Project {
	out := Project{Path: p.Path, Rel: p.Rel}
	if p.Tags != nil {
		out.Tags = append([]string{}, p.Tags...)
	}
	if p.Meta != nil {
		out.Meta = make(map[string]any, len(p.Meta))
		for k, v := range p.Meta {
			out.Meta[k] = v
		}
	}
	if p.Annotations != nil {
		out.Annotations = make(map[string]string, len(p.Annotations))
		for k, v := range p.Annotations {
			out.Annotations[k] = v
		}
	}
	return out
}
