//go:build needs_browser

// The half of internal/keyboard that needs a browser: what a real render with
// the page's own script running actually puts in the tab order, and what the
// keys do once focus is there.
//
// It is here rather than in the gate because deciding it needs a browser
// installed, which decisions/headless-and-unelevated.md keeps out of the merge
// gate. The expectation it is measured against is derived in the untagged file
// beside this one and runs in the gate, so a page that grew a control nobody can
// reach reds here rather than passing quietly in both places.
package keyboard

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

// tabCap bounds the walk through the tab order.
//
// A page whose focus handling traps the keyboard would otherwise run until the
// job timed out, and a timeout says "the runner" where this should say "the
// page". It is several times the largest control count the pages hold, so it
// does not quietly become the thing that ends the walk on a page that grew.
const tabCap = 60

func browser(t *testing.T) (context.Context, func()) {
	t.Helper()
	alloc, cancelAlloc := chromedp.NewExecAllocator(context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-extensions", true),
		)...)
	ctx, cancelCtx := chromedp.NewContext(alloc)
	ctx, cancelTimeout := context.WithTimeout(ctx, 120*time.Second)
	return ctx, func() { cancelTimeout(); cancelCtx(); cancelAlloc() }
}

// pageURL is the file address of a served page in this checkout.
//
// The pages are opened off the disk rather than off a server, so this check
// needs a browser and nothing else. A render that fetched anything would make it
// depend on somebody else's service while claiming to depend only on a browser,
// and site-fetches-nothing-outside is the leg that holds the pages to that.
func pageURL(t *testing.T, page string) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", filepath.FromSlash(page)))
	if err != nil {
		t.Fatalf("resolving %s: %v", page, err)
	}
	return (&url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(abs)}).String()
}

// focused reads the element that has focus, as the Control the untagged reader
// would have produced for it.
//
// The label is taken the same way: an explicit one where there is one, the text
// otherwise, whitespace collapsed. The tag name carries no space and a label
// may, so the first space is the separator and nothing else has to be escaped.
const focused = `(function(){
  var e = document.activeElement;
  if (!e || e === document.body || e === document.documentElement) { return "" }
  var label = e.getAttribute("aria-label");
  if (label === null) { label = e.textContent }
  return e.tagName.toLowerCase() + " " + label.split(/\s+/).filter(Boolean).join(" ");
})()`

func parseFocused(s string) (Control, bool) {
	tag, label, ok := strings.Cut(s, " ")
	if !ok || tag == "" {
		return Control{}, false
	}
	return Control{Tag: tag, Label: label}, true
}

// walk tabs from the top of the document and collects what focus lands on, in
// order, stopping when it comes back round to the first control or when the cap
// is reached.
func walk(ctx context.Context, t *testing.T, page string) []Control {
	t.Helper()
	if err := chromedp.Run(ctx,
		chromedp.Navigate(pageURL(t, page)),
		chromedp.WaitReady("body"),
		// Focus starts at the document rather than wherever the previous
		// navigation left it, so this is the walk a reader who has just opened
		// the page makes.
		chromedp.Evaluate(`document.activeElement && document.activeElement.blur && document.activeElement.blur()`, nil),
	); err != nil {
		t.Fatalf("%s: opening: %v", page, err)
	}

	var out []Control
	for i := 0; i < tabCap; i++ {
		var got string
		if err := chromedp.Run(ctx,
			chromedp.KeyEvent(kb.Tab),
			chromedp.Evaluate(focused, &got),
		); err != nil {
			t.Fatalf("%s: tab %d: %v", page, i+1, err)
		}
		c, ok := parseFocused(got)
		if !ok {
			// Focus has left the page's own controls, which is the browser's
			// own interface. The walk is over.
			break
		}
		if len(out) > 0 && c == out[0] {
			break
		}
		out = append(out, c)
	}
	return out
}

func TestEveryControlOnEveryServedPageIsReachableByTabAlone(t *testing.T) {
	ctx, done := browser(t)
	defer done()

	for _, page := range Pages {
		declared, err := Interactive(read(t, page))
		if err != nil {
			t.Fatalf("%s: %v", page, err)
		}
		if len(declared) == 0 {
			t.Fatalf("%s declares no control, so a green here would say nothing", page)
		}

		reached := walk(ctx, t, page)
		t.Logf("%s: %d control(s) declared, %d reached by Tab", page, len(declared), len(reached))
		if missing := Missing(declared, reached); len(missing) > 0 {
			t.Errorf("%s: the keyboard never reaches %s", page, Format(missing))
		}
		if extra := Extra(declared, reached); len(extra) > 0 {
			t.Errorf("%s: focus lands on %s, which the markup does not declare as a control, so this comparison no longer describes the page", page, Format(extra))
		}
	}
}

func TestEveryBrightnessAndPresetButtonIsOperableFromTheKeyboard(t *testing.T) {
	// Reachable and operable are different claims. A button that takes focus and
	// does nothing on Enter is the failure a pointer never shows.
	ctx, done := browser(t)
	defer done()

	page := "docs/design-system.html"
	var count int
	if err := chromedp.Run(ctx,
		chromedp.Navigate(pageURL(t, page)),
		chromedp.WaitReady("body"),
		chromedp.Evaluate(`document.querySelectorAll(".mode").length`, &count),
	); err != nil {
		t.Fatalf("opening %s: %v", page, err)
	}
	if count == 0 {
		t.Fatal("the page carries no brightness or preset button, so this proves nothing")
	}
	t.Logf("%s: %d brightness and preset button(s)", page, count)

	for i := 0; i < count; i++ {
		sel := fmt.Sprintf(`document.querySelectorAll(".mode")[%d]`, i)
		var before, after, label string
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(sel+`.textContent.trim()`, &label),
			chromedp.Evaluate(sel+`.focus()`, nil),
			chromedp.Evaluate(sel+`.getAttribute("aria-pressed")`, &before),
			chromedp.KeyEvent(kb.Enter),
			chromedp.Evaluate(sel+`.getAttribute("aria-pressed")`, &after),
		); err != nil {
			t.Fatalf("operating button %d: %v", i, err)
		}
		if after != "true" {
			t.Errorf("%q read %s before Enter and %s after, so pressing it from the keyboard did not select it",
				label, before, after)
		}
	}
}

func TestTheExampleRowMovesUnderTheArrowKeys(t *testing.T) {
	// The behaviour with no pointer equivalent at all, and the reason this half
	// cannot be decided by a Go test reading the file: the handling is in the
	// page's own script and only exists once that script has run.
	ctx, done := browser(t)
	defer done()

	page := "docs/design-system.html"
	at := `[].slice.call(document.querySelectorAll(".fcell")).findIndex(function(c){return c.getAttribute("aria-selected")==="true"})`

	var cells, start, right, back int
	if err := chromedp.Run(ctx,
		chromedp.Navigate(pageURL(t, page)),
		chromedp.WaitReady("body"),
		chromedp.Evaluate(`document.querySelectorAll(".fcell").length`, &cells),
		chromedp.Evaluate(at, &start),
		chromedp.Evaluate(`document.querySelectorAll(".fcell")[0].focus()`, nil),
		chromedp.KeyEvent(kb.ArrowRight),
		chromedp.Evaluate(at, &right),
		chromedp.KeyEvent(kb.ArrowLeft),
		chromedp.Evaluate(at, &back),
	); err != nil {
		t.Fatalf("operating the row: %v", err)
	}
	if cells < 2 {
		t.Fatalf("the row holds %d cell(s), so an arrow key has nowhere to go and this proves nothing", cells)
	}
	t.Logf("%s: %d cell(s) in the row", page, cells)
	if start != 0 {
		t.Fatalf("the row opens with cell %d selected, want the first", start)
	}
	if right != 1 {
		t.Errorf("ArrowRight moved the selection to %d, want 1", right)
	}
	if back != 0 {
		t.Errorf("ArrowLeft moved the selection back to %d, want 0", back)
	}
}
