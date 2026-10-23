// Package nightsky draws the contribution calendar as a night sky: every day
// is a star, its brightness is the day's contributions, and streaks of
// consecutive days are joined into constellations.
package nightsky

import (
	"fmt"
	"math"
	"slices"

	"github.com/msch128/galaxy-stats/internal/card"
	"github.com/msch128/galaxy-stats/internal/model"
	"github.com/msch128/galaxy-stats/internal/svg"
)

const (
	width         = 720
	height        = 380
	maxWeeks      = 26
	minWeeks      = 8
	minStreak     = 5 // days in a row that form a constellation
	skyTop        = 70
	labelY        = 300 // baseline of the constellation caption, below the sky
	rowHeight     = 30
	marginX       = 40
	font          = "ui-monospace,SFMono-Regular,Menlo,Consolas,monospace"
	starColor     = "#fff7d6"
	glowColor     = "#ffe9a8"
	lineColor     = "#9fb4ff"
	textColor     = "#7c86b8"
	headlineColor = "#ffe9a8"
)

// Card is the night sky card.
type Card struct{}

func (Card) ID() string { return "nightsky" }

func (Card) Render(s *model.Snapshot, o card.Options) ([]byte, error) {
	days := window(s.LastDays(maxWeeks * 7))
	weeks := (len(days) + 6) / 7
	stars := place(s, days, weeks)
	streaks := constellations(days)
	peak := 0
	lit := 0
	for _, d := range days {
		peak = max(peak, d.Count)
		if d.Count > 0 {
			lit++
		}
	}

	name := o.Name(s)
	desc := fmt.Sprintf("Night sky of %s: %d of the last %d days have contributions, joined into %d constellations.", name, lit, len(days), len(streaks))
	doc := svg.New(width, height, "Night sky of "+name, desc, font)
	doc.Raw(`<defs><linearGradient id="ns-sky" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#050816"/><stop offset="1" stop-color="#14204a"/></linearGradient></defs>`)
	doc.Raw(`<style>.ns-tw{animation:ns-tw 4s ease-in-out infinite}@keyframes ns-tw{50%{opacity:.35}}@media (prefers-reduced-motion:reduce){.ns-tw{animation:none}}</style>`)
	doc.F(`<rect width="%d" height="%d" rx="10" fill="url(#ns-sky)"/>`, width, height)
	doc.Header("NIGHT SKY", fmt.Sprintf("%s · last %d weeks · every day a star", name, weeks), headlineColor, textColor)

	dust := card.Rand(s, "nightsky-dust")
	for range 90 {
		doc.F(`<circle cx="%.0f" cy="%.0f" r=".6" fill="#fff" fill-opacity="%.2f"/>`, dust.Float64()*width, 50+dust.Float64()*(height-110), .08+dust.Float64()*.2)
	}

	for _, st := range streaks {
		doc.Raw(`<polyline points="`)
		for i, idx := range brightOnly(days, st) {
			if i > 0 {
				doc.Raw(" ")
			}
			doc.F("%.0f,%.0f", stars[idx].x, stars[idx].y)
		}
		doc.F(`" fill="none" stroke="%s" stroke-opacity=".35" stroke-width=".9"/>`, lineColor)
	}

	brightest := brightestIndexes(days, 3)
	for i, d := range days {
		p := stars[i]
		if d.Count == 0 {
			doc.F(`<circle cx="%.0f" cy="%.0f" r=".7" fill="#fff" fill-opacity=".18"/>`, p.x, p.y)
			continue
		}
		q := math.Sqrt(float64(d.Count) / float64(peak))
		r := 1.2 + 4.2*q
		class := ""
		if brightest[i] {
			class = ` class="ns-tw"`
		}
		doc.F(`<g%s><circle cx="%.0f" cy="%.0f" r="%.1f" fill="%s" fill-opacity="%.2f"/><circle cx="%.0f" cy="%.0f" r="%.1f" fill="%s"/>`,
			class, p.x, p.y, r*3, glowColor, .06+.12*q, p.x, p.y, r, starColor)
		if q > .55 {
			doc.F(`<path d="M%.0f %.0fH%.0fM%.0f %.0fV%.0f" stroke="%s" stroke-opacity=".5" stroke-width=".7"/>`,
				p.x-r*3, p.y, p.x+r*3, p.x, p.y-r*3, p.y+r*3, starColor)
		}
		doc.Raw(`</g>`)
	}

	longest := 0
	if len(streaks) > 0 {
		best := streaks[0]
		for _, st := range streaks[1:] {
			if len(st) > len(best) {
				best = st
			}
		}
		longest = len(best)
		mid := stars[best[len(best)/2]]
		first, last := days[best[0]].Time(), days[best[len(best)-1]].Time()
		label := fmt.Sprintf("constellation “%d-day streak” · %s–%s", len(best), first.Format("02 Jan"), last.Format("02 Jan"))
		half := float64(len([]rune(label))) * 6.1 / 2
		x := math.Min(math.Max(mid.x, 24+half), width-24-half)
		doc.F(`<line x1="%.0f" y1="%.0f" x2="%.0f" y2="%d" stroke="%s" stroke-opacity=".3" stroke-dasharray="2 3"/>`, mid.x, mid.y+8, mid.x, labelY-12, lineColor)
		doc.F(`<text x="%.0f" y="%d" fill="%s" font-size="10" font-style="italic" text-anchor="middle">%s</text>`, x, labelY, lineColor, svg.Esc(label))
	}

	doc.Stats([]svg.Stat{
		{Value: fmt.Sprint(lit), Label: "stars lit"},
		{Value: fmt.Sprint(len(streaks)), Label: fmt.Sprintf("constellations (%d+ days)", minStreak)},
		{Value: fmt.Sprint(peak), Label: "brightest night"},
		{Value: fmt.Sprintf("%d d", longest), Label: "longest constellation"},
	}, 24, height-34, width-48, starColor, textColor)
	return doc.Bytes(), nil
}

// window trims leading quiet weeks so a sky never opens with a dark gap, but
// keeps at least minWeeks so a short burst of activity still has room.
func window(days []model.Day) []model.Day {
	first := len(days)
	for i, d := range days {
		if d.Count > 0 {
			first = i
			break
		}
	}
	start := first - first%7 // keep whole seven-day columns
	if keep := minWeeks * 7; len(days)-start < keep {
		start = max(0, len(days)-keep)
	}
	return days[start:]
}

type point struct{ x, y float64 }

// place lays the days out week by week, with a little deterministic jitter and
// a gentle wave so the sky does not look like a grid.
func place(s *model.Snapshot, days []model.Day, weeks int) []point {
	rnd := card.Rand(s, "nightsky-stars")
	col := float64(width-2*marginX) / float64(max(weeks, 1))
	out := make([]point, len(days))
	for i, d := range days {
		wk := i / 7
		wd := int(d.Time().Weekday()+6) % 7 // Monday first
		x := marginX + (float64(wk)+.5)*col + (rnd.Float64()-.5)*col*.55
		y := skyTop + float64(wd)*rowHeight + (rnd.Float64()-.5)*18 + 10*math.Sin(float64(wk)/4)
		out[i] = point{x, y}
	}
	return out
}

// constellations returns runs of at least minStreak active days, as indexes.
func constellations(days []model.Day) [][]int {
	var out [][]int
	var cur []int
	flush := func() {
		if len(cur) >= minStreak {
			out = append(out, cur)
		}
		cur = nil
	}
	for i, d := range days {
		if d.Count > 0 {
			cur = append(cur, i)
		} else {
			flush()
		}
	}
	flush()
	return out
}

// brightOnly keeps the stars of a streak that are at least as bright as its
// median, so constellations zigzag between highlights instead of tracing
// every day of a week column.
func brightOnly(days []model.Day, streak []int) []int {
	counts := make([]int, len(streak))
	for i, idx := range streak {
		counts[i] = days[idx].Count
	}
	slices.Sort(counts)
	median := counts[len(counts)/2]
	var out []int
	for _, idx := range streak {
		if days[idx].Count >= median {
			out = append(out, idx)
		}
	}
	if len(out) < 3 {
		return streak
	}
	return out
}

// brightestIndexes marks the n days with the most contributions.
func brightestIndexes(days []model.Day, n int) map[int]bool {
	out := map[int]bool{}
	for range n {
		best := -1
		for i, d := range days {
			if d.Count > 0 && !out[i] && (best < 0 || d.Count > days[best].Count) {
				best = i
			}
		}
		if best < 0 {
			break
		}
		out[best] = true
	}
	return out
}
