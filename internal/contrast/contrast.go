// Package contrast refuses a focus colour that does not stand off the surface
// it is drawn on, in any brightness scheme and colour-vision preset the design
// system offers.
//
// The failure it prevents is a colour tweak nobody re-reads. Two brightness
// schemes times five colour-vision presets is ten palettes, and the accent is
// the one value that moves in all of them. A person checking a new accent looks
// at the scheme they happen to be in, and the other nine are then carried by
// memory. The preset that goes unreadable is by construction the one belonging
// to the eye least able to compensate for it.
//
// Its subject is docs/design-tokens.json rather than the served page, because
// that file is the authority for these values and a client on another platform
// reads it rather than the page. What holds the page to the file is #40, and
// until that exists a green here says the declared values are legible and says
// nothing about what the page renders.
//
// What it does not reach, so that a green is not read as more than it is. The
// text colours are not preset-dependent and are not judged here: ink, ink-2 and
// ink-3 against the surfaces are one pair set for the whole system, and #37 has
// the measurement of them. Nothing here reads a rendered page, so the size a
// piece of text is actually painted at, whether a focus ring is drawn at all,
// and whether an element can be reached from the keyboard are all outside it.
package contrast

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
)

// File is the tracked file this package judges, relative to the repository
// root.
const File = "docs/design-tokens.json"

// FocusFloor is the contrast ratio a focus colour must reach against the
// surface behind it.
//
// 3.0 is WCAG 2.2 success criterion 1.4.11, which is the criterion for a user
// interface component's own colour rather than for text. The focus indicator is
// exactly that: a ring and a tint, carrying no glyphs. Taking the text figure
// for it would be the wrong number rather than a stricter one, and taking
// nothing would leave the ten palettes decided by whoever last liked a hue.
//
// It is a floor and not a target. An accent that only just clears it is a value
// somebody should look at, and this package does not say so, because a check
// that warns is a check people learn to scroll past.
const FocusFloor = 3.0

// TextFloor is the contrast ratio text must reach against what is behind it.
//
// 4.5 is WCAG 2.2 success criterion 1.4.3 for text below the large-text
// boundary. The one text pair this package judges is primary ink over the soft
// form of the accent, which is the pressed state of a control and is the only
// text pair the preset moves. Every other text pair is fixed across the ten
// palettes and is not this check's subject.
const TextFloor = 4.5

// Colour is an sRGB colour and an alpha, in the shape docs/design-tokens.json
// writes one.
type Colour struct {
	SRGB  string  `json:"srgb"`
	Alpha float64 `json:"alpha"`
}

// RGB is a colour with the alpha already resolved, each channel in [0,1].
type RGB struct{ R, G, B float64 }

// Parse reads the hex form the token file uses.
//
// It refuses anything else rather than guessing, because a colour this reader
// could not understand and skipped would be a combination nobody judged, and
// the count this package prints would still say it examined them all.
func (c Colour) Parse() (RGB, error) {
	s := strings.TrimSpace(c.SRGB)
	if len(s) != 7 || s[0] != '#' {
		return RGB{}, fmt.Errorf("%q is not a #RRGGBB colour", c.SRGB)
	}
	var out [3]float64
	for i := 0; i < 3; i++ {
		v, err := strconv.ParseUint(s[1+2*i:3+2*i], 16, 8)
		if err != nil {
			return RGB{}, fmt.Errorf("%q is not a #RRGGBB colour: %w", c.SRGB, err)
		}
		out[i] = float64(v) / 255
	}
	if c.Alpha < 0 || c.Alpha > 1 {
		return RGB{}, fmt.Errorf("alpha %v is outside 0 to 1", c.Alpha)
	}
	return RGB{out[0], out[1], out[2]}, nil
}

// Over composites c onto bg using c's alpha.
//
// A tint is a colour a reader never sees on its own: the soft accent is painted
// over a surface, and judging the tint against that surface as if it were
// opaque answers a question nobody asked.
func (c Colour) Over(bg RGB) (RGB, error) {
	fg, err := c.Parse()
	if err != nil {
		return RGB{}, err
	}
	a := c.Alpha
	return RGB{
		R: a*fg.R + (1-a)*bg.R,
		G: a*fg.G + (1-a)*bg.G,
		B: a*fg.B + (1-a)*bg.B,
	}, nil
}

// Luminance is the WCAG relative luminance of an sRGB colour.
func Luminance(c RGB) float64 {
	lin := func(v float64) float64 {
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

// Ratio is the WCAG contrast ratio between two colours, from 1 to 21. It is
// symmetric: which of the two is the text does not change the number.
func Ratio(a, b RGB) float64 {
	la, lb := Luminance(a), Luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// Tokens is the part of the token file this package reads.
type Tokens struct {
	Surface struct {
		Tokens map[string]map[string]json.RawMessage `json:"tokens"`
	} `json:"surface"`
	Focus struct {
		Presets map[string]Preset `json:"presets"`
	} `json:"focus"`
}

// Preset is one colour-vision preset's entry.
type Preset struct {
	Dark  Pair `json:"dark"`
	Light Pair `json:"light"`
}

// Pair is one preset in one brightness scheme.
type Pair struct {
	Accent     Colour `json:"accent"`
	AccentSoft Colour `json:"accent-soft"`
}

// Schemes are the brightness schemes every colour carries, in report order.
var Schemes = []string{"dark", "light"}

// Surfaces are the surface tokens a focus indicator can be drawn on, in report
// order. The hairline is not among them: it separates where brightness already
// does, so it is not the only means of anything, and holding a deliberately
// faint line to a contrast floor would refuse the design rather than a defect.
var Surfaces = []string{"ground", "raise", "raise-2"}

// Load reads a token file.
func Load(r io.Reader) (Tokens, error) {
	var t Tokens
	dec := json.NewDecoder(r)
	if err := dec.Decode(&t); err != nil {
		return Tokens{}, fmt.Errorf("reading %s: %w", File, err)
	}
	if len(t.Focus.Presets) == 0 {
		return Tokens{}, fmt.Errorf("%s declares no colour-vision preset, so there is nothing to judge", File)
	}
	if len(t.Surface.Tokens) == 0 {
		return Tokens{}, fmt.Errorf("%s declares no surface, so there is nothing to judge against", File)
	}
	return t, nil
}

// surface resolves one surface token in one scheme.
func (t Tokens) surface(name, scheme string) (RGB, error) {
	tok, ok := t.Surface.Tokens[name]
	if !ok {
		return RGB{}, fmt.Errorf("%s declares no surface named %q", File, name)
	}
	raw, ok := tok[scheme]
	if !ok {
		return RGB{}, fmt.Errorf("%s declares no %s value for the surface %q", File, scheme, name)
	}
	var c Colour
	if err := json.Unmarshal(raw, &c); err != nil {
		return RGB{}, fmt.Errorf("%s: the %s value of %q is not a colour: %w", File, scheme, name, err)
	}
	// A surface is what everything else is drawn on, so it is composited onto
	// itself being opaque; the hairline is the only token that is not, and it
	// is not a surface.
	return c.Parse()
}

func (p Preset) scheme(name string) (Pair, error) {
	switch name {
	case "dark":
		return p.Dark, nil
	case "light":
		return p.Light, nil
	}
	return Pair{}, fmt.Errorf("%q is not a brightness scheme this reader knows", name)
}

// Rule is what a finding was refused under.
const (
	RuleFocus = "a focus colour stands off every surface it can be drawn on"
	RuleText  = "text on the soft accent is readable"
)

// Finding is one refusal.
type Finding struct {
	Preset  string
	Scheme  string
	Subject string
	Ratio   float64
	Floor   float64
	Rule    string
}

func (f Finding) String() string {
	return fmt.Sprintf("%s/%s: %s is %.2f:1, under the floor of %.1f:1 (%s)",
		f.Preset, f.Scheme, f.Subject, f.Ratio, f.Floor, f.Rule)
}

// Check judges every combination the token file offers and says how many it
// examined.
//
// The count is returned rather than left to be inferred. A reader who is told
// only that nothing was refused cannot tell a file whose ten palettes all clear
// the floor from one this reader walked past, and those are the two outcomes
// this check exists between.
func Check(t Tokens) (found []Finding, examined int, err error) {
	presets := make([]string, 0, len(t.Focus.Presets))
	for name := range t.Focus.Presets {
		presets = append(presets, name)
	}
	sort.Strings(presets)

	for _, name := range presets {
		for _, scheme := range Schemes {
			pair, err := t.Focus.Presets[name].scheme(scheme)
			if err != nil {
				return nil, 0, err
			}
			accent, err := pair.Accent.Parse()
			if err != nil {
				return nil, 0, fmt.Errorf("%s/%s accent: %w", name, scheme, err)
			}

			for _, s := range Surfaces {
				bg, err := t.surface(s, scheme)
				if err != nil {
					return nil, 0, err
				}
				examined++
				if r := Ratio(accent, bg); r < FocusFloor {
					found = append(found, Finding{
						Preset: name, Scheme: scheme,
						Subject: "the accent on " + s,
						Ratio:   r, Floor: FocusFloor, Rule: RuleFocus,
					})
				}
			}

			// The soft accent is a background rather than a mark: it is the
			// pressed state of a control, and primary ink is written on it.
			raise, err := t.surface("raise", scheme)
			if err != nil {
				return nil, 0, err
			}
			soft, err := pair.AccentSoft.Over(raise)
			if err != nil {
				return nil, 0, fmt.Errorf("%s/%s soft accent: %w", name, scheme, err)
			}
			ink, err := t.surface("ink", scheme)
			if err != nil {
				return nil, 0, err
			}
			examined++
			if r := Ratio(ink, soft); r < TextFloor {
				found = append(found, Finding{
					Preset: name, Scheme: scheme,
					Subject: "ink on the soft accent over raise",
					Ratio:   r, Floor: TextFloor, Rule: RuleText,
				})
			}
		}
	}

	if examined == 0 {
		return nil, 0, fmt.Errorf("%s produced no combination to judge", File)
	}
	return found, examined, nil
}
