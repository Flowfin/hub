package keyboard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func read(t *testing.T, page string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(page)))
	if err != nil {
		t.Fatalf("opening %s: %v", page, err)
	}
	return string(b)
}

func TestTheServedPagesDeclareTheControlsAKeyboardHasToReach(t *testing.T) {
	// The expectation the browser side is measured against, read here so that a
	// scan which quietly stopped finding anything is a failure in the gate
	// rather than an empty comparison the harness reports as clean.
	want := map[string]int{"docs/index.html": 1, "docs/design-system.html": 14}
	for _, page := range Pages {
		got, err := Interactive(read(t, page))
		if err != nil {
			t.Fatalf("%s: %v", page, err)
		}
		if len(got) != want[page] {
			t.Errorf("%s declares %d control(s), want %d: %s", page, len(got), want[page], Format(got))
		}
		for _, c := range got {
			if c.Label == "" {
				t.Errorf("%s declares a control with no text on it, which the browser side cannot match: %s", page, c)
			}
		}
	}
}

func TestAControlTakenOutOfTheTabOrderIsNotExpectedInIt(t *testing.T) {
	// tabindex="-1" is a deliberate statement that the keyboard does not go
	// there. Expecting it anyway would make this check red on a page that is
	// right, which is the fastest way to have it switched off.
	got, err := Interactive(`<button tabindex="-1">Hidden</button><button>Shown</button>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Label != "Shown" {
		t.Fatalf("read %s, want the shown button alone", Format(got))
	}
}

func TestSomethingPutInTheTabOrderByHandIsExpectedInIt(t *testing.T) {
	// The other direction, and it is the one that matters for a design system:
	// a div made focusable is a control, and a check reading only the native
	// tags would walk past exactly the element most likely to be operable by
	// pointer alone.
	got, err := Interactive(`<div tabindex="0">Custom</div>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Tag != "div" || got[0].Label != "Custom" {
		t.Fatalf("read %s, want the div", Format(got))
	}
}

func TestAnAnchorWithNoDestinationIsNotAControl(t *testing.T) {
	// An anchor without an href is not in the tab order, so expecting focus on
	// every in-page target name would fail on every page that has one.
	got, err := Interactive(`<a name="top"></a><a href="x.html">Go</a>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Label != "Go" {
		t.Fatalf("read %s, want the linked anchor alone", Format(got))
	}
}

func TestAnAttributeNameIsMatchedWholeAndNotAsASuffix(t *testing.T) {
	// The one-character mistake in the reader rather than in the page: a
	// substring match reads data-href as href, and the anchor above then counts
	// as a control that is not in the tab order.
	got, err := Interactive(`<a data-href="x.html">Not a link</a>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("read %s, want nothing", Format(got))
	}
}

func TestTheLabelIsTheTextAReaderSeesAndNotTheMarkupAroundIt(t *testing.T) {
	got, err := Interactive("<button class=\"fcell\"><span class=\"art\"></span><span>The\n  Shaft</span></button>")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Label != "The Shaft" {
		t.Fatalf("read %s, want the collapsed text", Format(got))
	}
}

func TestAnExplicitLabelWins(t *testing.T) {
	got, err := Interactive(`<button aria-label="Close the panel">x</button>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Label != "Close the panel" {
		t.Fatalf("read %s, want the explicit label", Format(got))
	}
}

func TestAControlThatIsNeverClosedIsAnErrorRatherThanASkip(t *testing.T) {
	// Unreadable is not a pass. A control whose text this reader could not find
	// and silently dropped would be a control the browser side never looks for.
	if _, err := Interactive(`<button>Never closed`); err == nil {
		t.Fatal("an unclosed control was walked past and the read stayed clean")
	}
}

func TestTwoControlsReadingTheSameAreTwoControls(t *testing.T) {
	// Multiset, not set. A page with two buttons labelled the same, one of which
	// fell out of the tab order, matches on a set comparison and is exactly the
	// defect this check is for.
	declared, err := Interactive(`<button>More</button><button>More</button>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(declared) != 2 {
		t.Fatalf("read %s, want two", Format(declared))
	}
	reached := []Control{{Tag: "button", Label: "More"}}
	missing := Missing(declared, reached)
	if len(missing) != 1 {
		t.Fatalf("Missing reported %s, want one of the two", Format(missing))
	}
	if extra := Extra(declared, reached); len(extra) != 0 {
		t.Fatalf("Extra reported %s over a subset", Format(extra))
	}
}

func TestFocusLandingSomewhereUndeclaredIsReported(t *testing.T) {
	// How this check goes blind. A page that put something in the tab order in a
	// shape the reader above does not understand would otherwise pass, because
	// nothing declared went unreached.
	declared := []Control{{Tag: "button", Label: "Dark"}}
	reached := []Control{{Tag: "button", Label: "Dark"}, {Tag: "div", Label: "Surprise"}}
	extra := Extra(declared, reached)
	if len(extra) != 1 || extra[0].Label != "Surprise" {
		t.Fatalf("Extra reported %s, want the undeclared one", Format(extra))
	}
	if missing := Missing(declared, reached); len(missing) != 0 {
		t.Fatalf("Missing reported %s over a superset", Format(missing))
	}
}

func TestFormatNamesAControlWithNoTextRatherThanPrintingNothing(t *testing.T) {
	got := Format([]Control{{Tag: "input", Label: ""}})
	if !strings.Contains(got, "no text") {
		t.Fatalf("a control with no label formatted as %q", got)
	}
}

func TestADisabledControlIsNotExpectedInTheTabOrder(t *testing.T) {
	// Disabled is out of the tab order by the specification, so expecting focus
	// on it would refuse a page that is right. The near neighbour is the one to
	// be careful about: aria-disabled says so to a screen reader and leaves the
	// control focusable, so it stays expected.
	got, err := Interactive(`<button disabled>Off</button><button aria-disabled="true">Announced</button>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Label != "Announced" {
		t.Fatalf("read %s, want the aria-disabled control alone", Format(got))
	}
}
