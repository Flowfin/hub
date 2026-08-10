// Package tokens holds page-matches-the-token-file, which refuses a difference
// between the values docs/design-tokens.json declares and the values the
// published design system page carries.
//
// Extracting the tokens into a file created a second copy of every value, which
// is a new way to be wrong rather than a fix on its own. Within a month one of
// the two is the real one and nobody knows which, unless something refuses a
// difference. That is this.
//
// It reads declared values and never a rendered pixel, so it stays fast, stays
// headless and stays in the merge gate. It does not re-implement the cascade
// either: which declaration wins for a given reader is a question about a
// browser, and answering it here would be a second stylesheet engine nobody
// maintains.
//
// Both directions are refused and they catch different mistakes. A check that
// only refused page values missing from the file would let the file grow stale
// entries forever. A check that only refused file entries missing from the page
// would let the page grow untracked colours. Neither half is the check.
//
// Its subject is one page, and the token file names which one. docs/index.html
// is not it: that page carries its own six abbreviations for the same palette,
// which is a real divergence and is #40's sibling rather than its subject. Report
// says so on every run, so a green here is not read as covering it.
package tokens

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// File is the token file, and Page is the page held to it. Both are relative to
// the repository root.
const (
	File = "docs/design-tokens.json"
	Page = "docs/design-system.html"
)

// Unheld is the served page this check is not the subject of, named so that a
// green over Page is not read as a green over the site.
const Unheld = "docs/index.html"

// DefaultScheme is the brightness scheme a declaration is in when nothing in
// its selector or its enclosing at-rule says otherwise.
//
// The page's bare :root block is its dark values, and its light values are all
// behind either a preference query or a data-theme attribute. That is a fact
// about the page rather than a convention of CSS, so it is named here: a page
// that flipped its default would red against every dark token until this
// constant moved with it, which is the right way round for a value the check
// cannot derive.
const DefaultScheme = "dark"

// DefaultPreset is the colour-vision preset a declaration is in when its
// selector carries no data-cvd attribute.
const DefaultPreset = "standard"

// Any is the scheme or preset of a value that does not change with one. A
// surface is the same colour under every preset, and a corner radius is the same
// length under both schemes, so demanding a context for either would be
// inventing a distinction the design system does not make.
const Any = ""

// Kind is what a value is, which is what decides when two spellings of it are
// the same value. #fff and #FFFFFF are one colour and a byte comparison reds on
// the first token it reaches; 12 and 12px are one length.
type Kind int

const (
	// ColourKind is an sRGB colour with an alpha.
	ColourKind Kind = iota
	// LengthKind is a number of pixels.
	LengthKind
	// ShadowKind is the one shadow the system has: four lengths and a colour.
	ShadowKind
	// FontKind is an ordered list of family names.
	FontKind
)

func (k Kind) String() string {
	switch k {
	case ColourKind:
		return "colour"
	case LengthKind:
		return "length"
	case ShadowKind:
		return "shadow"
	case FontKind:
		return "font stack"
	}
	return "unknown"
}

// Declaration is one value, canonical, with the context it applies in and where
// it was read.
type Declaration struct {
	Var    string
	Scheme string
	Preset string
	Kind   Kind

	// Value is the canonical form, which is what is compared. The two sides
	// write the same value differently and neither spelling is wrong.
	Value string

	// Where is the JSON path or the page selector the value was read at, for
	// the refusal to point at.
	Where string
}

// Finding is one refusal.
type Finding struct {
	Var    string
	Rule   string
	Detail string
}

func (f Finding) String() string { return fmt.Sprintf("%s: %s (%s)", f.Var, f.Detail, f.Rule) }

// The rules, in the words a refusal prints.
const (
	RuleTracked  = "every value the page declares is one the token file declares"
	RuleUsed     = "every token carrying a custom property is declared in the page"
	RuleInPlace  = "the page declares each value under the scheme and preset the file gives it"
	RuleKnownVar = "the page declares no custom property the token file does not name"
)

// Check compares the two sides in both directions and says how much it examined.
//
// The counts are returned rather than left to be inferred, for the reason
// internal/contrast returns its own: a reader told only that nothing was refused
// cannot tell a page that matches the file from a page this reader walked past.
func Check(fileSide, pageSide []Declaration) (found []Finding, examinedFile, examinedPage int, err error) {
	if len(fileSide) == 0 {
		return nil, 0, 0, fmt.Errorf("%s produced no value to compare, so there is nothing to hold %s to", File, Page)
	}
	if len(pageSide) == 0 {
		return nil, 0, 0, fmt.Errorf("%s declares no custom property, so there is nothing to hold to %s", Page, File)
	}

	byVarFile := group(fileSide)
	byVarPage := group(pageSide)

	// Direction one, the file against the page: a token nobody uses.
	for _, name := range sorted(byVarFile) {
		examinedFile += len(byVarFile[name])
		pageValues := values(byVarPage[name])
		if len(byVarPage[name]) == 0 {
			found = append(found, Finding{
				Var: name, Rule: RuleUsed,
				Detail: fmt.Sprintf("the token file declares it at %s and %s never declares it",
					strings.Join(places(byVarFile[name]), ", "), Page),
			})
			continue
		}
		for _, d := range byVarFile[name] {
			if !pageValues[d.Value] {
				found = append(found, Finding{
					Var: name, Rule: RuleUsed,
					Detail: fmt.Sprintf("the token file declares the %s %s at %s and no declaration in %s carries it",
						d.Kind, d.Value, d.Where, Page),
				})
			}
		}
	}

	// Direction two, the page against the file: an untracked value.
	fileValues := map[string]map[string]bool{}
	for name, ds := range byVarFile {
		fileValues[name] = values(ds)
	}
	for _, name := range sorted(byVarPage) {
		examinedPage += len(byVarPage[name])
		known, ok := fileValues[name]
		if !ok {
			found = append(found, Finding{
				Var: name, Rule: RuleKnownVar,
				Detail: fmt.Sprintf("%s declares it at %s and the token file names no token carrying it",
					Page, strings.Join(places(byVarPage[name]), ", ")),
			})
			continue
		}
		for _, d := range byVarPage[name] {
			if !known[d.Value] {
				found = append(found, Finding{
					Var: name, Rule: RuleTracked,
					Detail: fmt.Sprintf("%s declares the %s %s at %s and the token file declares %s",
						Page, d.Kind, d.Value, d.Where, listValues(byVarFile[name])),
				})
				continue
			}
			if !inPlace(d, byVarFile[name]) {
				found = append(found, Finding{
					Var: name, Rule: RuleInPlace,
					Detail: fmt.Sprintf("%s declares the %s %s at %s, and the token file gives that value to %s",
						Page, d.Kind, d.Value, d.Where, contextsOf(d.Value, byVarFile[name])),
				})
			}
		}
	}

	sort.SliceStable(found, func(i, j int) bool {
		if found[i].Var != found[j].Var {
			return found[i].Var < found[j].Var
		}
		return found[i].Detail < found[j].Detail
	})
	return found, examinedFile, examinedPage, nil
}

// inPlace reports whether the file gives this value a context the page's
// declaration is compatible with.
//
// Compatible rather than equal, because the file states the context a value
// applies in and the page relies on the cascade to carry it: the ring width is
// declared once and inherited by both schemes, and a token that does not change
// with the preset is declared under no preset at all. So a file side of Any
// matches any page context, and only a stated context has to agree.
func inPlace(d Declaration, fileSide []Declaration) bool {
	for _, f := range fileSide {
		if f.Value != d.Value {
			continue
		}
		if f.Scheme != Any && f.Scheme != d.Scheme {
			continue
		}
		if f.Preset != Any && f.Preset != d.Preset {
			continue
		}
		return true
	}
	return false
}

func contextsOf(value string, ds []Declaration) string {
	var out []string
	for _, d := range ds {
		if d.Value != value {
			continue
		}
		out = append(out, context(d.Scheme, d.Preset))
	}
	if len(out) == 0 {
		return "nothing"
	}
	sort.Strings(out)
	return strings.Join(unique(out), " and ")
}

func context(scheme, preset string) string {
	switch {
	case scheme == Any && preset == Any:
		return "every scheme and preset"
	case scheme == Any:
		return "the " + preset + " preset in every scheme"
	case preset == Any:
		return "the " + scheme + " scheme in every preset"
	}
	return "the " + preset + " preset in the " + scheme + " scheme"
}

func group(ds []Declaration) map[string][]Declaration {
	out := map[string][]Declaration{}
	for _, d := range ds {
		out[d.Var] = append(out[d.Var], d)
	}
	return out
}

func values(ds []Declaration) map[string]bool {
	out := map[string]bool{}
	for _, d := range ds {
		out[d.Value] = true
	}
	return out
}

func listValues(ds []Declaration) string {
	var out []string
	for _, d := range ds {
		out = append(out, d.Value)
	}
	sort.Strings(out)
	return strings.Join(unique(out), ", ")
}

func places(ds []Declaration) []string {
	var out []string
	for _, d := range ds {
		out = append(out, d.Where)
	}
	sort.Strings(out)
	return unique(out)
}

func unique(in []string) []string {
	out := in[:0:0]
	for i, s := range in {
		if i == 0 || s != in[i-1] {
			out = append(out, s)
		}
	}
	return out
}

func sorted[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Report writes what the run compared, including the page it did not read.
func Report(w io.Writer, found []Finding, examinedFile, examinedPage int, skipped []string) {
	fmt.Fprintf(w, "%s and %s: %d declared value(s) in the file and %d in the page, compared in both directions.\n",
		File, Page, examinedFile, examinedPage)
	if len(skipped) > 0 {
		fmt.Fprintf(w, "%d section(s) of the file carry no custom property and were not compared: %s.\n",
			len(skipped), strings.Join(skipped, ", "))
	}
	fmt.Fprintf(w, "%s is a served page and is not held to the file here; it carries its own abbreviations for the same palette.\n", Unheld)
	for _, f := range found {
		fmt.Fprintf(w, "  %s\n", f)
	}
}

// canonColour is the one spelling a colour is compared in.
func canonColour(srgb string, alpha float64) (string, error) {
	s := strings.TrimSpace(srgb)
	if !strings.HasPrefix(s, "#") {
		return "", fmt.Errorf("%q is not a hexadecimal colour", srgb)
	}
	hex := s[1:]
	// The short form is the same colour written in half the characters, and the
	// page uses it. Expanding rather than refusing is the whole reason this
	// function exists: #fff and #FFFFFF differ in every byte and are one value.
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	if len(hex) != 6 {
		return "", fmt.Errorf("%q is not a #RGB or #RRGGBB colour", srgb)
	}
	for i := 0; i < 6; i++ {
		if _, err := strconv.ParseUint(hex[i:i+1], 16, 8); err != nil {
			return "", fmt.Errorf("%q is not a #RRGGBB colour", srgb)
		}
	}
	if alpha < 0 || alpha > 1 {
		return "", fmt.Errorf("alpha %v is outside 0 to 1", alpha)
	}
	return fmt.Sprintf("#%s@%s", strings.ToLower(hex), number(alpha)), nil
}

// number formats a value so that 0.07, .07 and 7e-2 are one string.
func number(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

// parseColour reads the two forms the page writes a colour in.
func parseColour(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "#") {
		return canonColour(s, 1)
	}
	fn, args, ok := call(s)
	if !ok || (fn != "rgba" && fn != "rgb") {
		return "", fmt.Errorf("%q is neither a hexadecimal colour nor an rgb() or rgba() colour", raw)
	}
	if len(args) != 4 && len(args) != 3 {
		return "", fmt.Errorf("%q carries %d arguments and %s() takes three or four", raw, len(args), fn)
	}

	var channels [3]uint64
	for i := 0; i < 3; i++ {
		v, err := strconv.ParseUint(strings.TrimSpace(args[i]), 10, 8)
		if err != nil {
			return "", fmt.Errorf("%q: channel %d is %q, which is not 0 to 255", raw, i+1, args[i])
		}
		channels[i] = v
	}
	alpha := 1.0
	if len(args) == 4 {
		v, err := strconv.ParseFloat(strings.TrimSpace(args[3]), 64)
		if err != nil {
			return "", fmt.Errorf("%q: the alpha is %q, which is not a number", raw, args[3])
		}
		alpha = v
	}
	return canonColour(fmt.Sprintf("#%02X%02X%02X", channels[0], channels[1], channels[2]), alpha)
}

// call splits name(a,b,c) into its name and its arguments.
func call(s string) (name string, args []string, ok bool) {
	open := strings.Index(s, "(")
	if open < 0 || !strings.HasSuffix(s, ")") {
		return "", nil, false
	}
	return strings.ToLower(strings.TrimSpace(s[:open])), strings.Split(s[open+1:len(s)-1], ","), true
}

// parseLength reads a number of pixels, with or without the unit.
func parseLength(raw string) (string, error) {
	s := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(raw), "px"))
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return "", fmt.Errorf("%q is not a length in pixels", raw)
	}
	return number(v), nil
}

// parseShadow reads the four lengths and the colour of the one shadow the system
// has. Anything else in the value is refused rather than ignored, because a
// shadow this reader half-understood would be a value nobody compared.
func parseShadow(raw string) (string, error) {
	fields, err := shadowFields(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if len(fields) != 5 {
		return "", fmt.Errorf("%q carries %d parts and a shadow here is four lengths and a colour", raw, len(fields))
	}
	out := make([]string, 0, 5)
	for _, f := range fields[:4] {
		length, err := parseLength(f)
		if err != nil {
			return "", fmt.Errorf("%q: %w", raw, err)
		}
		out = append(out, length)
	}
	colour, err := parseColour(fields[4])
	if err != nil {
		return "", fmt.Errorf("%q: %w", raw, err)
	}
	return strings.Join(append(out, colour), " "), nil
}

// shadowFields splits on whitespace without splitting inside rgba(...).
func shadowFields(s string) ([]string, error) {
	var out []string
	depth, start := 0, 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("%q closes a bracket it did not open", s)
			}
		case ' ', '\t', '\n':
			if depth == 0 {
				if i > start {
					out = append(out, s[start:i])
				}
				start = i + 1
			}
		}
	}
	if depth != 0 {
		return nil, fmt.Errorf("%q leaves a bracket open", s)
	}
	if len(s) > start {
		out = append(out, s[start:])
	}
	return out, nil
}

// canonStack is the one spelling a font stack is compared in. The quotes a
// family name with a space in it needs are punctuation rather than part of the
// name, and the page writes them where the file does not.
func canonStack(families []string) string {
	out := make([]string, 0, len(families))
	for _, f := range families {
		f = strings.TrimSpace(f)
		f = strings.Trim(f, `"'`)
		out = append(out, strings.TrimSpace(f))
	}
	return strings.Join(out, ",")
}

func parseStack(raw string) string { return canonStack(strings.Split(raw, ",")) }

// parse reads a page value as the kind the token file says that custom property
// holds.
//
// Driven by the file rather than guessed from the text, so a colour written
// where a length belongs is a refusal that says so rather than a value this
// reader classified into agreement.
func parse(kind Kind, raw string) (string, error) {
	switch kind {
	case ColourKind:
		return parseColour(raw)
	case LengthKind:
		return parseLength(raw)
	case ShadowKind:
		return parseShadow(raw)
	case FontKind:
		return parseStack(raw), nil
	}
	return "", fmt.Errorf("%q is of no kind this reader knows", raw)
}

type jsonColour struct {
	SRGB  string  `json:"srgb"`
	Alpha float64 `json:"alpha"`
}

type jsonShadow struct {
	OffsetX float64    `json:"offset-x"`
	OffsetY float64    `json:"offset-y"`
	Blur    float64    `json:"blur"`
	Spread  float64    `json:"spread"`
	Colour  jsonColour `json:"colour"`
}

func (s jsonShadow) canonical() (string, error) {
	colour, err := canonColour(s.Colour.SRGB, s.Colour.Alpha)
	if err != nil {
		return "", err
	}
	return strings.Join([]string{
		number(s.OffsetX), number(s.OffsetY), number(s.Blur), number(s.Spread), colour,
	}, " "), nil
}

// carriesNoCustomProperty names the top-level sections of the token file that
// state no value the page holds in a custom property.
//
// Listed rather than skipped silently, and checked against what the file
// actually has: a section added later is refused until somebody decides which
// list it is in, so the file cannot grow a value this check quietly stops
// comparing.
var carriesNoCustomProperty = map[string]bool{
	"what": true, "not-here": true, "how-to-read-this": true,
	"type": true, "budget": true,
}

// FromFile reads every value the token file states in a custom property, and
// names the sections it did not read.
func FromFile(r io.Reader) (out []Declaration, skipped []string, err error) {
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", File, err)
	}

	readers := map[string]func(json.RawMessage) ([]Declaration, error){
		"surface": readSurface,
		"lift":    readLift,
		"focus":   readFocus,
		"shape":   readValues,
		"font":    readFont,
	}

	for _, section := range sorted(raw) {
		switch {
		case readers[section] != nil:
			ds, err := readers[section](raw[section])
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %s: %w", File, section, err)
			}
			out = append(out, ds...)
		case carriesNoCustomProperty[section]:
			skipped = append(skipped, section)
		default:
			return nil, nil, fmt.Errorf("%s carries a section this reader does not know, %q: it is either read here or named as carrying no custom property, and until one of those happens its values are compared against nothing",
				File, section)
		}
	}
	return out, skipped, nil
}

func readSurface(raw json.RawMessage) ([]Declaration, error) {
	var section struct {
		Tokens map[string]struct {
			CSSVar string     `json:"css-var"`
			Dark   jsonColour `json:"dark"`
			Light  jsonColour `json:"light"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(raw, &section); err != nil {
		return nil, err
	}

	var out []Declaration
	for _, name := range sorted(section.Tokens) {
		t := section.Tokens[name]
		if t.CSSVar == "" {
			return nil, fmt.Errorf("the token %q names no css-var", name)
		}
		for scheme, c := range map[string]jsonColour{"dark": t.Dark, "light": t.Light} {
			value, err := canonColour(c.SRGB, c.Alpha)
			if err != nil {
				return nil, fmt.Errorf("%s, %s: %w", name, scheme, err)
			}
			out = append(out, Declaration{
				Var: t.CSSVar, Scheme: scheme, Preset: Any, Kind: ColourKind,
				Value: value, Where: fmt.Sprintf("surface.tokens.%s.%s", name, scheme),
			})
		}
	}
	return out, nil
}

func readLift(raw json.RawMessage) ([]Declaration, error) {
	var section struct {
		CSSVar string     `json:"css-var"`
		Dark   jsonShadow `json:"dark"`
		Light  jsonShadow `json:"light"`
	}
	if err := json.Unmarshal(raw, &section); err != nil {
		return nil, err
	}
	if section.CSSVar == "" {
		return nil, fmt.Errorf("the shadow names no css-var")
	}

	var out []Declaration
	for scheme, s := range map[string]jsonShadow{"dark": section.Dark, "light": section.Light} {
		value, err := s.canonical()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", scheme, err)
		}
		out = append(out, Declaration{
			Var: section.CSSVar, Scheme: scheme, Preset: Any, Kind: ShadowKind,
			Value: value, Where: "lift." + scheme,
		})
	}
	return out, nil
}

func readFocus(raw json.RawMessage) ([]Declaration, error) {
	var section struct {
		Accent struct {
			CSSVar string `json:"css-var"`
		} `json:"accent"`
		AccentSoft struct {
			CSSVar string `json:"css-var"`
		} `json:"accent-soft"`
		Presets map[string]struct {
			RingWidth float64 `json:"ring-width"`
			Dark      struct {
				Accent     jsonColour `json:"accent"`
				AccentSoft jsonColour `json:"accent-soft"`
			} `json:"dark"`
			Light struct {
				Accent     jsonColour `json:"accent"`
				AccentSoft jsonColour `json:"accent-soft"`
			} `json:"light"`
		} `json:"presets"`
		DefaultRingWidth struct {
			CSSVar string  `json:"css-var"`
			Value  float64 `json:"value"`
		} `json:"default-ring-width"`
	}
	if err := json.Unmarshal(raw, &section); err != nil {
		return nil, err
	}
	if section.Accent.CSSVar == "" || section.AccentSoft.CSSVar == "" {
		return nil, fmt.Errorf("the accent or the soft accent names no css-var, so the page's values for them are held to nothing")
	}
	if section.DefaultRingWidth.CSSVar == "" {
		return nil, fmt.Errorf("the ring width names no css-var")
	}

	out := []Declaration{{
		Var: section.DefaultRingWidth.CSSVar, Scheme: Any, Preset: Any, Kind: LengthKind,
		Value: number(section.DefaultRingWidth.Value), Where: "focus.default-ring-width",
	}}

	for _, name := range sorted(section.Presets) {
		p := section.Presets[name]
		// The ring width applies to both schemes, which the file says in
		// ring-width-applies-to, so it carries no scheme here.
		out = append(out, Declaration{
			Var: section.DefaultRingWidth.CSSVar, Scheme: Any, Preset: name, Kind: LengthKind,
			Value: number(p.RingWidth), Where: fmt.Sprintf("focus.presets.%s.ring-width", name),
		})

		for scheme, pair := range map[string]struct{ Accent, AccentSoft jsonColour }{
			"dark":  {p.Dark.Accent, p.Dark.AccentSoft},
			"light": {p.Light.Accent, p.Light.AccentSoft},
		} {
			for field, spec := range map[string]struct {
				variable string
				colour   jsonColour
			}{
				"accent":      {section.Accent.CSSVar, pair.Accent},
				"accent-soft": {section.AccentSoft.CSSVar, pair.AccentSoft},
			} {
				value, err := canonColour(spec.colour.SRGB, spec.colour.Alpha)
				if err != nil {
					return nil, fmt.Errorf("%s, %s, %s: %w", name, scheme, field, err)
				}
				out = append(out, Declaration{
					Var: spec.variable, Scheme: scheme, Preset: name, Kind: ColourKind,
					Value: value, Where: fmt.Sprintf("focus.presets.%s.%s.%s", name, scheme, field),
				})
			}
		}
	}
	return out, nil
}

func readValues(raw json.RawMessage) ([]Declaration, error) {
	var section map[string]struct {
		CSSVar string  `json:"css-var"`
		Value  float64 `json:"value"`
	}
	if err := json.Unmarshal(raw, &section); err != nil {
		return nil, err
	}

	var out []Declaration
	for _, name := range sorted(section) {
		t := section[name]
		if t.CSSVar == "" {
			return nil, fmt.Errorf("the token %q names no css-var", name)
		}
		out = append(out, Declaration{
			Var: t.CSSVar, Scheme: Any, Preset: Any, Kind: LengthKind,
			Value: number(t.Value), Where: "shape." + name,
		})
	}
	return out, nil
}

func readFont(raw json.RawMessage) ([]Declaration, error) {
	var section map[string]json.RawMessage
	if err := json.Unmarshal(raw, &section); err != nil {
		return nil, err
	}

	var out []Declaration
	for _, name := range sorted(section) {
		var t struct {
			CSSVar string   `json:"css-var"`
			Stack  []string `json:"stack"`
		}
		if err := json.Unmarshal(section[name], &t); err != nil {
			// The section's prose keys are strings rather than objects, and
			// they carry no value. A key that is an object and still fails to
			// read is the case this must not swallow, so only the string form
			// is passed over.
			var prose string
			if json.Unmarshal(section[name], &prose) == nil {
				continue
			}
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if t.CSSVar == "" || len(t.Stack) == 0 {
			return nil, fmt.Errorf("the font %q names no css-var or carries no stack", name)
		}
		out = append(out, Declaration{
			Var: t.CSSVar, Scheme: Any, Preset: Any, Kind: FontKind,
			Value: canonStack(t.Stack), Where: "font." + name,
		})
	}
	return out, nil
}
