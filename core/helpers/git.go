package helpers

import (
	"bytes"
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const FieldGitLastCommit = "git.last_commit"

const (
	GitTagChanges = "changes"
	GitTagAhead   = "ahead"
	GitTagBehind  = "behind"
)

var GitStatusTagNames = []string{GitTagChanges, GitTagAhead, GitTagBehind}

// IsGitStatusTag reports whether tag is assigned by the git helper.
func IsGitStatusTag(tag string) bool {
	tag = strings.TrimSpace(tag)
	for _, name := range GitStatusTagNames {
		if strings.EqualFold(tag, name) {
			return true
		}
	}
	return false
}

type GitHelper struct{}

func (GitHelper) Name() string { return "git" }

func (GitHelper) Fields() []Field {
	return []Field{{
		ID:         FieldGitLastCommit,
		Label:      "Git updated",
		Kind:       FieldTime,
		Sortable:   true,
		Filterable: true,
	}}
}

func (GitHelper) Collect(ctx context.Context, projectPath string) (map[string]any, error) {
	meta, err := gitLastCommitMeta(ctx, projectPath)
	if err != nil {
		return nil, err
	}
	tags, tagErr := gitStatusTags(ctx, projectPath)
	if tagErr == nil {
		meta[MetaKeyTags] = tags
	}
	return meta, nil
}

func gitLastCommitMeta(ctx context.Context, projectPath string) (map[string]any, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", projectPath, "log", "-1", "--format=%cI")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	raw := strings.TrimSpace(stdout.String())
	if raw == "" {
		return map[string]any{}, nil
	}
	ts, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		if ts2, err2 := time.Parse("2006-01-02T15:04:05-07:00", raw); err2 == nil {
			ts = ts2
		} else {
			return nil, err
		}
	}
	return map[string]any{
		FieldGitLastCommit: ts.UTC(),
	}, nil
}

func gitStatusTags(ctx context.Context, projectPath string) ([]string, error) {
	dirty, err := gitWorktreeDirty(ctx, projectPath)
	if err != nil {
		return nil, err
	}
	ahead, behind, err := gitAheadBehind(ctx, projectPath)
	if err != nil {
		return nil, err
	}

	var tags []string
	if dirty {
		tags = append(tags, GitTagChanges)
	}
	if ahead > 0 {
		tags = append(tags, GitTagAhead)
	}
	if behind > 0 {
		tags = append(tags, GitTagBehind)
	}
	return tags, nil
}

func gitWorktreeDirty(ctx context.Context, projectPath string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", projectPath, "status", "--porcelain")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return false, err
	}
	return strings.TrimSpace(stdout.String()) != "", nil
}

func gitAheadBehind(ctx context.Context, projectPath string) (ahead, behind int, err error) {
	upstream, err := gitUpstream(ctx, projectPath)
	if err != nil || upstream == "" {
		return 0, 0, err
	}
	cmd := exec.CommandContext(ctx, "git", "-C", projectPath, "rev-list", "--left-right", "--count", upstream+"...HEAD")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return 0, 0, err
	}
	parts := strings.Fields(strings.TrimSpace(stdout.String()))
	if len(parts) != 2 {
		return 0, 0, nil
	}
	behind, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	ahead, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return ahead, behind, nil
}

func gitUpstream(ctx context.Context, projectPath string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", projectPath, "rev-parse", "--abbrev-ref", "@{upstream}")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", nil
	}
	return strings.TrimSpace(stdout.String()), nil
}
