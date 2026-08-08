package sources

import (
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

// good is a record every test below starts from and breaks in one place, so a
// failing test names the one difference rather than a whole file.
const good = `{
    "account": "an-account",
    "repository": "jellyfin-plugin-thing",
    "slug": "thing",
    "stable_tags": "^v?[0-9]+\\.[0-9]+$",
    "enabled": true,
    "note": "why this record is the way it is"
}
`

func set(files map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for name, body := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(body)}
	}
	return fsys
}

func TestLoadReadsARecord(t *testing.T) {
	got, err := Load(set(map[string]string{"thing.json": good}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("read %d records, want 1", len(got))
	}
	d := got[0]
	if d.Path() != "an-account/jellyfin-plugin-thing" {
		t.Errorf("Path() = %q", d.Path())
	}
	if !d.On() {
		t.Error("the record decoded as disabled")
	}
	if !d.IsFinished("v1.2") || d.IsFinished("1.2-beta.1") {
		t.Error("stable_tags did not compile into the split it describes")
	}
}

// refusals are the shapes the loader has to refuse, each one the good record
// with a single thing wrong. Every entry is a mistake somebody makes rather than
// one invented to fill a table.
func TestLoadRefusesARecordThatIsWrong(t *testing.T) {
	cases := []struct {
		name    string
		file    string
		body    string
		expects string
	}{
		{
			name: "a field the loader does not accept",
			file: "thing.json",
			// A typo in a field name otherwise decodes cleanly and means
			// something other than what was written.
			body:    strings.Replace(good, `"stable_tags"`, `"stable_tag"`, 1),
			expects: "stable_tag",
		},
		{
			name: "no enabled field",
			file: "thing.json",
			// The one that has to be caught. A bool defaults to false, so a
			// record that forgot this decodes as a plugin quietly dropped from
			// the catalogue.
			body:    strings.Replace(good, "\"enabled\": true,\n    ", "", 1),
			expects: "declares no enabled",
		},
		{
			name:    "no account",
			file:    "thing.json",
			body:    strings.Replace(good, `"account": "an-account"`, `"account": ""`, 1),
			expects: "declares no account",
		},
		{
			name:    "a pattern that does not compile",
			file:    "thing.json",
			body:    strings.Replace(good, `^v?[0-9]+\\.[0-9]+$`, `^v?[0-9+$`, 1),
			expects: "does not compile",
		},
		{
			name:    "a file named for something other than its slug",
			file:    "other.json",
			body:    good,
			expects: "the file is named thing.json",
		},
		{
			name:    "a slug that is not a slug",
			file:    "Thing_One.json",
			body:    strings.Replace(good, `"slug": "thing"`, `"slug": "Thing_One"`, 1),
			expects: "lowercase words joined by hyphens",
		},
		{
			name:    "content after the record",
			file:    "thing.json",
			body:    good + "{}\n",
			expects: "content after the record",
		},
		{
			name:    "not JSON at all",
			file:    "thing.json",
			body:    "account: an-account\n",
			expects: "thing.json",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Load(set(map[string]string{c.file: c.body}))
			if err == nil {
				t.Fatalf("the set loaded with %s", c.name)
			}
			if !strings.Contains(err.Error(), c.expects) {
				t.Fatalf("the refusal does not say what is wrong: %v", err)
			}
		})
	}
}

func TestLoadRefusesTwoRecordsClaimingOneThing(t *testing.T) {
	// Two files declaring one repository publish it twice, and two declaring one
	// slug make every line of the run's output ambiguous about which is meant.
	twice := strings.Replace(good, `"slug": "thing"`, `"slug": "other"`, 1)
	_, err := Load(set(map[string]string{"thing.json": good, "other.json": twice}))
	if err == nil || !strings.Contains(err.Error(), "already declared by") {
		t.Fatalf("two records for one repository were accepted: %v", err)
	}

	sameSlug := strings.Replace(good, `"repository": "jellyfin-plugin-thing"`, `"repository": "jellyfin-plugin-other"`, 1)
	_, err = Load(set(map[string]string{"thing.json": good, "thing-again.json": sameSlug}))
	if err == nil {
		t.Fatal("two records for one slug were accepted")
	}
}

func TestLoadRefusesAnEmptyDirectory(t *testing.T) {
	// An empty declared set produces exactly the manifest a total failure
	// produces, so it is refused here rather than discovered at the verdict.
	if _, err := Load(set(map[string]string{"README.md": "not a record\n"})); err == nil {
		t.Fatal("a directory with no declaration loaded")
	}
}

func TestTheDeclaredSetInThisRepositoryLoads(t *testing.T) {
	// The leg that matters: the real sources/ directory, held to the shape by
	// the loader that is the authority for it.
	declarations, err := Load(os.DirFS("../../" + Dir))
	if err != nil {
		t.Fatalf("the declared set does not load:\n%v", err)
	}
	if len(declarations) == 0 {
		t.Fatal("the declared set is empty")
	}
	for _, d := range declarations {
		if d.Note == "" {
			t.Errorf("%s declares no note; the field that gets set wrong is the one whose reason lived in somebody's memory", d.Slug)
		}
	}
}
