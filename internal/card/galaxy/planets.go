package galaxy

import (
	"fmt"
	"math"

	"github.com/msch128/galaxy-stats/internal/card"
	"github.com/msch128/galaxy-stats/internal/model"
	"github.com/msch128/galaxy-stats/internal/svg"
)

type ringKind int

const (
	noRing ringKind = iota
	thinRing
	broadRing  // Saturn-like: wide bands with a dark gap
	doubleRing // two separate narrow rings
)

// satellite is a moon or a space station on an orbit around its planet.
type satellite struct {
	orbit   float64 // semi-major axis
	size    float64
	period  float64 // seconds per orbit
	phase   float64 // 0..1, where on the orbit it starts
	retro   bool
	station bool
}

// look is everything that varies between planets beyond their world type,
// derived from the planet's name so it stays the same from run to run.
type look struct {
	ring       ringKind
	ringTilt   float64 // degrees
	ringRatio  float64 // minor/major axis of rings and orbits
	ringColor  string
	spin       float64 // seconds for the surface to pass by once
	cloudSpin  float64
	retrograde bool
	storm      bool
	lights     bool
	holo       float64 // seconds per turn of the holo ring
	holoRev    bool
	satellites []satellite
}

func lookFor(s *model.Snapshot, p planet, station bool) look {
	rnd := card.Rand(s, "planet-look/"+p.label)
	w := p.world
	l := look{
		ringTilt:  -26 + rnd.Float64()*44,
		ringRatio: .2 + rnd.Float64()*.15,
		ringColor: w.ring,
		spin:      45 + rnd.Float64()*75,
		holo:      12 + rnd.Float64()*20,
		holoRev:   rnd.Float64() < .35,
	}
	if l.ringColor == "" {
		l.ringColor = w.glow
	}
	l.retrograde = rnd.Float64() < .2
	l.cloudSpin = l.spin * (.5 + rnd.Float64()*.25)
	ringRoll := rnd.Float64()
	switch {
	case w.gas:
		l.ring = []ringKind{broadRing, broadRing, doubleRing, thinRing}[int(ringRoll*4)]
		l.storm = rnd.Float64() < .7
	case w.icy && ringRoll < .5, ringRoll < .12:
		l.ring = thinRing
	}
	l.lights = p.active && w.habitable

	orbit := p.r * 1.65
	if l.ring != noRing {
		orbit = p.r * 2.45 // clear of the rings
	}
	for k := range p.moons {
		l.satellites = append(l.satellites, satellite{
			orbit:  orbit + float64(k)*p.r*.42,
			size:   math.Max(1.6, p.r*(.08+rnd.Float64()*.07)),
			period: 10 + float64(k)*7 + rnd.Float64()*8,
			phase:  rnd.Float64(),
			retro:  rnd.Float64() < .15,
		})
	}
	if station {
		l.satellites = append(l.satellites, satellite{orbit: orbit + float64(p.moons)*p.r*.42 + 8, size: 3.4, period: 26, phase: rnd.Float64(), station: true})
	}
	return l
}

// surfaceTile returns the width of a planet's seamless surface tile: two
// diameters, so one pass shows the planet's whole circumference.
func surfaceTile(p planet) float64 { return round(4 * p.r) }

// planetDefs defines a planet's seamless surface and cloud tiles, their
// spin animations, its atmosphere and the clip paths that put satellites
// behind or in front of it.
func planetDefs(doc *svg.Doc, s *model.Snapshot, i int, p planet, l look) {
	w := p.world
	seed := card.Rand(s, fmt.Sprint("planet-", i)).IntN(900) + 1
	freq := w.detail / p.r
	tile, h := surfaceTile(p), round(2*p.r)

	baseFreq := fmt.Sprintf("%.4f", freq)
	switch {
	case w.gas:
		baseFreq = fmt.Sprintf("%.4f %.4f", freq*.12, freq*1.6) // stretched noise reads as cloud bands
	case w.blend:
		baseFreq = fmt.Sprintf("%.4f %.4f", freq*.55, freq*1.3) // slightly stretched: dunes, canopy, ridges
	}
	a, b := rgb(w.surface), rgb(w.base)
	if w.gas || w.blend {
		// Blend the two colours by the noise value for soft, continuous
		// surfaces; the gain stretches the noise's narrow range over both.
		doc.F(`<filter id="gx-tex%d" x="0" y="0" width="100%%" height="100%%" color-interpolation-filters="sRGB">`+
			`<feTurbulence type="fractalNoise" baseFrequency="%s" numOctaves="4" seed="%d" stitchTiles="stitch"/>`+
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
			`<feTurbulence type="%s" baseFrequency="%s" numOctaves="4" seed="%d" stitchTiles="stitch"/>`+
			`<feColorMatrix values="0 0 0 0 %.3f  0 0 0 0 %.3f  0 0 0 0 %.3f  %.1f 0 0 0 %.2f"/>`+
			`<feComposite in2="SourceGraphic" operator="in"/></filter>`,
			i, kind, baseFreq, seed, a[0], a[1], a[2], w.sharp, w.cut)
	}
	doc.F(`<g id="gx-surf%d"><rect width="%.1f" height="%.1f" fill="%s"/><rect width="%.1f" height="%.1f" filter="url(#gx-tex%d)"/>`, i, tile, h, w.base, tile, h, i)
	if l.storm { // a great storm, carried along with the bands
		doc.F(`<ellipse cx="%.1f" cy="%.1f" rx="%.1f" ry="%.1f" fill="#d9774f" fill-opacity=".85"/><ellipse cx="%.1f" cy="%.1f" rx="%.1f" ry="%.1f" fill="#f2b38a" fill-opacity=".7"/>`,
			tile*.3, h*.63, p.r*.34, p.r*.16, tile*.3, h*.63, p.r*.18, p.r*.08)
	}
	doc.Raw(`</g>`)
	if w.clouds {
		doc.F(`<filter id="gx-cl%d" x="0" y="0" width="100%%" height="100%%" color-interpolation-filters="sRGB">`+
			`<feTurbulence type="fractalNoise" baseFrequency="%.4f %.4f" numOctaves="3" seed="%d" stitchTiles="stitch"/>`+
			`<feColorMatrix values="0 0 0 0 1  0 0 0 0 1  0 0 0 0 1  3.2 0 0 0 -1.75"/>`+
			`<feComposite in2="SourceGraphic" operator="in"/></filter>`, i, freq*.8, freq*1.6, seed+31)
		doc.F(`<g id="gx-cloud%d"><rect width="%.1f" height="%.1f" filter="url(#gx-cl%d)"/></g>`, i, tile, h, i)
	}

	// The surface group slides one tile to the left (or right, for retrograde
	// spin) and starts over, which looks like the planet turning.
	dir := "normal"
	if l.retrograde {
		dir = "reverse"
	}
	doc.F(`<style>.gx-t%d{animation:gx-t%d %.0fs linear infinite %s}.gx-c%d{animation:gx-t%d %.0fs linear infinite %s}@keyframes gx-t%d{to{transform:translateX(-%.1fpx)}}</style>`,
		i, i, l.spin, dir, i, i, l.cloudSpin, dir, i, tile)

	doc.F(`<radialGradient id="gx-atm%d"><stop offset=".78" stop-color="%s" stop-opacity="0"/><stop offset=".9" stop-color="%s" stop-opacity=".55"/>`+
		`<stop offset="1" stop-color="%s" stop-opacity="0"/></radialGradient>`, i, w.glow, w.glow, w.glow)
	doc.F(`<clipPath id="gx-clip%d"><circle cx="%.1f" cy="%.1f" r="%.1f"/></clipPath>`, i, p.x, p.y, p.r)
	const far = 400.0 // clip half-planes, in the tilted orbital frame
	doc.F(`<clipPath id="gx-back%d"><rect x="%.1f" y="%.1f" width="%.0f" height="%.0f"/></clipPath>`, i, p.x-far, p.y-far, 2*far, far)
	doc.F(`<clipPath id="gx-front%d"><rect x="%.1f" y="%.1f" width="%.0f" height="%.0f"/></clipPath>`, i, p.x-far, p.y, 2*far, far)
}

func drawPlanet(doc *svg.Doc, i int, p planet, l look) {
	w := p.world
	opacity := 1.0
	if p.repo.Name == "" {
		opacity = .72
	}
	doc.F(`<g opacity="%.2f">`, opacity)
	if p.active { // a turning holo ring beneath systems pushed in the last two weeks
		dir := "normal"
		if l.holoRev {
			dir = "reverse"
		}
		doc.F(`<ellipse class="gx-spin" style="animation-duration:%.0fs;animation-direction:%s" cx="%.1f" cy="%.1f" rx="%.1f" ry="%.1f" fill="none" stroke="%s" stroke-opacity=".55" stroke-dasharray="10 5 2 5"/>`,
			l.holo, dir, p.x, p.y+p.r*.15, p.r*1.75, p.r*1.75*.38, hudColor)
	}
	doc.F(`<circle cx="%.1f" cy="%.1f" r="%.1f" fill="url(#gx-atm%d)"/>`, p.x, p.y, p.r*1.28, i)
	rings(doc, p, l, true)
	satellites(doc, i, p, l, true)

	tile := surfaceTile(p)
	doc.F(`<g clip-path="url(#gx-clip%d)">`, i)
	doc.F(`<g class="gx-turn gx-t%d"><use href="#gx-surf%d" x="%.1f" y="%.1f"/><use href="#gx-surf%d" x="%.1f" y="%.1f"/></g>`, i, i, p.x-p.r, p.y-p.r, i, p.x-p.r+tile, p.y-p.r)
	if w.cracks {
		doc.F(`<circle cx="%.1f" cy="%.1f" r="%.1f" fill="none" stroke="%s" stroke-opacity=".5" filter="url(#gx-blur2)"/>`, p.x, p.y, p.r*.8, w.surface)
	}
	if w.clouds {
		doc.F(`<g class="gx-turn gx-c%d" opacity=".85"><use href="#gx-cloud%d" x="%.1f" y="%.1f"/><use href="#gx-cloud%d" x="%.1f" y="%.1f"/></g>`, i, i, p.x-p.r, p.y-p.r, i, p.x-p.r+tile, p.y-p.r)
	}
	if w.caps {
		doc.F(`<ellipse cx="%.1f" cy="%.1f" rx="%.1f" ry="%.1f" fill="#f4f8ff" fill-opacity=".8" filter="url(#gx-blur1)"/>`, p.x, p.y-p.r*.95, p.r*.55, p.r*.2)
		doc.F(`<ellipse cx="%.1f" cy="%.1f" rx="%.1f" ry="%.1f" fill="#f4f8ff" fill-opacity=".6" filter="url(#gx-blur1)"/>`, p.x, p.y+p.r*.97, p.r*.4, p.r*.14)
	}
	doc.Raw(`</g>`)
	doc.F(`<circle cx="%.1f" cy="%.1f" r="%.1f" fill="url(#gx-shade)"/>`, p.x, p.y, p.r)
	if l.lights { // city lights on the night side of busy, habitable worlds
		for k := range 14 {
			a := .15 + float64(k)*.09 + .04*math.Sin(float64(k*7))
			d := p.r * (.45 + .4*math.Abs(math.Sin(float64(k)*2.3)))
			doc.F(`<circle cx="%.1f" cy="%.1f" r="%.1f" fill="#ffd27a" fill-opacity="%.2f"/>`, p.x+math.Cos(a)*d, p.y+math.Sin(a)*d, .5+.4*math.Abs(math.Cos(float64(k))), .55+.35*math.Abs(math.Sin(float64(k))))
		}
	}
	rings(doc, p, l, false)
	satellites(doc, i, p, l, false)
	doc.Raw(`</g>`)
	if p.repo.Name == "" { // fog of war over uncharted systems
		doc.F(`<circle cx="%.1f" cy="%.1f" r="%.1f" fill="url(#gx-fogfade)" filter="url(#gx-fog)" opacity=".8"/>`, p.x, p.y, p.r*2.1)
	}
}

// rings draws the back half (behind the planet) or the front half of a ring
// system, as concentric arcs of the planet's ring style.
func rings(doc *svg.Doc, p planet, l look, back bool) {
	if l.ring == noRing {
		return
	}
	type band struct{ scale, width, opacity float64 }
	var bands []band
	switch l.ring {
	case thinRing:
		bands = []band{{1.7, 1.3, .7}, {1.6, .6, .4}}
	case doubleRing:
		bands = []band{{2.15, 1.6, .7}, {2.05, .7, .45}, {1.62, 1.8, .6}}
	case broadRing:
		// The missing band between 1.98 and 1.78 is the dark division.
		bands = []band{{2.25, 2.2, .5}, {2.12, 2.6, .7}, {1.98, 2.6, .78}, {1.78, 2.2, .7}, {1.64, 2.4, .62}, {1.5, 2, .4}, {1.38, 1.4, .25}}
	}
	sweep := 1
	if back {
		sweep = 0
	}
	for _, b := range bands {
		rx, ry := p.r*b.scale, p.r*b.scale*l.ringRatio
		doc.F(`<path d="M%.1f %.1fA%.1f %.1f 0 0 %d %.1f %.1f" fill="none" stroke="%s" stroke-width="%.1f" stroke-opacity="%.2f" transform="rotate(%.1f %.1f %.1f)"/>`,
			p.x-rx, p.y, rx, ry, sweep, p.x+rx, p.y, l.ringColor, b.width, b.opacity, l.ringTilt, p.x, p.y)
	}
}

// satellites draws the moons and station on the far side of their orbit (back)
// or the near side. Both halves run the same motion, so each satellite passes
// behind its planet and comes round in front.
func satellites(doc *svg.Doc, i int, p planet, l look, back bool) {
	if len(l.satellites) == 0 {
		return
	}
	half := "front"
	if back {
		half = "back"
	}
	doc.F(`<g transform="rotate(%.1f %.1f %.1f)"><g clip-path="url(#gx-%s%d)">`, l.ringTilt*.6, p.x, p.y, half, i)
	for _, sat := range l.satellites {
		rx, ry := sat.orbit, sat.orbit*l.ringRatio*1.15
		orbit := fmt.Sprintf("M%.1f %.1fA%.1f %.1f 0 1 1 %.1f %.1fA%.1f %.1f 0 1 1 %.1f %.1f", p.x+rx, p.y, rx, ry, p.x-rx, p.y, rx, ry, p.x+rx, p.y)
		if back { // the orbit's path, faintly
			doc.F(`<path d="%s" fill="none" stroke="%s" stroke-opacity=".12" stroke-width=".6"/>`, orbit, hudColor)
		}
		keys := ""
		if sat.retro {
			keys = ` keyPoints="1;0" keyTimes="0;1" calcMode="linear"`
		}
		doc.F(`<g><animateMotion dur="%.1fs" begin="-%.1fs" repeatCount="indefinite" path="%s"%s/>`, sat.period, sat.phase*sat.period, orbit, keys)
		if sat.station {
			station(doc, sat.size)
		} else {
			doc.F(`<circle r="%.1f" fill="#cfc8e8"/><circle r="%.1f" fill="url(#gx-shade)"/>`, sat.size, sat.size)
		}
		doc.Raw(`</g>`)
	}
	doc.Raw(`</g></g>`)
}

// station draws a small orbital station around the origin.
func station(doc *svg.Doc, s float64) {
	doc.F(`<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="#7fb8ff" fill-opacity=".85"/>`, -s*2.6, -s*.35, s*1.4, s*.7)
	doc.F(`<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="#7fb8ff" fill-opacity=".85"/>`, s*1.2, -s*.35, s*1.4, s*.7)
	doc.F(`<circle r="%.1f" fill="none" stroke="#e6e1ff" stroke-width="1"/><circle r="%.1f" fill="#fffbe6"/>`, s, s*.38)
	doc.F(`<circle r="%.1f" fill="#ffd27a" fill-opacity=".5" filter="url(#gx-blur1)"/>`, s*.9)
}
