// Package demo builds a made-up but realistic snapshot. It drives the example
// images in the README and the golden tests, so it must stay deterministic.
package demo

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/msch128/galaxy-stats/internal/model"
)

// Today is the fixed end of the demo window.
var Today = time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)

type repoSpec struct {
	name     string
	language string
	ageDays  int
	diskKB   int
	share    float64 // share of daily activity
	from, to int     // active between these many days ago
}

var specs = []repoSpec{
	{"orbit-api", "Go", 900, 41_000, .34, 0, 365},
	{"nebula-ui", "TypeScript", 420, 23_500, .22, 0, 240},
	{"", "Rust", 300, 9_800, .16, 0, 120},
	{"pulsar", "Rust", 160, 7_600, .12, 0, 160},
	{"", "Go", 700, 64_000, .07, 20, 365},
	{"comet-cli", "Go", 1200, 1_900, .05, 40, 300},
	{"", "Python", 90, 3_100, .03, 60, 90},
	{"dotfiles", "Shell", 2100, 400, .01, 5, 365},
}

// Snapshot returns the demo snapshot.
func Snapshot() *model.Snapshot {
	rnd := rand.New(rand.NewPCG(42, 7))
	s := &model.Snapshot{Login: "astro-dev", Name: "Astro Dev", Today: Today.Format(time.DateOnly), Timezone: "Europe/Berlin"}

	repos := make([]model.Repo, len(specs))
	for i, sp := range specs {
		repos[i] = model.Repo{
			Name:     sp.name,
			Private:  sp.name == "",
			Language: sp.language,
			Created:  Today.AddDate(0, 0, -sp.ageDays),
			DiskKB:   sp.diskKB,
		}
	}

	for ago := 364; ago >= 0; ago-- {
		day := Today.AddDate(0, 0, -ago)
		count := dailyCount(rnd, day, ago)
		s.Calendar = append(s.Calendar, model.Day{Date: day.Format(time.DateOnly), Count: count})
		for i, sp := range specs {
			if ago < sp.from || ago > sp.to || count == 0 {
				continue
			}
			n := int(math.Round(float64(count) * sp.share * (.6 + .8*rnd.Float64())))
			if n == 0 && rnd.Float64() < sp.share*4 {
				n = 1 // small projects still get the odd commit
			}
			if n == 0 {
				continue
			}
			r := &repos[i]
			r.Commits += n
			r.Days = append(r.Days, model.Day{Date: day.Format(time.DateOnly), Count: n})
			r.Pushed = day.Add(time.Duration(9+rnd.IntN(14)) * time.Hour)
		}
	}
	for i := range repos {
		if days := repos[i].Days; len(days) > 100 {
			repos[i].Days = days[len(days)-100:] // the API returns at most 100 days per repository
		}
	}
	s.Repos = repos
	s.SortRepos()

	loc, _ := time.LoadLocation(s.Timezone)
	for _, d := range s.LastDays(14) {
		for range d.Count {
			hour := []int{9, 10, 11, 14, 15, 16, 17, 20, 21, 22, 23, 0}[rnd.IntN(12)]
			t := time.Date(d.Time().Year(), d.Time().Month(), d.Time().Day(), hour, rnd.IntN(60), 0, 0, loc)
			s.CommitTimes = append(s.CommitTimes, t.UTC())
		}
	}
	return s
}

// dailyCount shapes a year of activity: busier weekdays, a few release
// bursts, and a holiday break, the way a real calendar looks.
func dailyCount(rnd *rand.Rand, day time.Time, ago int) int {
	if ago > 330 && ago < 345 { // holiday
		return 0
	}
	base, quiet := 6.0, .2
	if wd := day.Weekday(); wd == time.Saturday || wd == time.Sunday {
		base, quiet = 2.5, .55
	}
	for _, burst := range []int{3, 95, 180, 260} {
		if d := ago - burst; d >= 0 && d < 12 {
			base += 3 * (12 - float64(d))
			quiet = 0
		}
	}
	if rnd.Float64() < quiet {
		return 0
	}
	r := rnd.Float64()
	return max(1, int(base*(.3+1.7*r*r)))
}
