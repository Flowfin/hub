package tokens

import (
	"fmt"
	"strings"
)

// FromPage reads every custom property the page declares, with the brightness
// scheme and colour-vision preset its block applies in.
//
// It reads declarations and never resolves the cascade. Which declaration wins
// for a given reader is a question about a browser, and the kinds are supplied
// by the token file rather than guessed from the text, so a value written in a
// shape this reader does not understand is a refusal rather than a value quietly
// classified into agreement.
func FromPage(html string, kinds map[string]Kind) ([]Declaration, error) {
	style, err := styleOf(html)
	if err != nil {
		return nil, err
	}

	var (
		out     []Declaration
		stack   []string
		prelude strings.Builder
	)
	css := stripComments(style)

	flush := func() error {
		text := strings.TrimSpace(prelude.String())
		prelude.Reset()
		if text == "" || len(stack) == 0 {
			return nil
		}
		name, value, ok := strings.Cut(text, ":")
		name = strings.TrimSpace(name)
		if !ok || !strings.HasPrefix(name, "--") {
			return nil
		}
		selector := stack[len(stack)-1]
		if !strings.HasPrefix(strings.TrimSpace(selector), ":root") {
			// A custom property set on something other than the root is a local
			// override rather than a token, and the design system declares none.
			return nil
		}

		kind, known := kinds[name]
		if !known {
			// The comparison is what refuses this, with the page's own spelling
			// in the refusal, so it is carried through unparsed rather than
			// dropped here where nothing would report it.
			out = append(out, Declaration{
				Var: name, Kind: ColourKind, Value: strings.TrimSpace(value),
				Scheme: schemeOf(stack), Preset: presetOf(stack), Where: where(stack),
			})
			return nil
		}

		canonical, err := parse(kind, value)
		if err != nil {
			return fmt.Errorf("%s: %s at %s: %w", Page, name, where(stack), err)
		}
		out = append(out, Declaration{
			Var: name, Kind: kind, Value: canonical,
			Scheme: schemeOf(stack), Preset: presetOf(stack), Where: where(stack),
		})
		return nil
	}

	for _, r := range css {
		switch r {
		case '{':
			stack = append(stack, strings.TrimSpace(prelude.String()))
			prelude.Reset()
		case '}':
			if err := flush(); err != nil {
				return nil, err
			}
			if len(stack) == 0 {
				return nil, fmt.Errorf("%s closes a block it did not open", Page)
			}
			stack = stack[:len(stack)-1]
		case ';':
			if err := flush(); err != nil {
				return nil, err
			}
		default:
			prelude.WriteRune(r)
		}
	}
	if len(stack) != 0 {
		return nil, fmt.Errorf("%s leaves %d block(s) open", Page, len(stack))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s declares no custom property on its root, and a reader that found none is not the same as a page that has none", Page)
	}
	return out, nil
}

// styleOf pulls the one stylesheet out of the page.
//
// One rather than the first: a second style element would carry values this
// reader never saw, and a check silently reading half a page is the shape of
// green that means nothing.
func styleOf(html string) (string, error) {
	const open, close = "<style>", "</style>"
	if n := strings.Count(html, "<style"); n != 1 {
		return "", fmt.Errorf("%s carries %d style elements and this reader reads one", Page, n)
	}
	from := strings.Index(html, open)
	to := strings.Index(html, close)
	if from < 0 || to < from {
		return "", fmt.Errorf("%s carries a style element this reader cannot delimit", Page)
	}
	return html[from+len(open) : to], nil
}

// stripComments removes CSS comments, which otherwise put a brace or a colon
// into the parse from inside prose.
func stripComments(css string) string {
	var out strings.Builder
	for {
		from := strings.Index(css, "/*")
		if from < 0 {
			out.WriteString(css)
			return out.String()
		}
		out.WriteString(css[:from])
		rest := css[from+2:]
		to := strings.Index(rest, "*/")
		if to < 0 {
			return out.String()
		}
		css = rest[to+2:]
	}
}

// theme attributes, as the page writes them.
const (
	themeLight = `[data-theme="light"]`
	themeDark  = `[data-theme="dark"]`
	cvdPrefix  = `[data-cvd="`
	lightQuery = "prefers-color-scheme: light"
	darkQuery  = "prefers-color-scheme: dark"
)

// schemeOf reads the brightness scheme out of the enclosing blocks.
//
// The selector wins over the enclosing query, because the page's light query
// carries selectors that exclude the dark theme and a reader taking the query
// first would put those in the wrong scheme. Where neither says anything the
// answer is DefaultScheme, which is a fact about this page and is named there.
func schemeOf(stack []string) string {
	selector := withoutNegations(stack[len(stack)-1])
	switch {
	case strings.Contains(selector, themeLight):
		return "light"
	case strings.Contains(selector, themeDark):
		return "dark"
	}
	for _, s := range stack {
		q := normaliseSpace(s)
		switch {
		case strings.Contains(q, lightQuery):
			return "light"
		case strings.Contains(q, darkQuery):
			return "dark"
		}
	}
	return DefaultScheme
}

// presetOf reads the colour-vision preset out of the innermost selector.
func presetOf(stack []string) string {
	selector := withoutNegations(stack[len(stack)-1])
	from := strings.Index(selector, cvdPrefix)
	if from < 0 {
		return DefaultPreset
	}
	rest := selector[from+len(cvdPrefix):]
	to := strings.Index(rest, `"`)
	if to < 0 {
		return DefaultPreset
	}
	return rest[:to]
}

// withoutNegations removes :not(...) groups before the attributes are read.
//
// The page writes :root[data-cvd="protan"]:not([data-theme="dark"]) inside its
// light query, and a reader looking for the dark attribute anywhere in the
// selector would place that block in the dark scheme, which is the opposite of
// what it is. This is the one piece of selector syntax the reader has to
// understand, and it understands it by deleting it.
func withoutNegations(selector string) string {
	var out strings.Builder
	for {
		from := strings.Index(selector, ":not(")
		if from < 0 {
			out.WriteString(selector)
			return out.String()
		}
		out.WriteString(selector[:from])
		rest := selector[from+len(":not("):]
		depth := 1
		i := 0
		for ; i < len(rest) && depth > 0; i++ {
			switch rest[i] {
			case '(':
				depth++
			case ')':
				depth--
			}
		}
		selector = rest[i:]
	}
}

func normaliseSpace(s string) string { return strings.Join(strings.Fields(s), " ") }

// where names the block a declaration was read in, innermost last, so a refusal
// points at a place somebody can find by searching the page.
func where(stack []string) string {
	parts := make([]string, 0, len(stack))
	for _, s := range stack {
		parts = append(parts, normaliseSpace(s))
	}
	return strings.Join(parts, " > ")
}

// Kinds is the map FromPage needs: which kind of value each custom property
// holds, taken from the token file rather than from a second list here.
func Kinds(fileSide []Declaration) map[string]Kind {
	out := map[string]Kind{}
	for _, d := range fileSide {
		out[d.Var] = d.Kind
	}
	return out
}
