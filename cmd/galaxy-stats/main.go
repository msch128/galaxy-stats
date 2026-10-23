// Command galaxy-stats renders GitHub profile cards as SVG files.
//
// It fetches one snapshot of the user's activity and renders every requested
// card from it. The token is read from GALAXY_STATS_TOKEN or GITHUB_TOKEN and
// never accepted as a flag, so it does not end up in shell history or logs.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/msch128/galaxy-stats/internal/card"
	"github.com/msch128/galaxy-stats/internal/cards"
	"github.com/msch128/galaxy-stats/internal/github"
	"github.com/msch128/galaxy-stats/internal/model"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "galaxy-stats:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("galaxy-stats", flag.ContinueOnError)
	user := fs.String("user", "", "GitHub login to render (required unless -snapshot is given)")
	selected := fs.String("cards", ids(), "comma-separated cards to render")
	out := fs.String("out", "cards", "directory to write the SVG files to")
	tz := fs.String("timezone", "UTC", "IANA time zone for commit hours, e.g. Europe/Berlin")
	theme := fs.String("theme", "dark", "colour theme for cards that support it: dark or light")
	name := fs.String("display-name", "", "name shown on the cards (default: the login)")
	showPrivate := fs.Bool("show-private-names", false, "show names of private repositories instead of anonymising them")
	snapshotIn := fs.String("snapshot", "", "render from a saved snapshot file instead of the GitHub API")
	snapshotOut := fs.String("save-snapshot", "", "also write the fetched snapshot to this file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	chosen, err := pick(*selected)
	if err != nil {
		return err
	}

	var snap *model.Snapshot
	if *snapshotIn != "" {
		snap, err = load(*snapshotIn)
	} else {
		snap, err = fetch(ctx, *user, *tz, *showPrivate)
	}
	if err != nil {
		return err
	}
	if *snapshotOut != "" {
		if err := save(*snapshotOut, snap); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}
	opts := card.Options{Theme: *theme, DisplayName: *name}
	for _, c := range chosen {
		b, err := c.Render(snap, opts)
		if err != nil {
			return fmt.Errorf("%s: %w", c.ID(), err)
		}
		path := filepath.Join(*out, c.ID()+".svg")
		if err := os.WriteFile(path, b, 0o644); err != nil {
			return err
		}
		fmt.Println("wrote", path)
	}
	return nil
}

func fetch(ctx context.Context, user, tz string, showPrivate bool) (*model.Snapshot, error) {
	if user == "" {
		return nil, fmt.Errorf("-user is required")
	}
	token := os.Getenv("GALAXY_STATS_TOKEN")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		return nil, fmt.Errorf("set GALAXY_STATS_TOKEN or GITHUB_TOKEN")
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, fmt.Errorf("timezone %q: %w", tz, err)
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	c := &github.Client{Token: token}
	return c.Fetch(ctx, github.Request{Login: user, Timezone: tz, Today: time.Now().In(loc), ShowPrivateNames: showPrivate})
}

func pick(list string) ([]card.Card, error) {
	var out []card.Card
	for _, id := range strings.Split(list, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		all := cards.All()
		i := slices.IndexFunc(all, func(c card.Card) bool { return c.ID() == id })
		if i < 0 {
			return nil, fmt.Errorf("unknown card %q (available: %s)", id, ids())
		}
		out = append(out, all[i])
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no cards selected (available: %s)", ids())
	}
	return out, nil
}

func ids() string {
	all := cards.All()
	names := make([]string, len(all))
	for i, c := range all {
		names[i] = c.ID()
	}
	return strings.Join(names, ",")
}

func load(path string) (*model.Snapshot, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s model.Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	s.SortRepos()
	return &s, nil
}

func save(path string, s *model.Snapshot) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
