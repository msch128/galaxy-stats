// Package model holds the data every card renders from. A Snapshot is fetched
// once per run and is plain JSON, so cards can be tested against fixtures.
package model

import (
	"sort"
	"time"
)

// Day is one calendar day and how much happened on it.
type Day struct {
	Date  string `json:"date"` // YYYY-MM-DD
	Count int    `json:"count"`
}

// Time parses Date. The calendar is date-only, so UTC is used throughout.
func (d Day) Time() time.Time {
	t, _ := time.Parse(time.DateOnly, d.Date)
	return t
}

// Repo is a repository the user committed to during the snapshot window.
// Name is empty for private repositories unless the user opted in to showing
// them; cards must never try to recover it.
type Repo struct {
	Name     string    `json:"name"`
	Private  bool      `json:"private"`
	Language string    `json:"language"`
	Created  time.Time `json:"created"`
	Pushed   time.Time `json:"pushed"`
	DiskKB   int       `json:"diskKB"`
	Commits  int       `json:"commits"` // commits in the snapshot window
	Days     []Day     `json:"days"`    // most recent days with commits, ascending
}

// Snapshot is everything a run knows about one user.
type Snapshot struct {
	Login       string      `json:"login"`
	Name        string      `json:"name"`
	Today       string      `json:"today"`       // YYYY-MM-DD, end of the window
	Timezone    string      `json:"timezone"`    // IANA name used for commit times
	Calendar    []Day       `json:"calendar"`    // contribution calendar, ascending
	Repos       []Repo      `json:"repos"`       // sorted by Commits, descending
	CommitTimes []time.Time `json:"commitTimes"` // the user's commits of the last 14 days
}

// Now returns Today as a time, so cards never read the wall clock.
func (s *Snapshot) Now() time.Time {
	t, _ := time.Parse(time.DateOnly, s.Today)
	return t
}

// Location returns the configured time zone, falling back to UTC.
func (s *Snapshot) Location() *time.Location {
	if loc, err := time.LoadLocation(s.Timezone); err == nil && s.Timezone != "" {
		return loc
	}
	return time.UTC
}

// LastDays returns the calendar's final n days (or fewer if the calendar is shorter).
func (s *Snapshot) LastDays(n int) []Day {
	if n >= len(s.Calendar) {
		return s.Calendar
	}
	return s.Calendar[len(s.Calendar)-n:]
}

// SortRepos orders repos by commits (descending), then by name for stability.
func (s *Snapshot) SortRepos() {
	sort.SliceStable(s.Repos, func(i, j int) bool {
		if s.Repos[i].Commits != s.Repos[j].Commits {
			return s.Repos[i].Commits > s.Repos[j].Commits
		}
		return s.Repos[i].Name < s.Repos[j].Name
	})
}
