package cards_test

import (
	"bytes"
	"encoding/xml"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/msch128/galaxy-stats/internal/card"
	"github.com/msch128/galaxy-stats/internal/cards"
	"github.com/msch128/galaxy-stats/internal/demo"
	"github.com/msch128/galaxy-stats/internal/model"
)

var update = flag.Bool("update", false, "rewrite the example images in docs/examples")

const examplesDir = "../../docs/examples"

// The README shows these files, so they double as golden files: a change in
// any card's output must be reviewed by regenerating them with -update.
func TestExamplesMatchDemo(t *testing.T) {
	for _, c := range cards.All() {
		got, err := c.Render(demo.Snapshot(), card.Options{})
		if err != nil {
			t.Fatalf("%s: %v", c.ID(), err)
		}
		path := filepath.Join(examplesDir, c.ID()+".svg")
		if *update {
			if err := os.WriteFile(path, got, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s (run go test ./internal/cards -update): %v", c.ID(), err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from %s; review and run go test ./internal/cards -update", c.ID(), path)
		}
	}
}

func TestEveryCardRendersValidSVGForAnyAccount(t *testing.T) {
	snapshots := map[string]*model.Snapshot{
		"demo":     demo.Snapshot(),
		"empty":    {Login: "newcomer", Today: "2026-10-23"},
		"sparse":   sparse(),
		"unsorted": {Login: "x", Today: "2026-10-23", Calendar: []model.Day{{Date: "2026-10-23", Count: 1}}, Repos: []model.Repo{{Name: "solo", Commits: 1, Days: []model.Day{{Date: "2026-10-23", Count: 1}}}}},
	}
	for name, s := range snapshots {
		for _, theme := range []string{"dark", "light"} {
			for _, c := range cards.All() {
				b, err := c.Render(s, card.Options{Theme: theme})
				if err != nil {
					t.Fatalf("%s/%s/%s: %v", name, theme, c.ID(), err)
				}
				assertWellFormed(t, name+"/"+c.ID(), b)
				if bytes.Contains(b, []byte("NaN")) || bytes.Contains(b, []byte("Inf")) {
					t.Errorf("%s/%s: output contains NaN or Inf coordinates", name, c.ID())
				}
			}
		}
	}
}

func TestRepositoryNamesAreEscaped(t *testing.T) {
	s := demo.Snapshot()
	s.Repos[0].Name = `<script>alert("x")</script>&co`
	for _, c := range cards.All() {
		b, err := c.Render(s, card.Options{DisplayName: `"><b>`})
		if err != nil {
			t.Fatal(err)
		}
		assertWellFormed(t, c.ID(), b)
		if bytes.Contains(b, []byte("<script>")) || bytes.Contains(b, []byte(`"><b>`)) {
			t.Errorf("%s: user text is not escaped", c.ID())
		}
	}
}

func TestAnonymousRepositoriesStayAnonymous(t *testing.T) {
	s := demo.Snapshot()
	for _, c := range cards.All() {
		b, _ := c.Render(s, card.Options{})
		if c.ID() == "galaxy" && !bytes.Contains(b, []byte("uncharted system")) {
			t.Errorf("galaxy: private repositories should appear as uncharted systems")
		}
		if c.ID() == "kubectl" && !bytes.Contains(b, []byte("closed-source-01")) {
			t.Errorf("kubectl: private repositories should appear as closed-source containers")
		}
	}
}

func TestRenderingIsDeterministic(t *testing.T) {
	for _, c := range cards.All() {
		a, _ := c.Render(demo.Snapshot(), card.Options{})
		b, _ := c.Render(demo.Snapshot(), card.Options{})
		if !bytes.Equal(a, b) {
			t.Errorf("%s: two renders of the same snapshot differ", c.ID())
		}
	}
}

func TestNightSkySkipsLeadingQuietWeeks(t *testing.T) {
	b, err := cardByID(t, "nightsky").Render(sparse(), card.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte("last 8 weeks")) {
		t.Errorf("a calendar with three recent active weeks should shrink to the 8-week minimum")
	}
}

// sparse has a quiet year except for the last three weeks.
func sparse() *model.Snapshot {
	today := demo.Today
	s := &model.Snapshot{Login: "sparse", Today: today.Format(time.DateOnly)}
	for ago := 364; ago >= 0; ago-- {
		n := 0
		if ago < 21 && ago%3 != 0 {
			n = 4
		}
		s.Calendar = append(s.Calendar, model.Day{Date: today.AddDate(0, 0, -ago).Format(time.DateOnly), Count: n})
	}
	return s
}

func cardByID(t *testing.T, id string) card.Card {
	for _, c := range cards.All() {
		if c.ID() == id {
			return c
		}
	}
	t.Fatalf("no card %q", id)
	return nil
}

func assertWellFormed(t *testing.T, name string, b []byte) {
	t.Helper()
	if !strings.HasPrefix(string(b), "<svg ") {
		t.Errorf("%s: output does not start with <svg", name)
	}
	dec := xml.NewDecoder(bytes.NewReader(b))
	for {
		_, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			t.Errorf("%s: not well-formed XML: %v", name, err)
			return
		}
	}
}
