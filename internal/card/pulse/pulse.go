// Package pulse draws recent activity as a heart monitor: one heartbeat per
// day, its height the day's contributions, with a sweep line running across
// the screen.
package pulse

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
	height    = 300
	maxDays   = 60
	minDays   = 21
	screenX   = 20
	screenY   = 52
	screenW   = width - 40
	screenH   = 160
	baseline  = screenY + screenH*0.62
	font      = "ui-monospace,SFMono-Regular,Menlo,Consolas,monospace"
	traceCol  = "#39ff8f"
	gridCol   = "#0f3d26"
	labelCol  = "#2f8f5c"
	valueCol  = "#c9ffe0"
	backColor = "#03100a"
)

// Card is the pulse card.
type Card struct{}

func (Card) ID() string { return "pulse" }

func (Card) Render(s *model.Snapshot, o card.Options) ([]byte, error) {
	days := window(s.LastDays(maxDays))
	peak, beats := 0, 0
	for _, d := range days {
		peak = max(peak, d.Count)
		if d.Count > 0 {
			beats++
		}
	}
	last30 := days[max(0, len(days)-30):]
	sum := 0
	for _, d := range last30 {
		sum += d.Count
	}
	bpm := 0.0
	if len(last30) > 0 {
		bpm = float64(sum) / float64(len(last30))
	}
	streak := 0
	for i := len(days) - 1; i >= 0 && days[i].Count > 0; i-- {
		streak++
	}

	name := o.Name(s)
	desc := fmt.Sprintf("Pulse of %s: %d of the last %d days have contributions, %.0f per day over the last 30 days.", name, beats, len(days), bpm)
	doc := svg.New(width, height, "Pulse of "+name, desc, font)
	doc.Raw(`<style>.pl-sweep{animation:pl-sweep 6s linear infinite}@keyframes pl-sweep{from{transform:translateX(0)}to{transform:translateX(` +
		fmt.Sprint(screenW-60) + `px)}}.pl-beat{transform-box:fill-box;transform-origin:center;animation:pl-beat 1s ease-in-out infinite}` +
		`@keyframes pl-beat{15%{transform:scale(1.25)}30%{transform:scale(1)}}` +
		`@media (prefers-reduced-motion:reduce){.pl-sweep,.pl-beat{animation:none}}</style>`)
	doc.F(`<defs><pattern id="pl-grid" width="12" height="12" patternUnits="userSpaceOnUse"><path d="M12 0H0V12" fill="none" stroke="%s" stroke-width=".6"/></pattern>`+
		`<filter id="pl-glow" x="-10%%" y="-50%%" width="120%%" height="200%%"><feGaussianBlur stdDeviation="2.2"/></filter>`+
		`<linearGradient id="pl-fade"><stop offset="0" stop-color="%s" stop-opacity="0"/><stop offset="1" stop-color="%s"/></linearGradient></defs>`, gridCol, backColor, backColor)
	doc.F(`<rect width="%d" height="%d" rx="10" fill="%s"/>`, width, height, backColor)
	doc.Header("PULSE", fmt.Sprintf("%s · last %d days · one beat per day", name, len(days)), traceCol, labelCol)
	doc.F(`<path class="pl-beat" d="M%d 22c-3-4-9-2-9 3 0 5 9 10 9 10s9-5 9-10c0-5-6-7-9-3z" fill="%s"/>`, width-38, traceCol)
	doc.F(`<rect x="%d" y="%d" width="%d" height="%d" fill="url(#pl-grid)"/>`, screenX, screenY, screenW, screenH)

	path := trace(days, peak)
	doc.F(`<path d="%s" fill="none" stroke="%s" stroke-width="3" opacity=".35" filter="url(#pl-glow)"/>`, path, traceCol)
	doc.F(`<path d="%s" fill="none" stroke="%s" stroke-width="1.5" stroke-linejoin="round"/>`, path, traceCol)
	doc.F(`<rect class="pl-sweep" x="%d" y="%d" width="60" height="%d" fill="url(#pl-fade)"/>`, screenX, screenY, screenH)
	if beats == 0 {
		doc.F(`<text x="%d" y="%.0f" fill="%s" font-size="12" text-anchor="middle">waiting for the first beat</text>`, width/2, baseline-20, labelCol)
	}

	doc.Stats([]svg.Stat{
		{Value: fmt.Sprintf("%.0f bpm", bpm), Label: "contributions/day, 30d"},
		{Value: fmt.Sprint(peak), Label: "strongest beat"},
		{Value: fmt.Sprintf("%d/%d", beats, len(days)), Label: "days with a pulse"},
		{Value: fmt.Sprintf("%d d", streak), Label: "current streak"},
	}, 24, height-50, width-48, valueCol, labelCol)
	return doc.Bytes(), nil
}

// trace builds the ECG line: a flat baseline with a P-QRS-T complex for every
// active day, scaled by the square root of its contributions.
func trace(days []model.Day, peak int) string {
	seg := float64(screenW) / float64(max(len(days), 1))
	var b strings.Builder
	fmt.Fprintf(&b, "M%d %.1f", screenX, baseline)
	for i, d := range days {
		x := float64(screenX) + float64(i)*seg
		if d.Count == 0 {
			fmt.Fprintf(&b, "L%.1f %.1f", x+seg, baseline)
			continue
		}
		a := 14 + 76*math.Sqrt(float64(d.Count)/float64(peak))
		points := [][2]float64{
			{.18, 0}, {.26, -a * .08}, {.34, 0}, // P wave
			{.42, a * .12}, {.5, -a}, {.58, a * .3}, // QRS complex
			{.66, 0}, {.8, -a * .14}, {.92, 0}, {1, 0}, // T wave
		}
		for _, p := range points {
			fmt.Fprintf(&b, "L%.1f %.1f", x+p[0]*seg, baseline+p[1])
		}
	}
	return b.String()
}

// window starts the monitor at the first active day, keeping at least minDays.
func window(days []model.Day) []model.Day {
	for i, d := range days {
		if d.Count > 0 {
			return days[min(i, max(0, len(days)-minDays)):]
		}
	}
	return days[max(0, len(days)-minDays):]
}
