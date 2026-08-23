package payload

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"

	"github.com/stmytsyk/agent-handoff-core/pkg/redaction"
)

type Manifest struct {
	SchemaVersion string    `json:"schema_version"`
	CreatedAt     time.Time `json:"created_at"`
	Branch        string    `json:"branch,omitempty"`
	GitStatus     string    `json:"git_status"`
	GitDiff       string    `json:"git_diff"`
	Summary       []string  `json:"summary"`
	PendingTasks  []string  `json:"pending_tasks,omitempty"`
}

type Builder struct {
	WorkDir string
	Now     func() time.Time
}

type Options struct {
	Summary      []string
	PendingTasks []string
}

func (b Builder) Build(ctx context.Context, opts Options) (Manifest, error) {
	now := b.Now
	if now == nil {
		now = time.Now
	}
	branch, _ := b.git(ctx, "branch", "--show-current")
	status, _ := b.git(ctx, "status", "--porcelain")
	diff, _ := b.git(ctx, "diff", "HEAD")
	return Manifest{
		SchemaVersion: "ahp-payload/v1",
		CreatedAt:     now().UTC(),
		Branch:        strings.TrimSpace(branch),
		GitStatus:     sanitizeGitFileList(status),
		GitDiff:       redaction.SanitizePayload(filterSensitiveDiff(diff)),
		Summary:       sanitizeList(opts.Summary),
		PendingTasks:  sanitizeList(opts.PendingTasks),
	}, nil
}

func (b Builder) JSON(ctx context.Context, opts Options) ([]byte, error) {
	manifest, err := b.Build(ctx, opts)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(manifest, "", "  ")
}

func (b Builder) git(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if b.WorkDir != "" {
		cmd.Dir = b.WorkDir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(out), err
}

func sanitizeList(in []string) []string {
	out := make([]string, 0, len(in))
	for _, item := range in {
		out = append(out, redaction.SanitizePayload(item))
	}
	return out
}

func sanitizeGitFileList(status string) string {
	var kept []string
	for _, line := range strings.Split(status, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		path := strings.TrimSpace(line[3:])
		if redaction.IsSensitivePath(path) {
			kept = append(kept, line[:3]+"[REDACTED_SENSITIVE_FILE]")
			continue
		}
		kept = append(kept, redaction.SanitizePayload(line))
	}
	return strings.Join(kept, "\n")
}

func filterSensitiveDiff(diff string) string {
	var out []string
	skip := false
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			skip = false
			fields := strings.Fields(line)
			for _, field := range fields {
				path := strings.TrimPrefix(strings.TrimPrefix(field, "a/"), "b/")
				if redaction.IsSensitivePath(path) {
					out = append(out, "diff --git [REDACTED_SENSITIVE_FILE]")
					skip = true
					break
				}
			}
			if !skip {
				out = append(out, line)
			}
			continue
		}
		if skip {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
