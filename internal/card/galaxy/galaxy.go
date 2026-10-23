// Package galaxy draws the repositories a user committed to as planets in a
// spiral galaxy, in the style of a strategy game's galactic map.
//
// Planet size is the number of commits, the planet's world type follows its
// primary language, moons count the weeks of activity, and a turning holo
// ring marks repositories pushed in the last two weeks. Hyperlanes connect
// repositories worked on in the same weeks. Private repositories are
// uncharted systems hidden in fog, without a name.
package galaxy

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/msch128/galaxy-stats/internal/card"
	"github.com/msch128/galaxy-stats/internal/model"
	"github.com/msch128/galaxy-stats/internal/svg"
)

const (
	width      = 760
	height     = 460
	maxSystems = 10
	font       = "ui-monospace,SFMono-Regular,Menlo,Consolas,monospace"
	hudColor   = "#c9b8ff"
	mutedColor = "#8479b8"
	textColor  = "#f1edff"
	laneColor  = "#8fd3ff"
	coreX      = width * .5
	coreY      = height * .47
	tilt       = -.21 // galaxy rotation in radians
	squash     = .42  // vertical flattening of the galactic disc
)

// Card is the galaxy card.
type Card struct{}

func (Card) ID() string { return "galaxy" }

type planet struct {
	repo   model.Repo
	label  string
	detail string
	world  world
	x, y   float64
	r      float64
	active bool // pushed within the last 14 days
	moons  int
	weeks  map[[2]int]bool
}

func (Card) Render(s *model.Snapshot, o card.Options) ([]byte, error) {
	var active []model.Repo
	for _, r := range s.Repos {
		if r.Commits > 0 {
			active = append(active, r)
		}
	}
	planets := layout(s, active[:min(len(active), maxSystems)])

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
	desc := fmt.Sprintf("Galaxy of %s: %d repositories with %d commits in the last 12 months, drawn as planets.", name, len(active), commits)
	doc := svg.New(width, height, "Galaxy of "+name, desc, font)
	doc.Raw(`<style>` +
		`.gx-flow{animation:gx-flow 3s linear infinite}@keyframes gx-flow{to{stroke-dashoffset:-28}}` +
		`.gx-spin{transform-box:fill-box;transform-origin:center;animation:gx-spin 18s linear infinite}@keyframes gx-spin{to{transform:rotate(360deg)}}` +
		`.gx-core{animation:gx-core 6s ease-in-out infinite}@keyframes gx-core{50%{opacity:.75}}` +
		`@media (prefers-reduced-motion:reduce){.gx-flow,.gx-spin,.gx-core{animation:none}}</style>`)

	doc.Raw(`<defs>`)
	sharedDefs(doc, s)
	for i, p := range planets {
		planetDefs(doc, s, i, p)
	}
	doc.Raw(`</defs>`)

	doc.F(`<rect width="%d" height="%d" rx="12" fill="#04030d"/>`, width, height)
	backdrop(doc, s)
	lanes(doc, planets)
	for i, p := range planets {
		drawPlanet(doc, i, p)
	}
	for i, l := range placeLabels(planets) {
		drawLabel(doc, planets[i], l)
	}
	if len(active) == 0 {
		doc.F(`<text x="%d" y="%d" fill="%s" font-size="12" text-anchor="middle" letter-spacing="1">A QUIET SECTOR · NO SYSTEMS CHARTED YET</text>`, width/2, height/2+70, mutedColor)
	}
	hud(doc, name, []svg.Stat{
		{Value: fmt.Sprint(len(active)), Label: "systems"},
		{Value: fmt.Sprint(commits), Label: "commits charted"},
		{Value: fmt.Sprint(uncharted), Label: "uncharted (private)"},
		{Value: fmt.Sprint(len(langs)), Label: "languages"},
	}, len(active)-len(planets))
	return doc.Bytes(), nil
}

// sharedDefs defines the gradients and filters every planet uses.
func sharedDefs(doc *svg.Doc, s *model.Snapshot) {
	doc.Raw(`<radialGradient id="gx-shade" cx=".36" cy=".32" r=".78">` +
		`<stop offset="0" stop-color="#fff" stop-opacity=".32"/><stop offset=".38" stop-color="#fff" stop-opacity="0"/>` +
		`<stop offset=".72" stop-color="#000" stop-opacity=".45"/><stop offset="1" stop-color="#000" stop-opacity=".88"/></radialGradient>`)
	doc.Raw(`<radialGradient id="gx-core"><stop offset="0" stop-color="#fffaf0"/><stop offset=".18" stop-color="#ffe2a8" stop-opacity=".9"/>` +
		`<stop offset=".5" stop-color="#ff9fd0" stop-opacity=".25"/><stop offset="1" stop-color="#7b4dff" stop-opacity="0"/></radialGradient>`)
	doc.Raw(`<radialGradient id="gx-disc"><stop offset="0" stop-color="#6d4bd8" stop-opacity=".38"/><stop offset=".55" stop-color="#3b2a8a" stop-opacity=".16"/>` +
		`<stop offset="1" stop-color="#120c33" stop-opacity="0"/></radialGradient>`)
	doc.Raw(`<radialGradient id="gx-fogfade"><stop offset=".25" stop-color="#b9b2e6"/><stop offset="1" stop-color="#b9b2e6" stop-opacity="0"/></radialGradient>`)
	doc.Raw(`<filter id="gx-blur8" x="-50%" y="-50%" width="200%" height="200%"><feGaussianBlur stdDeviation="8"/></filter>`)
	doc.Raw(`<filter id="gx-blur2" x="-50%" y="-50%" width="200%" height="200%"><feGaussianBlur stdDeviation="2"/></filter>`)
	seed := card.Rand(s, "galaxy-seed").IntN(900) + 1
	// Nebulae: fractal noise tinted violet and teal, faded towards the edges by a mask.
	doc.F(`<filter id="gx-nebula" x="0" y="0" width="100%%" height="100%%" color-interpolation-filters="sRGB">`+
		`<feTurbulence type="fractalNoise" baseFrequency=".006 .011" numOctaves="5" seed="%d"/>`+
		`<feColorMatrix values="0 0 0 0 .48  0 0 0 0 .26  0 0 0 0 .95  1.9 0 0 0 -.78"/></filter>`, seed)
	doc.F(`<filter id="gx-nebula2" x="0" y="0" width="100%%" height="100%%" color-interpolation-filters="sRGB">`+
		`<feTurbulence type="fractalNoise" baseFrequency=".009 .016" numOctaves="4" seed="%d"/>`+
		`<feColorMatrix values="0 0 0 0 .18  0 0 0 0 .62  0 0 0 0 .86  0 1.8 0 0 -.86"/></filter>`, seed+7)
	doc.Raw(`<radialGradient id="gx-nebmask-g" cx=".5" cy=".5" r=".55"><stop offset=".2" stop-color="#fff"/><stop offset="1" stop-color="#fff" stop-opacity="0"/></radialGradient>` +
		`<mask id="gx-nebmask"><rect width="100%" height="100%" fill="url(#gx-nebmask-g)"/></mask>`)
	doc.F(`<filter id="gx-fog" x="-20%%" y="-20%%" width="140%%" height="140%%" color-interpolation-filters="sRGB">`+
		`<feTurbulence type="fractalNoise" baseFrequency=".07" numOctaves="3" seed="%d"/>`+
		`<feColorMatrix values="0 0 0 0 .72  0 0 0 0 .69  0 0 0 0 .9  2.6 0 0 0 -.95"/>`+
		`<feComposite in2="SourceGraphic" operator="in"/><feGaussianBlur stdDeviation=".8"/></filter>`, seed+13)
}

// backdrop paints the spiral galaxy: background stars, nebulae, the disc's
// glow, two arms of dust and stars, and the core.
func backdrop(doc *svg.Doc, s *model.Snapshot) {
	rnd := card.Rand(s, "galaxy-field")
	for range 170 {
		doc.F(`<circle cx="%.1f" cy="%.1f" r="%.1f" fill="#fff" fill-opacity="%.2f"/>`, rnd.Float64()*width, rnd.Float64()*height, .3+rnd.Float64()*.7, .15+rnd.Float64()*.5)
	}
	doc.Raw(`<g mask="url(#gx-nebmask)"><rect width="100%" height="100%" filter="url(#gx-nebula)" opacity=".85"/><rect width="100%" height="100%" filter="url(#gx-nebula2)" opacity=".6"/></g>`)
	doc.F(`<ellipse cx="%.0f" cy="%.0f" rx="330" ry="150" fill="url(#gx-disc)" transform="rotate(%.1f %.0f %.0f)"/>`, coreX, coreY, tilt*180/math.Pi, coreX, coreY)

	arms := card.Rand(s, "galaxy-arms")
	tints := []string{"#9f7bff", "#ff8fcf", "#7fb8ff"}
	for arm := range 2 {
		for i := range 46 { // dust lanes along the arm
			t := float64(i) / 46
			x, y := spiral(arm, t, (arms.Float64()-.5)*.25)
			doc.F(`<ellipse cx="%.1f" cy="%.1f" rx="%.1f" ry="%.1f" fill="%s" opacity="%.2f" filter="url(#gx-blur8)"/>`,
				x, y, 14+22*t, 6+8*t, tints[arms.IntN(len(tints))], .10+.16*(1-t))
		}
		for range 330 { // stars along the arm, denser towards the core
			t := math.Pow(arms.Float64(), 1.35)
			x, y := spiral(arm, t, (arms.Float64()-.5)*(.18+.5*t))
			c := []string{"#ffffff", "#dfe7ff", "#ffe9c9", "#ffd1ea"}[arms.IntN(4)]
			doc.F(`<circle cx="%.1f" cy="%.1f" r="%.2f" fill="%s" fill-opacity="%.2f"/>`, x, y, .3+arms.Float64()*.9, c, .35+arms.Float64()*.6)
		}
	}
	doc.F(`<g class="gx-core"><ellipse cx="%.0f" cy="%.0f" rx="92" ry="44" fill="url(#gx-core)" transform="rotate(%.1f %.0f %.0f)"/>`+
		`<circle cx="%.0f" cy="%.0f" r="7" fill="#fffaf0" filter="url(#gx-blur2)"/></g>`, coreX, coreY, tilt*180/math.Pi, coreX, coreY, coreX, coreY)
}

// spiral returns a point on one of two logarithmic arms; t runs from the
// core (0) to the rim (1) and spread pushes the point off the arm's spine.
func spiral(arm int, t, spread float64) (float64, float64) {
	theta := t*3.6 + float64(arm)*math.Pi + spread
	r := 18 + 300*t
	x, y := r*math.Cos(theta), r*math.Sin(theta)*squash
	return coreX + x*math.Cos(tilt) - y*math.Sin(tilt), coreY + x*math.Sin(tilt) + y*math.Cos(tilt)
}

// layout puts the busiest repositories on the inner arms and pushes planets
// apart until none overlap, so every map stays readable.
func layout(s *model.Snapshot, repos []model.Repo) []planet {
	peak := 1
	for _, r := range repos {
		peak = max(peak, r.Commits)
	}
	now := s.Now()
	jitter := card.Rand(s, "galaxy-planets")
	out := make([]planet, len(repos))
	private := 0
	for i, r := range repos {
		label := r.Name
		if label == "" {
			label = "uncharted system " + card.PrivateLabel(private)
			private++
		}
		wk := weeks(r.Days)
		w := worldFor(r.Language)
		radius := 8 + 22*math.Sqrt(float64(r.Commits)/float64(peak))
		if w.gas {
			radius *= 1.12
		}
		t := .2 + .72*float64(i)/float64(max(len(repos)-1, 1))
		x, y := spiral(i%2, t, (jitter.Float64()-.5)*.35)
		out[i] = planet{
			repo:   r,
			label:  shorten(label, 24),
			detail: detail(r),
			world:  w,
			x:      x,
			y:      y,
			r:      round(radius),
			active: now.Sub(r.Pushed.UTC().Truncate(24*time.Hour)) <= 14*24*time.Hour,
			moons:  min(3, len(wk)/6),
			weeks:  wk,
		}
	}
	relax(out)
	return out
}

// relax separates overlapping planets and keeps them inside the map.
func relax(ps []planet) {
	const gap = 46.0
	for range 80 {
		for i := range ps {
			for j := i + 1; j < len(ps); j++ {
				dx, dy := ps[j].x-ps[i].x, ps[j].y-ps[i].y
				d := math.Hypot(dx, dy)
				want := ps[i].r + ps[j].r + gap
				if d >= want {
					continue
				}
				if d < .01 {
					dx, dy, d = 1, 0, 1
				}
				push := (want - d) / 2
				ps[i].x -= dx / d * push
				ps[i].y -= dy / d * push * .6
				ps[j].x += dx / d * push
				ps[j].y += dy / d * push * .6
			}
		}
		for i := range ps {
			ps[i].x = math.Min(math.Max(ps[i].x, 60+ps[i].r), width-60-ps[i].r)
			ps[i].y = math.Min(math.Max(ps[i].y, 78+ps[i].r), height-92-ps[i].r)
		}
	}
	for i := range ps { // snap, so floating-point differences between CPUs cannot move anything
		ps[i].x, ps[i].y = round(ps[i].x), round(ps[i].y)
	}
}

// lanes draws curved, glowing hyperlanes between planets that were worked on
// in the same weeks. Each planet keeps only its strongest routes, so the map
// reads like a network rather than a web, and energy flows along the busiest.
func lanes(doc *svg.Doc, ps []planet) {
	const perPlanet = 3
	type lane struct{ a, b, shared int }
	var all []lane
	for i := range ps {
		for j := i + 1; j < len(ps); j++ {
			shared := 0
			for w := range ps[i].weeks {
				if ps[j].weeks[w] {
					shared++
				}
			}
			if shared > 0 {
				all = append(all, lane{i, j, shared})
			}
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].shared > all[j].shared })
	degree := make([]int, len(ps))
	for _, l := range all {
		if degree[l.a] >= perPlanet || degree[l.b] >= perPlanet {
			continue
		}
		degree[l.a]++
		degree[l.b]++
		a, b := ps[l.a], ps[l.b]
		mx, my := (a.x+b.x)/2, (a.y+b.y)/2
		cx, cy := mx+(coreX-mx)*.25, my+(coreY-my)*.25 // bow towards the core
		path := fmt.Sprintf("M%.1f %.1fQ%.1f %.1f %.1f %.1f", a.x, a.y, cx, cy, b.x, b.y)
		strength := math.Min(float64(l.shared)/12, 1)
		doc.F(`<path d="%s" fill="none" stroke="%s" stroke-width="%.1f" stroke-opacity="%.2f" filter="url(#gx-blur2)"/>`, path, laneColor, 2+3*strength, .12+.2*strength)
		doc.F(`<path d="%s" fill="none" stroke="%s" stroke-width=".8" stroke-opacity="%.2f"/>`, path, laneColor, .25+.4*strength)
		if strength > .35 {
			doc.F(`<path class="gx-flow" d="%s" fill="none" stroke="#e8f7ff" stroke-width="1.4" stroke-linecap="round" stroke-dasharray="2 26" stroke-opacity=".9"/>`, path)
		}
	}
}

// planetDefs defines a planet's surface filters, atmosphere and clip path.
func planetDefs(doc *svg.Doc, s *model.Snapshot, i int, p planet) {
	w := p.world
	seed := card.Rand(s, fmt.Sprint("planet-", i)).IntN(900) + 1
	freq := w.detail / p.r
	baseFreq := fmt.Sprintf("%.4f", freq)
	if w.gas {
		baseFreq = fmt.Sprintf("%.4f %.4f", freq*.12, freq*1.6) // stretched noise reads as cloud bands
	} else if w.blend {
		baseFreq = fmt.Sprintf("%.4f %.4f", freq*.55, freq*1.3) // slightly stretched: dunes, canopy, ridges
	}
	a, b := rgb(w.surface), rgb(w.base)
	if w.gas || w.blend {
		// Blend the two colours by the noise value for soft, continuous surfaces;
		// the gain stretches the noise's narrow value range over both colours.
		doc.F(`<filter id="gx-tex%d" x="0" y="0" width="100%%" height="100%%" color-interpolation-filters="sRGB">`+
			`<feTurbulence type="fractalNoise" baseFrequency="%s" numOctaves="4" seed="%d"/>`+
			`<feColorMatrix values="%.3f 0 0 0 %.3f  %.3f 0 0 0 %.3f  %.3f 0 0 0 %.3f  0 0 0 0 1"/>`+
			`<feComposite in2="SourceGraphic" operator="in"/></filter>`,
			i, baseFreq, seed, (a[0]-b[0])*w.gain(), b[0]-(a[0]-b[0])*w.offset(), (a[1]-b[1])*w.gain(), b[1]-(a[1]-b[1])*w.offset(), (a[2]-b[2])*w.gain(), b[2]-(a[2]-b[2])*w.offset())
	} else {
		// Threshold the noise into sharp-edged continents, ice sheets or lava cracks.
		kind := "fractalNoise"
		if w.cracks {
			kind = "turbulence"
		}
		doc.F(`<filter id="gx-tex%d" x="0" y="0" width="100%%" height="100%%" color-interpolation-filters="sRGB">`+
			`<feTurbulence type="%s" baseFrequency="%s" numOctaves="4" seed="%d"/>`+
			`<feColorMatrix values="0 0 0 0 %.3f  0 0 0 0 %.3f  0 0 0 0 %.3f  %.1f 0 0 0 %.2f"/>`+
			`<feComposite in2="SourceGraphic" operator="in"/></filter>`,
			i, kind, baseFreq, seed, a[0], a[1], a[2], w.sharp, w.cut)
	}
	if w.clouds {
		doc.F(`<filter id="gx-cl%d" x="0" y="0" width="100%%" height="100%%" color-interpolation-filters="sRGB">`+
			`<feTurbulence type="fractalNoise" baseFrequency="%.4f %.4f" numOctaves="3" seed="%d"/>`+
			`<feColorMatrix values="0 0 0 0 1  0 0 0 0 1  0 0 0 0 1  3.2 0 0 0 -1.75"/>`+
			`<feComposite in2="SourceGraphic" operator="in"/></filter>`, i, freq*.8, freq*1.6, seed+31)
	}
	doc.F(`<radialGradient id="gx-atm%d"><stop offset=".78" stop-color="%s" stop-opacity="0"/><stop offset=".9" stop-color="%s" stop-opacity=".55"/>`+
		`<stop offset="1" stop-color="%s" stop-opacity="0"/></radialGradient>`, i, w.glow, w.glow, w.glow)
	doc.F(`<clipPath id="gx-clip%d"><circle cx="%.1f" cy="%.1f" r="%.1f"/></clipPath>`, i, p.x, p.y, p.r)
}

func drawPlanet(doc *svg.Doc, i int, p planet) {
	w := p.world
	ringRX, ringRY := p.r*2.05, p.r*.5
	opacity := 1.0
	if p.repo.Name == "" {
		opacity = .7
	}
	doc.F(`<g opacity="%.2f">`, opacity)
	if p.active { // turning holo ring beneath active systems
		doc.F(`<ellipse class="gx-spin" cx="%.1f" cy="%.1f" rx="%.1f" ry="%.1f" fill="none" stroke="%s" stroke-opacity=".55" stroke-dasharray="10 5 2 5"/>`,
			p.x, p.y+p.r*.15, p.r*1.75, p.r*1.75*.38, hudColor)
	}
	doc.F(`<circle cx="%.1f" cy="%.1f" r="%.1f" fill="url(#gx-atm%d)"/>`, p.x, p.y, p.r*1.28, i)
	if w.gas { // back half of the ring
		ringArc(doc, p, ringRX, ringRY, true)
	}
	doc.F(`<circle cx="%.1f" cy="%.1f" r="%.1f" fill="%s"/>`, p.x, p.y, p.r, w.base)
	doc.F(`<circle cx="%.1f" cy="%.1f" r="%.1f" fill="#000" filter="url(#gx-tex%d)"/>`, p.x, p.y, p.r, i)
	if w.cracks {
		doc.F(`<circle cx="%.1f" cy="%.1f" r="%.1f" fill="none" stroke="%s" stroke-opacity=".5" filter="url(#gx-blur2)" clip-path="url(#gx-clip%d)"/>`, p.x, p.y, p.r*.8, w.surface, i)
	}
	if w.clouds {
		doc.F(`<circle cx="%.1f" cy="%.1f" r="%.1f" fill="#000" filter="url(#gx-cl%d)" opacity=".85"/>`, p.x, p.y, p.r, i)
	}
	doc.F(`<circle cx="%.1f" cy="%.1f" r="%.1f" fill="url(#gx-shade)"/>`, p.x, p.y, p.r)
	if w.gas { // front half of the ring
		ringArc(doc, p, ringRX, ringRY, false)
	}
	for m := range p.moons {
		a := 2.3 + float64(m)*1.9
		mx, my := p.x+math.Cos(a)*p.r*1.75, p.y+math.Sin(a)*p.r*.95
		mr := math.Max(1.8, p.r*.13)
		doc.F(`<circle cx="%.1f" cy="%.1f" r="%.1f" fill="#cfc8e8"/><circle cx="%.1f" cy="%.1f" r="%.1f" fill="url(#gx-shade)"/>`, mx, my, mr, mx, my, mr)
	}
	doc.Raw(`</g>`)
	if p.repo.Name == "" { // fog of war over uncharted systems
		doc.F(`<circle cx="%.1f" cy="%.1f" r="%.1f" fill="url(#gx-fogfade)" filter="url(#gx-fog)" opacity=".8"/>`, p.x, p.y, p.r*2.1)
	}
}

// ringArc draws the back (behind the planet) or front half of a ring system.
func ringArc(doc *svg.Doc, p planet, rx, ry float64, back bool) {
	sweep := 1
	if back {
		sweep = 0
	}
	for k, sc := range []float64{1, .86, .74} {
		doc.F(`<path d="M%.1f %.1fA%.1f %.1f 0 0 %d %.1f %.1f" fill="none" stroke="%s" stroke-width="%.1f" stroke-opacity="%.2f" transform="rotate(-14 %.1f %.1f)"/>`,
			p.x-rx*sc, p.y, rx*sc, ry*sc, sweep, p.x+rx*sc, p.y, p.world.ring, 2.4-float64(k)*.6, .75-float64(k)*.18, p.x, p.y)
	}
}

type label struct {
	x, y   float64 // top-left of the name plate
	w      float64
	px, py float64 // where the leader line meets the plate
}

// placeLabels puts each name plate on the side of its planet that collides
// least with planets, earlier plates and the header and footer.
func placeLabels(ps []planet) []label {
	const plateH, charW = 30.0, 6.2
	type box struct{ x0, y0, x1, y1 float64 }
	overlap := func(a, b box) float64 {
		w := math.Min(a.x1, b.x1) - math.Max(a.x0, b.x0)
		h := math.Min(a.y1, b.y1) - math.Max(a.y0, b.y0)
		if w <= 0 || h <= 0 {
			return 0
		}
		return w * h
	}
	var taken []box
	for _, p := range ps {
		taken = append(taken, box{p.x - p.r*1.4, p.y - p.r*1.2, p.x + p.r*1.4, p.y + p.r*1.2})
	}
	bounds := box{14, 56, width - 14, height - 70}
	out := make([]label, len(ps))
	for i, p := range ps {
		w := round(float64(max(len([]rune(p.label)), len([]rune(p.detail))))*charW + 16)
		gap := p.r*1.35 + 14
		cands := []label{
			{x: p.x + gap, y: p.y - plateH/2, px: p.x + gap, py: p.y},
			{x: p.x - gap - w, y: p.y - plateH/2, px: p.x - gap, py: p.y},
			{x: p.x - w/2, y: p.y + p.r*1.3 + 10, px: p.x, py: p.y + p.r*1.3 + 10},
			{x: p.x - w/2, y: p.y - p.r*1.3 - 10 - plateH, px: p.x, py: p.y - p.r*1.3 - 10},
		}
		best, bestCost := 0, math.Inf(1)
		for k, c := range cands {
			b := box{c.x, c.y, c.x + w, c.y + plateH}
			cost := 0.0
			for j, t := range taken {
				if j != i {
					cost += overlap(b, t)
				}
			}
			cost += (w*plateH - overlap(b, bounds)) * 4
			if cost < bestCost-.5 { // ties keep the earlier, preferred side
				best, bestCost = k, cost
			}
		}
		c := cands[best]
		c.w = w
		out[i] = c
		taken = append(taken, box{c.x, c.y, c.x + w, c.y + plateH})
	}
	return out
}

func drawLabel(doc *svg.Doc, p planet, l label) {
	accent := card.LanguageColor(p.repo.Language)
	nameColor := textColor
	if p.repo.Name == "" {
		accent, nameColor = mutedColor, "#b4acd9"
	}
	edgeX, edgeY := p.x, p.y // the leader line starts on the planet's rim, pointing at the plate
	if d := math.Hypot(l.px-p.x, l.py-p.y); d > 0 {
		edgeX, edgeY = p.x+(l.px-p.x)/d*p.r*1.1, p.y+(l.py-p.y)/d*p.r*1.1
	}
	doc.F(`<path d="M%.1f %.1fL%.1f %.1f" stroke="%s" stroke-opacity=".6" stroke-width=".8"/>`, edgeX, edgeY, l.px, l.py, accent)
	doc.F(`<rect x="%.1f" y="%.1f" width="%.1f" height="30" rx="3" fill="#0a0820" fill-opacity=".78" stroke="%s" stroke-opacity=".55" stroke-width=".8"/>`, l.x, l.y, l.w, accent)
	doc.F(`<rect x="%.1f" y="%.1f" width="2.5" height="30" fill="%s"/>`, l.x, l.y, accent)
	doc.F(`<text x="%.1f" y="%.1f" fill="%s" font-size="10" font-weight="600">%s</text>`, l.x+9, l.y+13, nameColor, svg.Esc(p.label))
	doc.F(`<text x="%.1f" y="%.1f" fill="%s" font-size="8.5">%s</text>`, l.x+9, l.y+24, mutedColor, svg.Esc(p.detail))
}

// hud draws the header, the frame's corner brackets and the stats panel.
func hud(doc *svg.Doc, name string, stats []svg.Stat, beyond int) {
	for _, c := range [][4]float64{{10, 10, 1, 1}, {width - 10, 10, -1, 1}, {10, height - 10, 1, -1}, {width - 10, height - 10, -1, -1}} {
		doc.F(`<path d="M%.0f %.0fv%.0fM%.0f %.0fh%.0f" stroke="%s" stroke-opacity=".6" stroke-width="1.5" fill="none"/>`,
			c[0], c[1], 16*c[3], c[0], c[1], 16*c[2], hudColor)
	}
	doc.F(`<text x="26" y="36" fill="%s" font-size="15" font-weight="700" letter-spacing="3">GALAXY</text>`, hudColor)
	doc.F(`<text x="118" y="36" fill="%s" font-size="11">%s · 12 months · every repository is a world</text>`, mutedColor, svg.Esc(name))
	doc.F(`<rect x="18" y="%d" width="%d" height="50" rx="4" fill="#0a0820" fill-opacity=".72" stroke="%s" stroke-opacity=".3"/>`, height-66, width-36, hudColor)
	step := float64(width-36) / float64(len(stats))
	for i, s := range stats {
		x := 18 + float64(i)*step
		if i > 0 {
			doc.F(`<path d="M%.0f %dv30" stroke="%s" stroke-opacity=".25"/>`, x, height-56, hudColor)
		}
		doc.F(`<text x="%.0f" y="%d" fill="%s" font-size="17" font-weight="700">%s</text>`, x+16, height-38, textColor, svg.Esc(s.Value))
		label := s.Label
		if i == 0 && beyond > 0 {
			label = fmt.Sprintf("systems (%d beyond the map)", beyond)
		}
		doc.F(`<text x="%.0f" y="%d" fill="%s" font-size="9.5" letter-spacing=".5">%s</text>`, x+16, height-24, mutedColor, svg.Esc(strings.ToUpper(label)))
	}
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

// round snaps a value to a hundredth, so floating-point differences between
// CPU architectures cannot change the layout.
func round(v float64) float64 { return math.Round(v*100) / 100 }
