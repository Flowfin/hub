// Package manifest holds the document a Jellyfin server reads to learn which
// plugins exist and where to download them, and the encoder that writes it.
//
// The field order of the two types below is the key order of the emitted file,
// and the encoder settings are the rest of its byte format. Both are fixed by
// decisions/manifest-schema.md, and manifest/testdata/golden-manifest.json is
// the specimen they are measured against.
package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// Plugin is one entry in the manifest. Struct field order is emitted key order,
// so reordering these fields changes every byte of every published entry.
type Plugin struct {
	GUID        string `json:"guid"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Overview    string `json:"overview"`
	Owner       string `json:"owner"`
	Category    string `json:"category"`
	// ImageURL is omitted rather than emitted as null when a plugin has no
	// artwork, which is what the entries without it in the ecosystem's own
	// catalogue do.
	ImageURL string    `json:"imageUrl,omitempty"`
	Versions []Version `json:"versions"`
}

// Version is one installable build of a plugin.
type Version struct {
	Version   string `json:"version"`
	Changelog string `json:"changelog"`
	TargetABI string `json:"targetAbi"`
	SourceURL string `json:"sourceUrl"`
	Checksum  string `json:"checksum"`
	Timestamp string `json:"timestamp"`
}

// Indent is the indentation the published file uses, four spaces, matching the
// catalogue this file sits beside.
const Indent = "    "

// Encode writes plugins as the manifest, in the byte format
// decisions/manifest-schema.md fixes.
//
// Two settings carry that format and neither is the default. SetEscapeHTML is
// turned off, because encoding/json otherwise rewrites an ampersand, a less-than
// and a greater-than as \u escapes, which produces a valid file that differs from
// the format in every changelog containing one. SetIndent supplies the four
// spaces. The trailing newline comes from Encode itself.
//
// An empty plugin list is written as an empty array rather than as null, so a
// caller holding nothing produces a file a server can read rather than one it
// cannot. Whether an empty list may be published at all is not this function's
// question; decisions/failure-posture.md answers it before the call is made.
func Encode(w io.Writer, plugins []Plugin) error {
	if plugins == nil {
		plugins = []Plugin{}
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", Indent)
	if err := enc.Encode(plugins); err != nil {
		return fmt.Errorf("encoding manifest: %w", err)
	}

	if _, err := w.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}
	return nil
}
