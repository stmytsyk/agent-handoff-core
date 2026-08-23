package payload

import (
	"context"
	"testing"
)

func TestBuilderOmitsGitErrorsOutsideRepository(t *testing.T) {
	manifest, err := (Builder{WorkDir: t.TempDir()}).Build(context.Background(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if manifest.GitStatus != "" {
		t.Fatalf("expected empty git status outside repo, got %q", manifest.GitStatus)
	}
	if manifest.GitDiff != "" {
		t.Fatalf("expected empty git diff outside repo, got %q", manifest.GitDiff)
	}
}
