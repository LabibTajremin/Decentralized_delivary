// Package assets serves the demo imagery that ships with the binary.
//
// These are generated brand placeholders, not merchant-supplied media: they
// exist so a fresh clone has a product that looks like a product, without a
// CDN, an object store, or a network connection. Real merchant media is
// uploaded to object storage in P05 and never lives in the binary.
//
// Because they are demo assets, the handler refuses to mount in production.
// A production deployment that serves fake merchant logos is worse than one
// that serves none.
package assets

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:demo
var demo embed.FS

// ErrNotForProduction is returned when demo assets are requested in a
// production deployment.
var ErrNotForProduction = errors.New("demo assets are not served in production")

// Prefix is the URL path the demo assets are mounted under.
const Prefix = "/static/demo/"

// Root is the directory the assets live in inside the embedded filesystem. It
// is also the last path segment of Prefix, which is what lets the file server
// strip only "/static/" and let the FS supply the rest.
const Root = "demo"

// FS exposes the embedded demo assets. Paths inside it are "demo/merchants/…".
// Manifest paths are relative to Root, so callers join with Root themselves —
// or, more usually, let Handler and URL do it.
func FS() fs.FS { return demo }

// Entry describes one demo image in the manifest.
type Entry struct {
	Name     string `json:"name,omitempty"`
	Label    string `json:"label,omitempty"`
	Category string `json:"category,omitempty"`
	Logo     string `json:"logo,omitempty"`
	Cover    string `json:"cover,omitempty"`
	Tile     string `json:"tile,omitempty"`
	Image    string `json:"image,omitempty"`
}

// Manifest maps demo entity ids to their imagery. Seeds and demo fixtures read
// it rather than hard-coding filenames, so regenerating the images cannot leave
// a seed pointing at a file that no longer exists.
type Manifest struct {
	Merchants  map[string]Entry `json:"merchants"`
	Categories map[string]Entry `json:"categories"`
	Avatars    map[string]Entry `json:"avatars"`
}

// LoadManifest reads and validates the embedded manifest.
//
// Validation is the point: it checks every path it names is really embedded,
// so a manifest that has drifted from the files fails here rather than as a
// broken image in a demo.
func LoadManifest() (Manifest, error) { return ParseManifest(demo) }

// ParseManifest reads and validates a manifest from any filesystem.
//
// It takes the filesystem rather than reading the embedded one directly so its
// failure paths — a missing manifest, malformed JSON, an entry naming a file
// that is not there — are reachable from a test. A validator whose error
// handling has never been executed is not a validator.
func ParseManifest(fsys fs.FS) (Manifest, error) {
	raw, err := fs.ReadFile(fsys, path.Join(Root, "manifest.json"))
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	for id, e := range m.Merchants {
		for _, p := range []string{e.Logo, e.Cover} {
			if err := mustExist(fsys, id, p); err != nil {
				return Manifest{}, err
			}
		}
	}
	for id, e := range m.Categories {
		if err := mustExist(fsys, id, e.Tile); err != nil {
			return Manifest{}, err
		}
	}
	for id, e := range m.Avatars {
		if err := mustExist(fsys, id, e.Image); err != nil {
			return Manifest{}, err
		}
	}
	return m, nil
}

func mustExist(fsys fs.FS, id, rel string) error {
	if rel == "" {
		return fmt.Errorf("manifest entry %s has no image path", id)
	}
	if _, err := fs.Stat(fsys, path.Join(Root, rel)); err != nil {
		return fmt.Errorf("manifest entry %s points at missing file %s", id, rel)
	}
	return nil
}

// URL returns the absolute URL for a manifest path against a public base URL.
//
// The server builds this, not the client: the apps render whatever URL they
// are given rather than assembling one from a host and a filename, which is
// the same thin-client rule that applies to money (2.9).
func URL(baseURL, path string) string {
	return strings.TrimRight(baseURL, "/") + Prefix + strings.TrimLeft(path, "/")
}

// Handler serves the demo assets under Prefix.
//
// Assets are immutable — regenerating them is a new deploy — so they are served
// with a long max-age. That is what makes a demo over a slow connection load
// once rather than on every screen.
func Handler(production bool) (http.Handler, error) {
	if production {
		return nil, ErrNotForProduction
	}
	// Strip only "/static/": the embedded filesystem supplies the "demo/"
	// segment itself, so the two halves meet exactly at the URL Prefix names.
	files := http.FileServer(http.FS(demo))
	return http.StripPrefix(strings.TrimSuffix(Prefix, Root+"/"), cacheForever(files)), nil
}

func cacheForever(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		next.ServeHTTP(w, r)
	})
}
