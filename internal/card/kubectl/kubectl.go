// Package kubectl draws recent activity as the output of `kubectl top`:
// every active repository is a container, CPU is commits over the last 30
// days, memory is the repository size, restarts are breaks of more than a
// week, and the footer reports the node's working hours.
package kubectl

import (
	"fmt"
	"sort"
	"time"

	"github.com/msch128/galaxy-stats/internal/card"
	"github.com/msch128/galaxy-stats/internal/model"
	"github.com/msch128/galaxy-stats/internal/svg"
)

// rowFormat lays out one table row; widths are in characters.
const rowFormat = "%-24s%10s%8s   %-11s%9s%8s"

const (
	maxRows    = 8
	activeDays = 90
	lineHeight = 19
	charWidth  = 7.6 // advance of the monospace font at 12.5px
	font       = "ui-monospace,SFMono-Regular,Menlo,Consolas,monospace"
)

type palette struct {
	bg, bar, barText, text, muted, prompt, running string
}

var palettes = map[string]palette{
	"dark":  {bg: "#0d1117", bar: "#1d2230", barText: "#8a93a8", text: "#e6edf3", muted: "#8b949e", prompt: "#7ee787", running: "#7ee787"},
	"light": {bg: "#ffffff", bar: "#eaeef2", barText: "#57606a", text: "#1f2328", muted: "#6e7781", prompt: "#1a7f37", running: "#1a7f37"},
}

// Card is the kubectl card.
type Card struct{}

func (Card) ID() string { return "kubectl" }

type row struct {
	name, cpu, mem, status, restarts, age string
	commits30                             int
}

func (Card) Render(s *model.Snapshot, o card.Options) ([]byte, error) {
	pal, ok := palettes[o.Theme]
	if !ok {
		pal = palettes["dark"]
	}
	name := o.Name(s)
	rows := containers(s)

	header := fmt.Sprintf(rowFormat, "NAME", "CPU(30d)", "MEM", "STATUS", "RESTARTS", "AGE")
	footer := "# " + nodeSummary(s, name)
	command := "$ kubectl top engineer " + name + " --containers"

	longest := max(len([]rune(header)), len([]rune(footer)), len([]rune(command)))
	width := int(48 + float64(longest)*charWidth)
	height := 72 + lineHeight*(max(len(rows), 1)+4) + 14

	desc := fmt.Sprintf("kubectl top for %s: %d active repositories in the last %d days.", name, len(rows), activeDays)
	doc := svg.New(width, height, "kubectl top engineer "+name, desc, font)
	doc.Raw(`<style>.kc-cur{animation:kc-cur 1.1s steps(1) infinite}@keyframes kc-cur{50%{opacity:0}}@media (prefers-reduced-motion:reduce){.kc-cur{animation:none}}</style>`)
	doc.F(`<rect width="%d" height="%d" rx="10" fill="%s"/>`, width, height, pal.bg)
	doc.F(`<path d="M0 30V10a10 10 0 0 1 10-10h%da10 10 0 0 1 10 10v20z" fill="%s"/>`, width-20, pal.bar)
	for i, c := range []string{"#ff5f57", "#febc2e", "#28c840"} {
		doc.F(`<circle cx="%d" cy="15" r="5.5" fill="%s"/>`, 20+i*18, c)
	}
	doc.F(`<text x="%d" y="19" fill="%s" font-size="11" text-anchor="middle">%s@cluster: ~</text>`, width/2, pal.barText, svg.Esc(name))

	y := 62
	text := func(s, color string) {
		doc.F(`<text x="24" y="%d" fill="%s" font-size="12.5" xml:space="preserve">%s</text>`, y, color, s)
		y += lineHeight
	}
	text(svg.Esc(command), pal.prompt)
	y += lineHeight
	text(svg.Esc(header), pal.muted)
	for _, r := range rows {
		color := pal.muted
		if r.status == "Running" {
			color = pal.running
		}
		before := fmt.Sprintf("%-24s%10s%8s   ", r.name, r.cpu, r.mem)
		after := fmt.Sprintf("%9s%8s", r.restarts, r.age)
		text(fmt.Sprintf(`%s<tspan fill="%s">%-11s</tspan>%s`, svg.Esc(before), color, r.status, svg.Esc(after)), pal.text)
	}
	if len(rows) == 0 {
		text("No resources found: the node is idle.", pal.text)
	}
	y += lineHeight
	text(svg.Esc(footer), pal.muted)
	doc.F(`<rect class="kc-cur" x="24" y="%d" width="8" height="15" fill="%s"/>`, y-12, pal.prompt)
	return doc.Bytes(), nil
}

// containers turns the repositories active in the last activeDays days into
// table rows, busiest first.
func containers(s *model.Snapshot) []row {
	now := s.Now()
	var out []row
	for _, r := range s.Repos {
		if len(r.Days) == 0 || now.Sub(r.Days[len(r.Days)-1].Time()) > activeDays*24*time.Hour {
			continue
		}
		c30, restarts := 0, 0
		for i, d := range r.Days {
			if now.Sub(d.Time()) < 30*24*time.Hour {
				c30 += d.Count
			}
			if i > 0 && d.Time().Sub(r.Days[i-1].Time()) > 7*24*time.Hour {
				restarts++
			}
		}
		status := "Completed"
		if now.Sub(r.Pushed.UTC().Truncate(24*time.Hour)) <= 14*24*time.Hour {
			status = "Running"
		}
		out = append(out, row{
			name:      truncate(r.Name, 23),
			cpu:       fmt.Sprintf("%dc", c30),
			mem:       fmt.Sprintf("%dMi", max(r.DiskKB/1024, 1)),
			status:    status,
			restarts:  fmt.Sprint(restarts),
			age:       age(now.Sub(r.Created)),
			commits30: c30,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].commits30 > out[j].commits30 })
	out = out[:min(len(out), maxRows)]
	private := 0
	for i := range out {
		if out[i].name == "" {
			private++
			out[i].name = fmt.Sprintf("closed-source-%02d", private)
		}
	}
	return out
}

// nodeSummary describes the user's rhythm over the last 14 days.
func nodeSummary(s *model.Snapshot, name string) string {
	if len(s.CommitTimes) == 0 {
		return fmt.Sprintf("node %s: cordoned for maintenance · replicas: 1 (no autoscaling)", name)
	}
	loc := s.Location()
	late := 0
	for _, t := range s.CommitTimes {
		if h := t.In(loc).Hour(); h >= 22 || h < 5 {
			late++
		}
	}
	return fmt.Sprintf("node %s: %.0f commits/day · %d%% scheduled after 22:00 · replicas: 1 (no autoscaling)",
		name, float64(len(s.CommitTimes))/14, late*100/len(s.CommitTimes))
}

func age(d time.Duration) string {
	days := int(d.Hours() / 24)
	switch {
	case days < 365:
		return fmt.Sprintf("%dd", max(days, 0))
	case days%365 < 30:
		return fmt.Sprintf("%dy", days/365)
	default:
		return fmt.Sprintf("%dy%dm", days/365, days%365/30)
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
