package contrast

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func load(t *testing.T, name string) Tokens {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("opening the fixture: %v", err)
	}
	defer f.Close()
	tok, err := Load(f)
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return tok
}

func TestTheRatioIsTheOneEverybodyElseComputes(t *testing.T) {
	// Two anchors from the specification rather than from this file's own
	// arithmetic: black against white is 21, and any colour against itself
	// is 1. A luminance formula with a transposed coefficient still produces
	// plausible middling numbers and fails both of these.
	black, white := RGB{0, 0, 0}, RGB{1, 1, 1}
	if r := Ratio(black, white); math.Abs(r-21) > 0.001 {
		t.Fatalf("black on white is %.4f:1, want 21", r)
	}
	if r := Ratio(white, black); math.Abs(r-21) > 0.001 {
		t.Fatalf("the ratio is not symmetric: white on black is %.4f:1", r)
	}
	mid := RGB{0.5, 0.5, 0.5}
	if r := Ratio(mid, mid); math.Abs(r-1) > 0.001 {
		t.Fatalf("a colour against itself is %.4f:1, want 1", r)
	}
}

func TestATintIsJudgedOverWhatItIsPaintedOn(t *testing.T) {
	// The half a reader is most likely to skip. A soft accent at alpha 0.16 is
	// never seen on its own, and judging it as if it were opaque answers a
	// question about a colour nobody paints.
	bg := RGB{0, 0, 0}
	over, err := Colour{SRGB: "#FFFFFF", Alpha: 0.5}.Over(bg)
	if err != nil {
		t.Fatalf("compositing: %v", err)
	}
	if over.R <= 0.49 || over.R >= 0.51 {
		t.Fatalf("white at half alpha over black came out at %.3f, want about 0.5", over.R)
	}
	opaque, err := Colour{SRGB: "#FFFFFF", Alpha: 0.5}.Parse()
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	if Ratio(opaque, bg) == Ratio(over, bg) {
		t.Fatal("the alpha made no difference, so the tint was judged as if it were opaque")
	}
}

func TestEveryCombinationTheFileOffersIsExamined(t *testing.T) {
	// A check that walked past a palette reports the same clean verdict as one
	// that judged it, which is why the count is asserted rather than only the
	// findings. One preset, two schemes, three surfaces plus the text pair.
	found, examined, err := Check(load(t, "one_preset_clean.json"))
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if examined != 8 {
		t.Fatalf("examined %d combinations, want 8", examined)
	}
	if len(found) != 0 {
		t.Fatalf("a fixture holding the published values was refused: %v", found)
	}
}

func TestRefusesAnAccentLoweredAgainstOneSurfaceOnly(t *testing.T) {
	// The one-character mistake somebody actually makes: an accent nudged until
	// it still clears the ground it was looked at against and no longer clears
	// the raised surface it is also drawn on. A check judging the ground alone
	// passes this.
	found, examined, err := Check(load(t, "accent_lowered_one_step.json"))
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if examined != 8 {
		t.Fatalf("examined %d combinations, want 8", examined)
	}
	if len(found) != 1 {
		t.Fatalf("the lowered accent produced %d refusals, want 1: %v", len(found), found)
	}
	f := found[0]
	if f.Rule != RuleFocus || f.Scheme != "dark" || f.Preset != "standard" {
		t.Fatalf("the refusal does not say which palette it is about: %s", f)
	}
	if !strings.Contains(f.Subject, "raise-2") {
		t.Fatalf("the refusal does not name the surface: %s", f)
	}
	if f.Ratio >= FocusFloor {
		t.Fatalf("the refusal quotes %.2f, which is not under the floor", f.Ratio)
	}
}

func TestRefusesTextTheSoftAccentHasSwallowed(t *testing.T) {
	// The other direction, and it is not caught by the rule above: the accent
	// itself stands off every surface, and the tint behind the text has been
	// taken up to where the text on it is gone.
	found, _, err := Check(load(t, "soft_accent_swamps_the_text.json"))
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("the swamped text produced %d refusals, want 1: %v", len(found), found)
	}
	if found[0].Rule != RuleText {
		t.Fatalf("refused under %q, want %q", found[0].Rule, RuleText)
	}
}

func TestAColourThisReaderCannotUnderstandIsRefusedRatherThanSkipped(t *testing.T) {
	if _, _, err := Check(load(t, "accent_is_not_a_colour.json")); err == nil {
		t.Fatal("a value that is not a colour was walked past and the run stayed clean")
	}
}

func TestAFileWithNothingToJudgeIsAnErrorRatherThanACleanRun(t *testing.T) {
	for _, name := range []string{"declares_no_preset.json", "declares_no_surface.json"} {
		f, err := os.Open(filepath.Join("testdata", name))
		if err != nil {
			t.Fatalf("opening %s: %v", name, err)
		}
		_, err = Load(f)
		f.Close()
		if err == nil {
			t.Fatalf("%s was read as a token file with nothing wrong in it", name)
		}
	}
}

func TestTheDesignSystemsFocusColourStandsOffEverySurface(t *testing.T) {
	// The leg, against what this repository actually publishes.
	f, err := os.Open(filepath.Join("..", "..", filepath.FromSlash(File)))
	if err != nil {
		t.Fatalf("opening the token file: %v", err)
	}
	defer f.Close()
	tok, err := Load(f)
	if err != nil {
		t.Fatalf("reading the token file: %v", err)
	}
	found, examined, err := Check(tok)
	if err != nil {
		t.Fatalf("checking the token file: %v", err)
	}
	// Five presets, two schemes, three surfaces plus the text pair. Stated so
	// that a preset quietly dropped from the file is a failure here rather than
	// one palette fewer to walk.
	if examined != 40 {
		t.Fatalf("examined %d combinations of the published tokens, want 40", examined)
	}
	if len(found) != 0 {
		var lines []string
		for _, f := range found {
			lines = append(lines, f.String())
		}
		t.Fatalf("the focus colour does not stand off every surface:\n%s", strings.Join(lines, "\n"))
	}
}
