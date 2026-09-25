package store

// FilterRule is the JSON form of an advanced filter rule.
type FilterRule struct {
	FieldID string   `json:"field_id"`
	Value   string   `json:"value,omitempty"`
	Values  []string `json:"values,omitempty"`
	Negated []string `json:"negated,omitempty"`
	Unset   bool     `json:"unset,omitempty"`
}

// RootSession stores list UI state for a scan root between runs.
type RootSession struct {
	SelectedPath string       `json:"selected_path,omitempty"`
	ListIndex    int          `json:"list_index,omitempty"`
	QuickFilter  string       `json:"quick_filter,omitempty"`
	SortFieldID  string       `json:"sort_field_id,omitempty"`
	SortFieldIDs []string     `json:"sort_field_ids,omitempty"`
	SortSpecs    []SortSpec   `json:"sort_specs,omitempty"`
	AdvFilter    []FilterRule `json:"adv_filter,omitempty"`
	ActiveViewID string       `json:"active_view_id,omitempty"`
}

// ViewConfig captures layout, sort, and filter settings for a saved view.
type ViewConfig struct {
	Layout       string       `json:"layout"`
	PathRelative bool         `json:"path_relative"`
	PathTilde    bool         `json:"path_tilde"`
	SortFieldID  string       `json:"sort_field_id,omitempty"`
	SortFieldIDs []string     `json:"sort_field_ids,omitempty"`
	SortSpecs    []SortSpec   `json:"sort_specs,omitempty"`
	QuickFilter  string       `json:"quick_filter,omitempty"`
	AdvFilter    []FilterRule `json:"adv_filter,omitempty"`
}

// SavedView is a user-named view stored per scan root.
type SavedView struct {
	Name   string     `json:"name"`
	Config ViewConfig `json:"config"`
}

// SortSpec is one sort key: a field ID and a direction.
type SortSpec struct {
	ID   string `json:"id"`
	Desc bool   `json:"desc,omitempty"`
}
