// Package galaxy draws the repositories a user committed to as star systems:
// size is the number of commits, colour is the primary language, glow is how
// recently the repository was pushed, and dashed hyperlanes connect
// repositories worked on in the same week. Private repositories appear as
// uncharted systems without a name.
package galaxy

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/msch128/galaxy-stats/internal/card"
	"github.com/msch128/galaxy-stats/internal/model"
	"github.com/msch128/galaxy-stats/internal/svg"
)

const (
	width       = 720
	height      = 420
	maxSystems  = 10
	goldenAngle = 2.39996
	font        = "ui-monospace,SFMono-Regular,Menlo,Consolas,monospace"
	labelColor  = "#e9e4ff"
	mutedColor  = "#7a6fa8"
	laneColor   = "#9f8cff"
	headColor   = "#cdbcff"
	valueColor  = "#f1edff"
)

// Card is the galaxy card.
type Card struct{}

func (Card) ID() string { return "galaxy" }

type system struct {
	repo   model.Repo
	label  string
	detail string
	x, y   float64
	r      float64
	heat   float64 // 0.25 (cold) .. 1 (pushed today)
	weeks  map[[2]int]bool
}

func (Card) Render(s *model.Snapshot, o card.Options) ([]byte, error) {
	var active []model.Repo
	for _, r := range s.Repos {
		if r.Commits > 0 {
			active = append(active, r)
		}
	}
	shown := active[:min(len(active), maxSystems)]
	systems := layout(s, shown)

	commits, uncharted := 0, 0
	langs := map[string]bool{}
	for _, r := range active {
		commits += r.Commits
		if r.Name == "" {
			uncharted++
		}
		if r.Language != "" {
			langs[r.Language] = true
		}
	}

	name := o.Name(s)
	desc := fmt.Sprintf("Galaxy of %s: %d repositories with %d commits in the last 12 months.", name, len(active), commits)
	doc := svg.New(width, height, "Galaxy of "+name, desc, font)
	doc.Raw(`<defs><radialGradient id="gx-neb"><stop offset="0" stop-color="#6b4bd6" stop-opacity=".35"/><stop offset="1" stop-color="#6b4bd6" stop-opacity="0"/></radialGradient>` +
		`<radialGradient id="gx-glow"><stop offset="0" stop-color="#fff" stop-opacity=".9"/><stop offset="1" stop-color="#fff" stop-opacity="0"/></radialGradient></defs>`)
	doc.Raw(`<style>.gx-pulse{animation:gx-pulse 5s ease-in-out infinite}@keyframes gx-pulse{50%{opacity:.15}}@media (prefers-reduced-motion:reduce){.gx-pulse{animation:none}}</style>`)
	doc.F(`<rect width="%d" height="%d" rx="10" fill="#07061a"/>`, width, height)

	field := card.Rand(s, "galaxy-field")
	for range 240 {
		r := []float64{.5, .6, .8, 1.1}[field.IntN(4)]
		doc.F(`<circle cx="%.0f" cy="%.0f" r="%.1f" fill="#fff" fill-opacity="%.2f"/>`, field.Float64()*width, field.Float64()*height, r, .15+field.Float64()*.55)
	}
	doc.Raw(`<ellipse cx="380" cy="230" rx="300" ry="150" fill="url(#gx-neb)"/>`)
	doc.Header("GALAXY", name+" · 12 months · every repository is a star system", headColor, mutedColor)

	for i, a := range systems {
		for _, b := range systems[i+1:] {
			shared := 0
			for w := range a.weeks {
				if b.weeks[w] {
					shared++
				}
			}
			if shared == 0 {
				continue
			}
			doc.F(`<line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="%s" stroke-opacity="%.2f" stroke-width="%.1f" stroke-dasharray="3 4"/>`,
				a.x, a.y, b.x, b.y, laneColor, math.Min(.15+float64(shared)/40, .6), math.Min(.6+float64(shared)/10, 2.4))
		}
	}

	for _, sy := range systems {
		col := card.LanguageColor(sy.repo.Language)
		class := ""
		if sy.heat > .9 {
			class = ` class="gx-pulse"`
		}
		doc.F(`<circle%s cx="%.0f" cy="%.0f" r="%.0f" fill="url(#gx-glow)" opacity="%.2f"/>`, class, sy.x, sy.y, sy.r*2.6, sy.heat*.45)
		doc.F(`<circle cx="%.0f" cy="%.0f" r="%.1f" fill="%s" fill-opacity="%.2f"/>`, sy.x, sy.y, sy.r, col, .5+sy.heat/2)
		doc.F(`<ellipse cx="%.0f" cy="%.0f" rx="%.0f" ry="%.0f" fill="none" stroke="%s" stroke-opacity=".35"/>`, sy.x, sy.y, sy.r*1.9, sy.r*.55, col)
	}
	for i, l := range placeLabels(systems) {
		sy := systems[i]
		nameColor := labelColor
		if sy.repo.Name == "" {
			nameColor = mutedColor
		}
		doc.F(`<text x="%.0f" y="%.0f" fill="%s" font-size="10" text-anchor="%s">%s</text>`, l.x, l.y, nameColor, l.anchor, svg.Esc(sy.label))
		doc.F(`<text x="%.0f" y="%.0f" fill="%s" font-size="8.5" text-anchor="%s">%s</text>`, l.x, l.y+12, mutedColor, l.anchor, svg.Esc(sy.detail))
	}

	if len(active) == 0 {
		doc.F(`<text x="%d" y="%d" fill="%s" font-size="12" text-anchor="middle">a quiet sector · no commits charted yet</text>`, width/2, height/2, mutedColor)
	}
	systemsLabel := "star systems"
	if hidden := len(active) - len(shown); hidden > 0 {
		systemsLabel = fmt.Sprintf("star systems (%d beyond the map)", hidden)
	}
	doc.Stats([]svg.Stat{
		{Value: fmt.Sprint(len(active)), Label: systemsLabel},
		{Value: fmt.Sprint(commits), Label: "commits charted"},
		{Value: fmt.Sprint(uncharted), Label: "uncharted (private)"},
		{Value: fmt.Sprint(len(langs)), Label: "languages"},
	}, 24, height-30, width-48, valueColor, mutedColor)
	return doc.Bytes(), nil
}

// layout places systems on a flattened golden-angle spiral, biggest in the
// middle, so the map fills evenly whatever the number of repositories.
func layout(s *model.Snapshot, repos []model.Repo) []system {
	peak := 1
	for _, r := range repos {
		peak = max(peak, r.Commits)
	}
	now := s.Now()
	out := make([]system, len(repos))
	private := 0
	for i, r := range repos {
		a := float64(i)*goldenAngle + .6
		d := 44 + 50*math.Sqrt(float64(i)+.3)
		label := r.Name
		if label == "" {
			label = "uncharted system " + card.PrivateLabel(private)
			private++
		}
		age := now.Sub(r.Pushed.UTC().Truncate(24*time.Hour)).Hours() / 24
		out[i] = system{
			repo:   r,
			label:  shorten(label, 26),
			detail: detail(r),
			x:      380 + d*math.Cos(a)*1.45,
			y:      228 + d*math.Sin(a)*.66,
			r:      4 + 20*math.Sqrt(float64(r.Commits)/float64(peak)),
			heat:   math.Max(.25, 1-age/90),
			weeks:  weeks(r.Days),
		}
	}
	return out
}

func detail(r model.Repo) string {
	out := fmt.Sprintf("%d commits", r.Commits)
	if r.Commits == 1 {
		out = "1 commit"
	}
	if r.Language != "" {
		out += " · " + r.Language
	}
	return out
}

type box struct{ x0, y0, x1, y1 float64 }

func (a box) overlap(b box) float64 {
	w := math.Min(a.x1, b.x1) - math.Max(a.x0, b.x0)
	h := math.Min(a.y1, b.y1) - math.Max(a.y0, b.y0)
	if w <= 0 || h <= 0 {
		return 0
	}
	return w * h
}

type label struct {
	x, y   float64 // baseline of the first line
	anchor string
}

// placeLabels puts each system's two-line label right, left, below or above
// the planet, whichever collides least with planets, earlier labels and the
// card's header and footer.
func placeLabels(systems []system) []label {
	const lineH, charW = 24.0, 6.1
	var taken []box
	for _, sy := range systems {
		taken = append(taken, box{sy.x - sy.r*1.9, sy.y - sy.r, sy.x + sy.r*1.9, sy.y + sy.r})
	}
	bounds := box{16, 48, width - 16, height - 52}
	out := make([]label, len(systems))
	for i, sy := range systems {
		w := float64(max(len([]rune(sy.label)), len([]rune(sy.detail))*85/100)) * charW
		gap := sy.r*1.9 + 6
		candidates := []struct {
			l label
			b box
		}{
			{label{sy.x + gap, sy.y + 3, "start"}, box{sy.x + gap, sy.y - 8, sy.x + gap + w, sy.y - 8 + lineH}},
			{label{sy.x - gap, sy.y + 3, "end"}, box{sy.x - gap - w, sy.y - 8, sy.x - gap, sy.y - 8 + lineH}},
			{label{sy.x, sy.y + sy.r + 14, "middle"}, box{sy.x - w/2, sy.y + sy.r + 4, sy.x + w/2, sy.y + sy.r + 4 + lineH}},
			{label{sy.x, sy.y - sy.r - 18, "middle"}, box{sy.x - w/2, sy.y - sy.r - 28, sy.x + w/2, sy.y - sy.r - 28 + lineH}},
		}
		best, bestCost := 0, math.Inf(1)
		for k, c := range candidates {
			cost := 0.0
			for j, t := range taken {
				if j != i {
					cost += c.b.overlap(t)
				}
			}
			outside := c.b.x1 - c.b.x0
			outside = outside*(c.b.y1-c.b.y0) - c.b.overlap(bounds)
			cost += outside * 4
			if cost < bestCost {
				best, bestCost = k, cost
			}
		}
		out[i] = candidates[best].l
		taken = append(taken, candidates[best].b)
	}
	return out
}

func weeks(days []model.Day) map[[2]int]bool {
	out := map[[2]int]bool{}
	for _, d := range days {
		y, w := d.Time().ISOWeek()
		out[[2]int{y, w}] = true
	}
	return out
}

func shorten(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n-1])) + "…"
}
