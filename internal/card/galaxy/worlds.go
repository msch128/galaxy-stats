package galaxy

import "strconv"

// world describes how a planet's surface is painted.
type world struct {
	base, surface string  // ocean or ground colour, and continents, dunes, bands or ice
	glow, ring    string  // atmosphere and ring colour
	detail        float64 // feature density; divided by the radius for the noise frequency
	sharp, cut    float64 // alpha = sharp*noise + cut thresholds the surface into shapes
	gas           bool    // banded gas giant with rings
	blend         bool    // soft two-colour blend instead of hard-edged shapes
	cracks        bool    // ridged noise, for lava and ice fissures
	clouds        bool    // white cloud layer
}

var (
	ocean  = world{base: "#0d4f8f", surface: "#3f9a63", glow: "#6fc3ff", detail: 2.2, sharp: 7, cut: -3.4, clouds: true}
	terran = world{base: "#1c5f86", surface: "#b39b62", glow: "#9fd2ff", detail: 2.0, sharp: 7, cut: -3.3, clouds: true}
	jungle = world{base: "#1f6b45", surface: "#7cc76a", glow: "#a6f0b4", detail: 2.6, blend: true, clouds: true}
	desert = world{base: "#b8743c", surface: "#f0c58a", glow: "#ffd9a8", detail: 1.6, blend: true}
	ice    = world{base: "#d8ecff", surface: "#6c9fd4", glow: "#e2f3ff", detail: 1.8, sharp: 9, cut: -1.2, cracks: true}
	lava   = world{base: "#2a0d08", surface: "#ff6a1f", glow: "#ff8a4c", detail: 1.5, sharp: 10, cut: -1.6, cracks: true}
	rock   = world{base: "#6d6a78", surface: "#a7a3b4", glow: "#c9c5d9", detail: 2.4, blend: true}
)

// gain and offset map noise (about 0.2..0.8) onto the blend between base and
// surface; rocky worlds get more contrast than gas giants.
func (w world) gain() float64 {
	if w.gas {
		return 1.6
	}
	return 2.4
}

func (w world) offset() float64 { return (w.gain() - 1) / 2 }

func gasGiant(dark, light string) world {
	return world{base: dark, surface: light, glow: light, ring: light, detail: 3.4, gas: true}
}

// worldFor picks a planet type for a repository's primary language.
func worldFor(lang string) world {
	switch lang {
	case "Go", "Dart":
		return ocean
	case "Rust", "C", "C++", "Zig":
		return desert
	case "TypeScript", "Swift", "Kotlin":
		return ice
	case "Vue", "Python", "Elixir":
		return jungle
	case "Svelte", "HTML", "Ruby":
		return lava
	case "PHP", "CSS":
		return gasGiant("#3a2f7a", "#b7a6ff")
	case "JavaScript":
		return gasGiant("#7a5a12", "#f3dc7a")
	case "Lua":
		return gasGiant("#1b2a6b", "#8fa8ff")
	case "Java", "C#":
		return terran
	case "Shell", "":
		return rock
	default:
		return terran
	}
}

// rgb converts #rrggbb to 0..1 channel values for SVG colour matrices.
func rgb(hex string) [3]float64 {
	var out [3]float64
	for i := range 3 {
		v, _ := strconv.ParseUint(hex[1+2*i:3+2*i], 16, 8)
		out[i] = float64(v) / 255
	}
	return out
}
