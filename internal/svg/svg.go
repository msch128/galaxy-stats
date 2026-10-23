// Package svg is a tiny string builder for hand-drawn SVG cards.
package svg

import (
	"fmt"
	"html"
	"strings"
)

// Doc accumulates SVG markup.
type Doc struct {
	b strings.Builder
}

// New starts an accessible SVG document of the given size. The title and
// description are read by screen readers and shown as alt text.
func New(width, height int, title, desc, font string) *Doc {
	d := &Doc{}
	d.f(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-labelledby="gs-title gs-desc" font-family="%s">`,
		width, height, width, height, Esc(font))
	d.f(`<title id="gs-title">%s</title><desc id="gs-desc">%s</desc>`, Esc(title), Esc(desc))
	return d
}

// F appends formatted markup. Callers escape any user-controlled text with Esc.
func (d *Doc) F(format string, args ...any) { d.f(format, args...) }

func (d *Doc) f(format string, args ...any) {
	fmt.Fprintf(&d.b, format, args...)
}

// Raw appends markup verbatim.
func (d *Doc) Raw(s string) { d.b.WriteString(s) }

// Bytes closes the document and returns it.
func (d *Doc) Bytes() []byte {
	d.b.WriteString("</svg>\n")
	return []byte(d.b.String())
}

// Esc escapes text for use in SVG content and attribute values.
func Esc(s string) string { return html.EscapeString(s) }

// Stat is one big number with a caption, as drawn in a card's footer row.
type Stat struct{ Value, Label string }

// Stats draws a row of stats spread evenly across width, starting at x.
func (d *Doc) Stats(stats []Stat, x, y, width float64, valueColor, labelColor string) {
	if len(stats) == 0 {
		return
	}
	step := width / float64(len(stats))
	for i, s := range stats {
		sx := x + float64(i)*step
		d.f(`<text x="%.0f" y="%.0f" fill="%s" font-size="18" font-weight="700">%s</text>`, sx, y, valueColor, Esc(s.Value))
		d.f(`<text x="%.0f" y="%.0f" fill="%s" font-size="10">%s</text>`, sx, y+16, labelColor, Esc(s.Label))
	}
}

// Header draws the card name and a subtitle on one line.
func (d *Doc) Header(name, subtitle, nameColor, subColor string) {
	d.f(`<text x="24" y="34" fill="%s" font-size="15" font-weight="700" letter-spacing="2">%s</text>`, nameColor, Esc(name))
	d.f(`<text x="%d" y="34" fill="%s" font-size="11">%s</text>`, 24+len(name)*12+14, subColor, Esc(subtitle))
}
