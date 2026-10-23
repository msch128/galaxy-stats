// Package card defines what a card is and the helpers cards share.
package card

import (
	"hash/fnv"
	"math/rand/v2"

	"github.com/msch128/galaxy-stats/internal/model"
)

// Card renders one SVG from a snapshot.
type Card interface {
	// ID is the name used in configuration and as the output file name.
	ID() string
	Render(s *model.Snapshot, o Options) ([]byte, error)
}

// Options are the user's settings. Cards read only the fields they know.
type Options struct {
	Theme       string // "dark" (default) or "light"; scene cards may ignore it
	DisplayName string // name shown where a card addresses the user, defaults to the login
}

// Name returns the display name, falling back to the login.
func (o Options) Name(s *model.Snapshot) string {
	if o.DisplayName != "" {
		return o.DisplayName
	}
	return s.Login
}

// Rand returns a generator seeded by the login and a salt, so a user's card
// looks the same on every run and different from everybody else's.
func Rand(s *model.Snapshot, salt string) *rand.Rand {
	h := fnv.New64a()
	h.Write([]byte(s.Login + "/" + salt))
	seed := h.Sum64()
	return rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
}

// PrivateLabel names the n-th (0-based) unnamed private repository without
// revealing anything about it.
func PrivateLabel(n int) string {
	const greek = "αβγδεζηθικλμνξοπρστυφχψω"
	letters := []rune(greek)
	return string(letters[n%len(letters)])
}

// LanguageColor returns a colour for a repository's primary language.
func LanguageColor(lang string) string {
	if c, ok := languageColors[lang]; ok {
		return c
	}
	return "#c9c3e6"
}

var languageColors = map[string]string{
	"Go":         "#00ADD8",
	"Rust":       "#dea584",
	"TypeScript": "#3178c6",
	"JavaScript": "#f1e05a",
	"Vue":        "#41b883",
	"Svelte":     "#ff3e00",
	"PHP":        "#777bb4",
	"Python":     "#3572A5",
	"Lua":        "#7f7fff",
	"HTML":       "#e34c26",
	"CSS":        "#8f6dd8",
	"Java":       "#b07219",
	"Kotlin":     "#A97BFF",
	"Swift":      "#F05138",
	"C":          "#9b9b9b",
	"C++":        "#f34b7d",
	"C#":         "#178600",
	"Ruby":       "#cc342d",
	"Shell":      "#89e051",
	"Dart":       "#00B4AB",
	"Elixir":     "#6e4a7e",
	"Zig":        "#ec915c",
}
