// Package cards is the registry of every available card.
package cards

import (
	"github.com/msch128/galaxy-stats/internal/card"
	"github.com/msch128/galaxy-stats/internal/card/galaxy"
	"github.com/msch128/galaxy-stats/internal/card/kubectl"
	"github.com/msch128/galaxy-stats/internal/card/nightsky"
)

// All returns every card in default render order. Register new cards here.
func All() []card.Card {
	return []card.Card{galaxy.Card{}, nightsky.Card{}, kubectl.Card{}}
}
