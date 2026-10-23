package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/msch128/galaxy-stats/internal/card"
	"github.com/msch128/galaxy-stats/internal/cards"
	"github.com/msch128/galaxy-stats/internal/model"
)

const fakeGraphQL = `{"data":{"user":{"login":"octo","name":"Octo","createdAt":"2025-03-01T00:00:00Z","contributionsCollection":{
  "contributionCalendar":{"weeks":[{"contributionDays":[{"date":"2026-10-22","contributionCount":3},{"date":"2026-10-23","contributionCount":5}]}]},
  "commitContributionsByRepository":[
    {"repository":{"name":"secret-project","nameWithOwner":"octo/secret-project","owner":{"login":"octo"},"isPrivate":true,
      "createdAt":"2025-01-01T00:00:00Z","pushedAt":"2026-10-23T10:00:00Z","diskUsage":2048,"primaryLanguage":{"name":"Go"}},
     "contributions":{"totalCount":5,"nodes":[{"occurredAt":"2026-10-23T07:00:00Z","commitCount":3},{"occurredAt":"2026-10-22T07:00:00Z","commitCount":2}]}},
    {"repository":{"name":"tool","nameWithOwner":"acme/tool","owner":{"login":"acme"},"isPrivate":false,
      "createdAt":"2024-01-01T00:00:00Z","pushedAt":"2026-10-20T10:00:00Z","diskUsage":512,"primaryLanguage":null},
     "contributions":{"totalCount":3,"nodes":[{"occurredAt":"2026-10-22T07:00:00Z","commitCount":3}]}},
    {"repository":{"name":"octo","nameWithOwner":"octo/octo","owner":{"login":"octo"},"isPrivate":false,
      "createdAt":"2024-01-01T00:00:00Z","pushedAt":"2026-10-20T10:00:00Z","diskUsage":1,"primaryLanguage":null},
     "contributions":{"totalCount":9,"nodes":[{"occurredAt":"2026-10-22T07:00:00Z","commitCount":9}]}}
  ]}}}}`

func fakeAPI(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("authorization header = %q", got)
		}
		switch {
		case r.URL.Path == "/graphql":
			body, _ := io.ReadAll(r.Body)
			if strings.Contains(string(body), "totalContributions") {
				io.WriteString(w, `{"data":{"user":{"y2025":{"contributionCalendar":{"totalContributions":40}},"y2026":{"contributionCalendar":{"totalContributions":8}}}}}`)
				return
			}
			if !strings.Contains(string(body), `"login":"octo"`) {
				t.Errorf("graphql variables missing login: %s", body)
			}
			io.WriteString(w, fakeGraphQL)
		case strings.HasSuffix(r.URL.Path, "/commits"):
			if r.URL.Query().Get("author") != "octo" {
				t.Errorf("commits not filtered by author: %s", r.URL)
			}
			io.WriteString(w, `[{"commit":{"author":{"date":"2026-10-23T21:30:00Z","name":"Octo","email":"o@example.com"},"message":"secret message"}}]`)
		default:
			http.NotFound(w, r)
		}
	}))
}

func fetch(t *testing.T, showPrivate bool) []byte {
	t.Helper()
	srv := fakeAPI(t)
	defer srv.Close()
	c := &Client{Token: "test-token", BaseURL: srv.URL}
	s, err := c.Fetch(context.Background(), Request{Login: "octo", Timezone: "Europe/Berlin", Today: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC), ShowPrivateNames: showPrivate})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(s)
	for _, c := range cards.All() {
		svg, err := c.Render(s, card.Options{})
		if err != nil {
			t.Fatal(err)
		}
		b = append(b, svg...)
	}
	return b
}

func TestPrivateRepositoryNamesNeverLeaveTheFetcher(t *testing.T) {
	out := string(fetch(t, false))
	for _, secret := range []string{"secret-project", "secret message", "o@example.com"} {
		if strings.Contains(out, secret) {
			t.Errorf("%q leaked into the snapshot or a card", secret)
		}
	}
	if !strings.Contains(out, "acme/tool") {
		t.Errorf("public repositories of other owners should be named owner/name")
	}
}

func TestPrivateNamesCanBeShownOnRequest(t *testing.T) {
	if !strings.Contains(string(fetch(t, true)), "secret-project") {
		t.Errorf("show-private-names should keep the private repository's name")
	}
}

func TestProfileRepositoryIsSkipped(t *testing.T) {
	if strings.Contains(string(fetch(t, false)), `"name":"octo"`) {
		t.Errorf("the profile repository octo/octo should not be charted")
	}
}

func TestFetchKeepsCalendarAndCommitTimes(t *testing.T) {
	srv := fakeAPI(t)
	defer srv.Close()
	c := &Client{Token: "test-token", BaseURL: srv.URL}
	s, err := c.Fetch(context.Background(), Request{Login: "octo", Timezone: "UTC", Today: time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Calendar) != 2 || s.Calendar[1].Count != 5 {
		t.Errorf("calendar = %+v", s.Calendar)
	}
	if len(s.Repos) != 2 || s.Repos[0].Commits != 5 || s.Repos[0].Days[0].Date != "2026-10-22" {
		t.Errorf("repos should be sorted by commits with days ascending: %+v", s.Repos)
	}
	if len(s.CommitTimes) != 2 { // one commit from each recently active repository
		t.Errorf("commit times = %v", s.CommitTimes)
	}
	if len(s.Years) != 2 || s.Years[0] != (model.Year{Year: 2025, Count: 40}) || s.Years[1].Count != 8 {
		t.Errorf("years = %+v", s.Years)
	}
}

func TestFetchReportsGraphQLErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"errors":[{"message":"Could not resolve to a User"}]}`)
	}))
	defer srv.Close()
	c := &Client{Token: "t", BaseURL: srv.URL}
	_, err := c.Fetch(context.Background(), Request{Login: "nobody", Timezone: "UTC", Today: time.Now()})
	if err == nil || !strings.Contains(err.Error(), "Could not resolve") {
		t.Errorf("err = %v", err)
	}
}
