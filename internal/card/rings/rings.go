// Package rings draws a user's years on GitHub as the growth rings of a tree:
// one ring per calendar year, its width the year's contributions.
package rings

import (
	"fmt"
	"math"
	"strings"

	"github.com/msch128/galaxy-stats/internal/card"
	"github.com/msch128/galaxy-stats/internal/model"
	"github.com/msch128/galaxy-stats/internal/svg"
)

const (
	width     = 720
	cx        = 200.0
	font      = "ui-monospace,SFMono-Regular,Menlo,Consolas,monospace"
	headColor = "#f0c27b"
	textColor = "#c7a77e"
	lineColor = "#9a7b53"
	barkColor = "#4a3019"
	backColor = "#1a120b"
	maxRadius = 122.0
	maxLabels = 8
)

var woodTones = []string{"#e8c48f", "#dcb47c", "#d0a56b", "#c4955b", "#b8864d", "#ab7841"}

// Card is the rings card.
type Card struct{}

func (Card) ID() string { return "rings" }

func (Card) Render(s *model.Snapshot, o card.Options) ([]byte, error) {
	years := s.Years
	if len(years) == 0 { // older snapshots: fall back to the calendar's year
		total := 0
		for _, d := range s.Calendar {
			total += d.Count
		}
		years = []model.Year{{Year: s.Now().Year(), Count: total}}
	}

	peak, total, best := 1, 0, years[0]
	for _, y := range years {
		peak = max(peak, y.Count)
		total += y.Count
		if y.Count > best.Count {
			best = y
		}
	}
	// Ring width grows with the logarithm of the count, so a quiet year is a
	// narrow ring rather than a missing one. Widths are then scaled to fit.
	widths := make([]float64, len(years))
	sum := 0.0
	for i, y := range years {
		widths[i] = 4 + 26*math.Log1p(float64(y.Count))/math.Log1p(float64(peak))
		sum += widths[i]
	}
	scale := math.Min(1, (maxRadius-10)/sum)

	shown := years[max(0, len(years)-maxLabels):]
	height := max(340, 100+len(shown)*40)
	cy := float64(height)/2 + 16
	name := o.Name(s)
	desc := fmt.Sprintf("Growth rings of %s: %d years on GitHub with %d contributions, the strongest in %d.", name, len(years), total, best.Year)
	doc := svg.New(width, height, "Growth rings of "+name, desc, font)
	doc.F(`<rect width="%d" height="%d" rx="10" fill="%s"/>`, width, height, backColor)
	doc.Header("RINGS", name+" · one ring per year on GitHub", headColor, lineColor)

	rnd := card.Rand(s, "rings")
	phase := rnd.Float64() * 2 * math.Pi
	radii := make([]float64, len(years)+1)
	radii[0] = 6
	for i, w := range widths {
		radii[i+1] = radii[i] + w*scale
	}
	outer := radii[len(radii)-1]
	doc.F(`<path d="%s" fill="%s"/>`, ring(cx, cy, outer+9, phase), barkColor)
	doc.F(`<path d="%s" fill="none" stroke="#2e1d0f" stroke-width="2" stroke-dasharray="1 5"/>`, ring(cx, cy, outer+6, phase))
	for i := len(years) - 1; i >= 0; i-- {
		doc.F(`<path d="%s" fill="%s" stroke="#7a5230" stroke-width="1.1"/>`, ring(cx, cy, radii[i+1], phase), woodTones[i%len(woodTones)])
	}
	doc.F(`<circle cx="%.0f" cy="%.0f" r="4" fill="#7a5230"/>`, cx, cy)
	doc.F(`<path d="M%.0f %.0fL%.0f %.0f" stroke="#5a3b22" stroke-width="1.2" stroke-opacity=".7"/>`, cx+radii[1], cy+3, cx+outer, cy+18) // a heartwood crack

	top := cy - float64(len(shown)-1)*20 + 4
	for k, y := range shown {
		i := len(years) - len(shown) + k
		mid := (radii[i] + radii[i+1]) / 2
		ly := top + float64(k)*40
		ax, ay := cx+mid*math.Cos(-.35+float64(k)*.12), cy+mid*math.Sin(-.35+float64(k)*.12)
		doc.F(`<path d="M%.1f %.1fL%d %.1f" stroke="%s" stroke-width=".7" fill="none"/><circle cx="%.1f" cy="%.1f" r="1.8" fill="%s"/>`, ax, ay, 430, ly-4, lineColor, ax, ay, headColor)
		color := textColor
		if y == best {
			color = headColor
		}
		doc.F(`<text x="440" y="%.0f" fill="%s" font-size="14" font-weight="700">%d</text>`, ly, headColor, y.Year)
		doc.F(`<text x="490" y="%.0f" fill="%s" font-size="11">%s contributions</text>`, ly, color, thousands(y.Count))
	}
	doc.F(`<text x="440" y="%d" fill="%s" font-size="10">%d years · %s contributions in all</text>`, height-18, lineColor, len(years), thousands(total))
	return doc.Bytes(), nil
}

// ring returns a slightly wobbly closed circle, like a real growth ring.
func ring(x, y, r, phase float64) string {
	const steps = 72
	var b strings.Builder
	for t := range steps {
		a := float64(t) / steps * 2 * math.Pi
		rr := r * (1 + .025*math.Sin(5*a+phase) + .015*math.Sin(9*a+phase*2))
		cmd := "L"
		if t == 0 {
			cmd = "M"
		}
		fmt.Fprintf(&b, "%s%.1f %.1f", cmd, x+rr*math.Cos(a), y+rr*math.Sin(a))
	}
	b.WriteString("Z")
	return b.String()
}

func thousands(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
