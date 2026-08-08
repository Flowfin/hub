// Package sources reads the declared set of source repositories.
//
// decisions/source-set-declaration.md is the argument for why the set is data
// in this repository rather than a list in the source or a query against the
// API, and it hands the exact shape of a declaration to this loader rather than
// stating it there. This file is therefore the authority for that shape, and the
// fields below are it.
//
// Every field a declaration carries is a fact this repository states. Nothing a
// release already supplies is here: not the version, not the download URL, not
// the checksum, not the plugin's identity.
package sources

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
)

// Dir is where the declarations live, at the root of this repository.
const Dir = "sources"

// Declaration is one plugin's record.
//
// Enabled is a pointer because the zero value of a bool is the dangerous answer.
// A record that forgot the field would decode as disabled, and a plugin silently
// dropped from the catalogue is the failure this whole decision exists against,
// so the field is required and its absence is refused.
type Declaration struct {
	Account    string `json:"account"`
	Repository string `json:"repository"`
	Slug       string `json:"slug"`
	StableTags string `json:"stable_tags"`
	Enabled    *bool  `json:"enabled"`
	Note       string `json:"note,omitempty"`

	// stable is the compiled StableTags, so that a caller never recompiles and
	// never has to handle a pattern error the loader has already refused.
	stable *regexp.Regexp
}

// Path is the declared repository as the API addresses it.
func (d Declaration) Path() string { return d.Account + "/" + d.Repository }

// On reports whether this record is read on this run.
func (d Declaration) On() bool { return d.Enabled != nil && *d.Enabled }

// IsFinished reports whether a release tag enters the finished list.
//
// decisions/channel-model.md is why this is the tag and not the release's
// pre-release flag: the two disagree on eleven of fifty-four releases in the one
// repository with a long history, and the flag is a display setting somebody can
// move afterwards while the tag is chosen with the intent and never changes.
func (d Declaration) IsFinished(tag string) bool { return d.stable.MatchString(tag) }

// slugPattern is what a slug may be. It is the name the project uses for a
// plugin in its own output and it is also a file name, so it is held to
// something that is both.
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Load reads every declaration in fsys and refuses the set if any record is
// wrong.
//
// It refuses rather than skips, and it refuses the whole set rather than the bad
// record. decisions/failure-posture.md puts a declaration that does not parse,
// or that carries a field the loader does not accept, in the fatal column: the
// declared set is this repository's own statement about what the catalogue is,
// and a run that guessed past a broken statement would publish a catalogue
// nobody declared.
func Load(fsys fs.FS) ([]Declaration, error) {
	names, err := fs.Glob(fsys, "*.json")
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", Dir, err)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("%s declares no plugin; a catalogue with no declared source is a decision somebody makes rather than one a run infers", Dir)
	}
	sort.Strings(names)

	var (
		out      []Declaration
		problems []string
		bySlug   = map[string]string{}
		byPath   = map[string]string{}
	)

	for _, name := range names {
		body, err := fs.ReadFile(fsys, name)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", name, err))
			continue
		}

		d, err := decode(name, body)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}

		if first, seen := bySlug[d.Slug]; seen {
			problems = append(problems, fmt.Sprintf("%s: slug %q is already declared by %s", name, d.Slug, first))
			continue
		}
		if first, seen := byPath[d.Path()]; seen {
			problems = append(problems, fmt.Sprintf("%s: %s is already declared by %s", name, d.Path(), first))
			continue
		}
		bySlug[d.Slug], byPath[d.Path()] = name, name
		out = append(out, d)
	}

	if len(problems) > 0 {
		return nil, fmt.Errorf("the declared source set is not usable:\n  %s", strings.Join(problems, "\n  "))
	}
	return out, nil
}

// decode reads one record and holds it to the shape.
func decode(name string, body []byte) (Declaration, error) {
	var d Declaration

	dec := json.NewDecoder(bytes.NewReader(body))
	// A field the loader does not accept is refused rather than ignored. A typo
	// in a field name is otherwise a record that decodes cleanly and means
	// something other than what was written.
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return d, fmt.Errorf("%s: %v", name, err)
	}
	// One record per file, and anything after it is refused rather than dropped.
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return d, fmt.Errorf("%s: content after the record; the declarations are one plugin per file", name)
	}

	var missing []string
	for _, f := range []struct{ name, value string }{
		{"account", d.Account},
		{"repository", d.Repository},
		{"slug", d.Slug},
		{"stable_tags", d.StableTags},
	} {
		if strings.TrimSpace(f.value) == "" {
			missing = append(missing, f.name)
		}
	}
	if d.Enabled == nil {
		missing = append(missing, "enabled")
	}
	if len(missing) > 0 {
		return d, fmt.Errorf("%s: declares no %s", name, strings.Join(missing, ", no "))
	}

	if !slugPattern.MatchString(d.Slug) {
		return d, fmt.Errorf("%s: slug %q is not lowercase words joined by hyphens", name, d.Slug)
	}
	if want := d.Slug + ".json"; path.Base(name) != want {
		return d, fmt.Errorf("%s: declares slug %q, so the file is named %s", name, d.Slug, want)
	}

	// A pattern that does not compile is refused here rather than at the first
	// release it is asked about, because the run that met it would otherwise
	// have already reported on every plugin ahead of this one.
	stable, err := regexp.Compile(d.StableTags)
	if err != nil {
		return d, fmt.Errorf("%s: stable_tags does not compile: %v", name, err)
	}
	d.stable = stable

	return d, nil
}
