// Package forecast reports the last two weeks of activity as a weather
// forecast: commits per day as the temperature, late-night work as the
// chance of night commits, the busiest languages as an incoming front.
package forecast

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/msch128/galaxy-stats/internal/card"
	"github.com/msch128/galaxy-stats/internal/model"
	"github.com/msch128/galaxy-stats/internal/svg"
)

const (
	width  = 720
	height = 300
	window = 14
	sans   = "-apple-system,Segoe UI,Helvetica,Arial,sans-serif"
	font   = "ui-monospace,SFMono-Regular,Menlo,Consolas,monospace"
)

// Card is the forecast card.
type Card struct{}

func (Card) ID() string { return "forecast" }

type weather int

const (
	calm weather = iota
	cloudy
	partly
	sunny
	clearNight
)

type report struct {
	perDay    float64
	night     float64 // share of commits between 22:00 and 05:00
	peakHour  int
	langs     []string
	langShare []int
	storm     string // busiest repository, or a description of a private one
	trend     float64
	sky       weather
}

func (Card) Render(s *model.Snapshot, o card.Options) ([]byte, error) {
	r := analyse(s)
	name := o.Name(s)
	desc := fmt.Sprintf("Commit forecast for %s: %.0f commits per day over the last %d days.", name, r.perDay, window)
	doc := svg.New(width, height, "Commit forecast for "+name, desc, font)

	top, bottom := "#2f7de1", "#0b2a5b"
	if r.sky == clearNight {
		top, bottom = "#1a1f4d", "#05071a"
	}
	doc.F(`<defs><linearGradient id="fc-sky" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="%s"/><stop offset="1" stop-color="%s"/></linearGradient></defs>`, top, bottom)
	doc.Raw(`<style>.fc-drift{animation:fc-drift 7s ease-in-out infinite alternate}@keyframes fc-drift{to{transform:translateX(8px)}}` +
		`.fc-glow{animation:fc-glow 4s ease-in-out infinite}@keyframes fc-glow{50%{opacity:.7}}` +
		`@media (prefers-reduced-motion:reduce){.fc-drift,.fc-glow{animation:none}}</style>`)
	doc.F(`<rect width="%d" height="%d" rx="10" fill="url(#fc-sky)"/>`, width, height)
	doc.F(`<text x="24" y="34" fill="#fff" font-size="15" font-weight="700" letter-spacing="2">COMMIT FORECAST</text>`)
	doc.F(`<text x="222" y="34" fill="#a9c7ff" font-size="11">%s · issued %s · %s</text>`, svg.Esc(name), s.Now().Format("02 Jan 2006"), svg.Esc(s.Location().String()))

	icon(doc, r.sky, 96, 136)
	doc.F(`<text x="190" y="128" fill="#fff" font-size="44" font-weight="700" font-family="%s">%.0f°c</text>`, sans, r.perDay)
	doc.F(`<text x="192" y="150" fill="#a9c7ff" font-size="11">commits per day · %d-day average · %s</text>`, window, skyName(r.sky))

	for i, line := range r.lines() {
		doc.F(`<text x="190" y="%d" fill="#eaf2ff" font-size="12.5" font-family="%s">• %s</text>`, 186+i*22, sans, svg.Esc(line))
	}
	return doc.Bytes(), nil
}

func analyse(s *model.Snapshot) report {
	var r report
	loc := s.Location()
	hours := make([]int, 24)
	for _, t := range s.CommitTimes {
		h := t.In(loc).Hour()
		hours[h]++
		if h >= 22 || h < 5 {
			r.night++
		}
	}
	if n := len(s.CommitTimes); n > 0 {
		r.perDay = float64(n) / window
		r.night /= float64(n)
		for h, c := range hours {
			if c > hours[r.peakHour] {
				r.peakHour = h
			}
		}
	}

	now := s.Now()
	byLang := map[string]int{}
	busiest, busiestCount := model.Repo{}, 0
	for _, repo := range s.Repos {
		n := 0
		for _, d := range repo.Days {
			if now.Sub(d.Time()) < window*24*time.Hour {
				n += d.Count
			}
		}
		if n == 0 {
			continue
		}
		if repo.Language != "" {
			byLang[repo.Language] += n
		}
		if n > busiestCount {
			busiest, busiestCount = repo, n
		}
	}
	total := 0
	for lang, n := range byLang {
		r.langs = append(r.langs, lang)
		total += n
	}
	sort.Slice(r.langs, func(i, j int) bool {
		if byLang[r.langs[i]] != byLang[r.langs[j]] {
			return byLang[r.langs[i]] > byLang[r.langs[j]]
		}
		return r.langs[i] < r.langs[j]
	})
	r.langs = r.langs[:min(2, len(r.langs))]
	for _, l := range r.langs {
		r.langShare = append(r.langShare, byLang[l]*100/total)
	}
	switch {
	case busiestCount == 0:
	case busiest.Name == "":
		r.storm = "a classified project"
	default:
		r.storm = busiest.Name
	}

	recent, before := 0, 0
	for _, d := range s.LastDays(window) {
		if now.Sub(d.Time()) < 7*24*time.Hour {
			recent += d.Count
		} else {
			before += d.Count
		}
	}
	if before > 0 {
		r.trend = float64(recent-before) / float64(before)
	}

	switch {
	case len(s.CommitTimes) > 0 && r.night >= .4:
		r.sky = clearNight
	case r.perDay >= 8:
		r.sky = sunny
	case r.perDay >= 2:
		r.sky = partly
	case r.perDay > 0:
		r.sky = cloudy
	}
	return r
}

func (r report) lines() []string {
	if r.perDay == 0 {
		return []string{"Calm conditions: no commits in the last two weeks.", "Outlook: the next push may come at any moment."}
	}
	out := []string{
		fmt.Sprintf("%s commit activity, peaking around %02d:00.", intensity(r.perDay), r.peakHour),
		fmt.Sprintf("%.0f%% chance of commits after 22:00.%s", r.night*100, nightNote(r.night)),
	}
	switch len(r.langs) {
	case 1:
		out = append(out, fmt.Sprintf("Language front: %s holding steady.", r.langs[0]))
	case 2:
		out = append(out, fmt.Sprintf("Language front: %s and %s moving in (%d%% / %d%%).", r.langs[0], r.langs[1], r.langShare[0], r.langShare[1]))
	}
	if r.storm != "" {
		out = append(out, "Storm cell over "+r.storm+".")
	}
	switch {
	case r.trend > .15:
		out = append(out, fmt.Sprintf("Pressure rising: +%.0f%% on the week before.", r.trend*100))
	case r.trend < -.15:
		out = append(out, fmt.Sprintf("Easing off: %.0f%% on the week before.", r.trend*100))
	}
	return out[:min(len(out), 5)]
}

func intensity(perDay float64) string {
	switch {
	case perDay >= 20:
		return "Extreme"
	case perDay >= 8:
		return "Heavy"
	case perDay >= 2:
		return "Steady"
	default:
		return "Light"
	}
}

func nightNote(share float64) string {
	if share >= .25 {
		return " Night owls advised."
	}
	return ""
}

func skyName(w weather) string {
	return [...]string{"calm", "cloudy", "partly sunny", "sunny", "clear night"}[w]
}

// icon draws the weather symbol centred on (x, y).
func icon(doc *svg.Doc, w weather, x, y float64) {
	cloud := func(dx, dy, scale float64, color string) {
		doc.F(`<path class="fc-drift" transform="translate(%.1f %.1f) scale(%.2f)" d="M-30 18a20 20 0 0 1 4-39a26 26 0 0 1 48 6a17 17 0 0 1 2 33z" fill="%s"/>`, x+dx, y+dy, scale, color)
	}
	sun := func(dx, dy float64) {
		doc.F(`<g class="fc-glow"><circle cx="%.1f" cy="%.1f" r="28" fill="#ffd34d"/><g stroke="#ffd34d" stroke-width="4" stroke-linecap="round">`, x+dx, y+dy)
		for a := range 8 {
			ang := float64(a) * math.Pi / 4
			doc.F(`<path d="M%.1f %.1fL%.1f %.1f"/>`, x+dx+38*math.Cos(ang), y+dy+38*math.Sin(ang), x+dx+48*math.Cos(ang), y+dy+48*math.Sin(ang))
		}
		doc.Raw(`</g></g>`)
	}
	switch w {
	case sunny:
		sun(0, 0)
	case partly:
		sun(-8, -12)
		cloud(10, 22, 1, "#eaf2ff")
	case cloudy:
		cloud(-10, -4, .8, "#9fb3d1")
		cloud(8, 14, 1, "#dfe8f5")
	case clearNight:
		doc.F(`<path class="fc-glow" d="M%.1f %.1fa32 32 0 1 0 30 42a26 26 0 1 1-30-42z" fill="#f4ecc9"/>`, x-6, y-34)
		for _, st := range [][2]float64{{34, -30}, {46, 4}, {-44, -26}, {22, 30}} {
			doc.F(`<circle cx="%.1f" cy="%.1f" r="1.8" fill="#fff"/>`, x+st[0], y+st[1])
		}
	default:
		for i := range 3 {
			doc.F(`<path d="M%.1f %.1fh60" stroke="#c7d6ea" stroke-width="5" stroke-linecap="round" opacity="%.2f"/>`, x-30+float64(i%2)*8, y-14+float64(i)*14, .9-float64(i)*.2)
		}
	}
}
