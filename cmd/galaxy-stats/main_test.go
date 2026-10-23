package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msch128/galaxy-stats/internal/demo"
)

func TestRendersSelectedCardsFromASavedSnapshot(t *testing.T) {
	dir := t.TempDir()
	snap := filepath.Join(dir, "snapshot.json")
	if err := save(snap, demo.Snapshot()); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if err := run(context.Background(), []string{"-snapshot", snap, "-out", out, "-cards", "nightsky, kubectl"}); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(out)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if strings.Join(names, ",") != "kubectl.svg,nightsky.svg" {
		t.Errorf("wrote %v", names)
	}
}

func TestRejectsUnknownCardsAndMissingToken(t *testing.T) {
	if err := run(context.Background(), []string{"-cards", "snake"}); err == nil || !strings.Contains(err.Error(), "available: galaxy,nightsky,kubectl") {
		t.Errorf("unknown card: err = %v", err)
	}
	t.Setenv("GALAXY_STATS_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	if err := run(context.Background(), []string{"-user", "octo", "-out", t.TempDir()}); err == nil || !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Errorf("missing token: err = %v", err)
	}
}
