// Package github fetches a Snapshot from the GitHub GraphQL and REST APIs.
//
// It deliberately avoids the events API: push events no longer carry commit
// details, which is what breaks most profile generators sooner or later.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/msch128/galaxy-stats/internal/model"
)

// Client talks to the GitHub API on behalf of one token.
type Client struct {
	Token   string
	BaseURL string // defaults to https://api.github.com
	HTTP    *http.Client
}

// Request describes what to fetch.
type Request struct {
	Login            string
	Timezone         string // IANA name, e.g. Europe/Berlin
	Today            time.Time
	ShowPrivateNames bool
}

// Fetch builds a snapshot covering the 365 days up to and including req.Today.
func (c *Client) Fetch(ctx context.Context, req Request) (*model.Snapshot, error) {
	loc, err := time.LoadLocation(req.Timezone)
	if err != nil {
		return nil, fmt.Errorf("timezone %q: %w", req.Timezone, err)
	}
	today := time.Date(req.Today.Year(), req.Today.Month(), req.Today.Day(), 0, 0, 0, 0, time.UTC)
	from := today.AddDate(0, 0, -364)
	to := today.Add(24*time.Hour - time.Second)

	var res userResponse
	vars := map[string]any{"login": req.Login, "from": from.Format(time.RFC3339), "to": to.Format(time.RFC3339)}
	if err := c.graphql(ctx, userQuery, vars, &res); err != nil {
		return nil, err
	}
	u := res.User
	if u == nil {
		return nil, fmt.Errorf("user %q not found", req.Login)
	}

	s := &model.Snapshot{Login: u.Login, Name: u.Name, Today: today.Format(time.DateOnly), Timezone: loc.String()}
	for _, w := range u.Contributions.Calendar.Weeks {
		for _, d := range w.Days {
			s.Calendar = append(s.Calendar, model.Day{Date: d.Date, Count: d.Count})
		}
	}

	if s.Years, err = c.years(ctx, u.Login, u.CreatedAt, today); err != nil {
		return nil, err
	}

	recentSince := today.AddDate(0, 0, -13)
	var recent []string // repositories with commits in the last 14 days, as owner/name
	for _, rc := range u.Contributions.ByRepository {
		r := rc.Repository
		if strings.EqualFold(r.NameWithOwner, u.Login+"/"+u.Login) {
			continue // the profile repository itself is not activity worth charting
		}
		repo := model.Repo{
			Private:  r.IsPrivate,
			Language: r.PrimaryLanguage.Name,
			Created:  r.CreatedAt,
			Pushed:   r.PushedAt,
			DiskKB:   r.DiskUsage,
			Commits:  rc.Contributions.TotalCount,
		}
		if !r.IsPrivate || req.ShowPrivateNames {
			repo.Name = r.Name
			if !strings.EqualFold(r.Owner.Login, u.Login) {
				repo.Name = r.NameWithOwner
			}
		}
		for _, n := range slices.Backward(rc.Contributions.Nodes) { // API order is newest first
			day := n.OccurredAt.UTC().Format(time.DateOnly)
			repo.Days = append(repo.Days, model.Day{Date: day, Count: n.CommitCount})
		}
		if len(repo.Days) > 0 && !repo.Days[len(repo.Days)-1].Time().Before(recentSince) {
			recent = append(recent, r.NameWithOwner)
		}
		s.Repos = append(s.Repos, repo)
	}
	s.SortRepos()

	for _, full := range recent {
		times, err := c.commitTimes(ctx, full, u.Login, recentSince)
		if err != nil {
			return nil, err
		}
		s.CommitTimes = append(s.CommitTimes, times...)
	}
	return s, nil
}

// years returns the contribution total of every calendar year since the
// account was created, in one request with an aliased field per year.
func (c *Client) years(ctx context.Context, login string, joined, today time.Time) ([]model.Year, error) {
	end := today.Add(24*time.Hour - time.Second)
	var q strings.Builder
	q.WriteString("query($login: String!) { user(login: $login) {")
	for y := joined.Year(); y <= today.Year(); y++ {
		from := time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC)
		if from.Before(joined) {
			from = joined.UTC()
		}
		to := time.Date(y, 12, 31, 23, 59, 59, 0, time.UTC)
		if to.After(end) {
			to = end
		}
		fmt.Fprintf(&q, " y%d: contributionsCollection(from: %q, to: %q) { contributionCalendar { totalContributions } }",
			y, from.Format(time.RFC3339), to.Format(time.RFC3339))
	}
	q.WriteString(" } }")

	var res struct {
		User map[string]struct {
			Calendar struct {
				Total int `json:"totalContributions"`
			} `json:"contributionCalendar"`
		} `json:"user"`
	}
	if err := c.graphql(ctx, q.String(), map[string]any{"login": login}, &res); err != nil {
		return nil, err
	}
	var out []model.Year
	for y := joined.Year(); y <= today.Year(); y++ {
		out = append(out, model.Year{Year: y, Count: res.User[fmt.Sprint("y", y)].Calendar.Total})
	}
	return out, nil
}

// commitTimes lists when the user committed to a repository since a date.
// Only timestamps are kept; messages and file names never leave this function.
func (c *Client) commitTimes(ctx context.Context, fullName, login string, since time.Time) ([]time.Time, error) {
	const maxPages = 3
	var out []time.Time
	q := url.Values{"author": {login}, "since": {since.Format(time.RFC3339)}, "per_page": {"100"}}
	for page := 1; page <= maxPages; page++ {
		q.Set("page", fmt.Sprint(page))
		var commits []struct {
			Commit struct {
				Author struct {
					Date time.Time `json:"date"`
				} `json:"author"`
			} `json:"commit"`
		}
		err := c.do(ctx, http.MethodGet, "/repos/"+fullName+"/commits?"+q.Encode(), nil, &commits)
		var he *httpError
		if errors.As(err, &he) && (he.status == http.StatusConflict || he.status == http.StatusNotFound) {
			return out, nil // empty or inaccessible repository: nothing to count
		}
		if err != nil {
			return nil, err
		}
		for _, cm := range commits {
			out = append(out, cm.Commit.Author.Date)
		}
		if len(commits) < 100 {
			break
		}
	}
	return out, nil
}

func (c *Client) graphql(ctx context.Context, query string, vars map[string]any, out any) error {
	var res struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := c.do(ctx, http.MethodPost, "/graphql", map[string]any{"query": query, "variables": vars}, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		msgs := make([]string, len(res.Errors))
		for i, e := range res.Errors {
			msgs[i] = e.Message
		}
		return fmt.Errorf("graphql: %s", strings.Join(msgs, "; "))
	}
	return json.Unmarshal(res.Data, out)
}

type httpError struct {
	status int
	body   string
}

func (e *httpError) Error() string { return fmt.Sprintf("github api: %d %s", e.status, e.body) }

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	base := c.BaseURL
	if base == "" {
		base = "https://api.github.com"
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "galaxy-stats")
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return &httpError{status: resp.StatusCode, body: strings.TrimSpace(string(b))}
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

const userQuery = `query($login: String!, $from: DateTime!, $to: DateTime!) {
  user(login: $login) {
    login
    name
    createdAt
    contributionsCollection(from: $from, to: $to) {
      contributionCalendar { weeks { contributionDays { date contributionCount } } }
      commitContributionsByRepository(maxRepositories: 100) {
        repository {
          name
          nameWithOwner
          owner { login }
          isPrivate
          createdAt
          pushedAt
          diskUsage
          primaryLanguage { name }
        }
        contributions(first: 100, orderBy: {field: OCCURRED_AT, direction: DESC}) {
          totalCount
          nodes { occurredAt commitCount }
        }
      }
    }
  }
}`

type userResponse struct {
	User *struct {
		Login         string    `json:"login"`
		Name          string    `json:"name"`
		CreatedAt     time.Time `json:"createdAt"`
		Contributions struct {
			Calendar struct {
				Weeks []struct {
					Days []struct {
						Date  string `json:"date"`
						Count int    `json:"contributionCount"`
					} `json:"contributionDays"`
				} `json:"weeks"`
			} `json:"contributionCalendar"`
			ByRepository []struct {
				Repository struct {
					Name          string `json:"name"`
					NameWithOwner string `json:"nameWithOwner"`
					Owner         struct {
						Login string `json:"login"`
					} `json:"owner"`
					IsPrivate       bool      `json:"isPrivate"`
					CreatedAt       time.Time `json:"createdAt"`
					PushedAt        time.Time `json:"pushedAt"`
					DiskUsage       int       `json:"diskUsage"`
					PrimaryLanguage struct {
						Name string `json:"name"`
					} `json:"primaryLanguage"`
				} `json:"repository"`
				Contributions struct {
					TotalCount int `json:"totalCount"`
					Nodes      []struct {
						OccurredAt  time.Time `json:"occurredAt"`
						CommitCount int       `json:"commitCount"`
					} `json:"nodes"`
				} `json:"contributions"`
			} `json:"commitContributionsByRepository"`
		} `json:"contributionsCollection"`
	} `json:"user"`
}
