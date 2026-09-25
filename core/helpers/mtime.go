package helpers

import (
	"context"
	"os"
)

const FieldDirMtime = "dir.mtime"

type MtimeHelper struct{}

func (MtimeHelper) Name() string { return "mtime" }

func (MtimeHelper) Fields() []Field {
	return []Field{{
		ID:         FieldDirMtime,
		Label:      "Dir mtime",
		Kind:       FieldTime,
		Sortable:   true,
		Filterable: true,
	}}
}

func (MtimeHelper) Collect(ctx context.Context, projectPath string) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info, err := os.Stat(projectPath)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		FieldDirMtime: info.ModTime().UTC(),
	}, nil
}
