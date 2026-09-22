package platform

import (
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/rootlogic-lab/delivery/backend/internal/platform/assets"
)

// TestDemoAssetsAreNotServedInProduction is the guard that matters. Generated
// placeholder logos beside real merchants would be indistinguishable to a
// customer, so the handler refuses to exist rather than relying on nobody
// linking to it.
func TestDemoAssetsAreNotServedInProduction(t *testing.T) {
	h, err := assets.Handler(true)
	if !errors.Is(err, assets.ErrNotForProduction) {
		t.Fatalf("error = %v, want ErrNotForProduction", err)
	}
	if h != nil {
		t.Error("no handler may be returned for production")
	}
}

func TestHandlerServesAnEmbeddedImage(t *testing.T) {
	h, err := assets.Handler(false)
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		assets.Prefix+"merchants/MER-DEMO-0001-logo.png", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/png") {
		t.Errorf("content-type = %q, want image/png", ct)
	}
	body := rec.Body.Bytes()
	if len(body) < 1024 {
		t.Errorf("body is %d bytes, too small to be the image", len(body))
	}
	// The PNG signature: proves a real image came back, not an error page.
	if string(body[1:4]) != "PNG" {
		t.Errorf("body is not a PNG: % x", body[:8])
	}
}

// TestServedAssetsAreCacheable: the images never change without a deploy, and
// a demo over a slow connection should load them once, not per screen.
func TestServedAssetsAreCacheable(t *testing.T) {
	h, _ := assets.Handler(false)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		assets.Prefix+"categories/food.png", nil))
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("cache-control = %q, want an immutable long max-age", cc)
	}
}

func TestHandlerReportsAMissingAsset(t *testing.T) {
	h, _ := assets.Handler(false)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, assets.Prefix+"merchants/nope.png", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// TestManifestMatchesTheEmbeddedFiles is the check that keeps the manifest
// honest. A seed reads filenames from here, so a manifest that has drifted
// from the images is a demo full of broken pictures.
func TestManifestMatchesTheEmbeddedFiles(t *testing.T) {
	m, err := assets.LoadManifest()
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if len(m.Merchants) == 0 || len(m.Categories) == 0 || len(m.Avatars) == 0 {
		t.Fatalf("manifest is missing a section: %+v", m)
	}
	for id, e := range m.Merchants {
		if e.Name == "" || e.Category == "" {
			t.Errorf("merchant %s = %+v, want a name and a category", id, e)
		}
	}
}

// TestEveryEmbeddedImageIsInTheManifest catches the other direction: a file
// added without a manifest entry is a file nothing can reference.
func TestEveryEmbeddedImageIsInTheManifest(t *testing.T) {
	m, err := assets.LoadManifest()
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	named := map[string]bool{}
	for _, e := range m.Merchants {
		named[e.Logo], named[e.Cover] = true, true
	}
	for _, e := range m.Categories {
		named[e.Tile] = true
	}
	for _, e := range m.Avatars {
		named[e.Image] = true
	}

	err = fs.WalkDir(assets.FS(), assets.Root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel := strings.TrimPrefix(p, assets.Root+"/")
		if d.IsDir() || rel == "manifest.json" {
			return nil
		}
		if !named[rel] {
			t.Errorf("%s is embedded but no manifest entry references it", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

// The manifest is what stops a seed pointing at an image that is not there, so
// its own failure paths are worth asserting: a validator that has never
// rejected anything is not known to reject anything.

func manifestFS(t *testing.T, body string) fstest.MapFS {
	t.Helper()
	return fstest.MapFS{
		assets.Root + "/manifest.json":            {Data: []byte(body)},
		assets.Root + "/merchants/real-logo.png":  {Data: []byte("PNG")},
		assets.Root + "/merchants/real-cover.png": {Data: []byte("PNG")},
	}
}

func TestParseManifestReportsAMissingFile(t *testing.T) {
	if _, err := assets.ParseManifest(fstest.MapFS{}); err == nil {
		t.Error("ParseManifest must fail when there is no manifest")
	}
}

func TestParseManifestReportsMalformedJSON(t *testing.T) {
	_, err := assets.ParseManifest(manifestFS(t, "{not json"))
	if err == nil || !strings.Contains(err.Error(), "parse manifest") {
		t.Errorf("error = %v, want a parse failure", err)
	}
}

func TestParseManifestRejectsAMerchantPointingAtAMissingImage(t *testing.T) {
	_, err := assets.ParseManifest(manifestFS(t, `{"merchants":{"MER-1":
		{"name":"Demo","category":"food","logo":"merchants/gone.png","cover":"merchants/real-cover.png"}}}`))
	if err == nil || !strings.Contains(err.Error(), "merchants/gone.png") {
		t.Errorf("error = %v, want the missing file named", err)
	}
}

func TestParseManifestRejectsAnEntryWithNoImagePath(t *testing.T) {
	_, err := assets.ParseManifest(manifestFS(t, `{"merchants":{"MER-1":{"name":"Demo"}}}`))
	if err == nil || !strings.Contains(err.Error(), "no image path") {
		t.Errorf("error = %v, want an empty path rejected", err)
	}
}

func TestParseManifestRejectsACategoryPointingAtAMissingTile(t *testing.T) {
	_, err := assets.ParseManifest(manifestFS(t, `{"categories":{"food":{"label":"Food","tile":"categories/gone.png"}}}`))
	if err == nil || !strings.Contains(err.Error(), "categories/gone.png") {
		t.Errorf("error = %v, want the missing tile named", err)
	}
}

func TestParseManifestRejectsAnAvatarPointingAtAMissingImage(t *testing.T) {
	_, err := assets.ParseManifest(manifestFS(t, `{"avatars":{"USR-1":{"name":"Demo","image":"avatars/gone.png"}}}`))
	if err == nil || !strings.Contains(err.Error(), "avatars/gone.png") {
		t.Errorf("error = %v, want the missing avatar named", err)
	}
}

func TestParseManifestAcceptsAConsistentManifest(t *testing.T) {
	m, err := assets.ParseManifest(manifestFS(t, `{"merchants":{"MER-1":
		{"name":"Demo","category":"food","logo":"merchants/real-logo.png","cover":"merchants/real-cover.png"}}}`))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if m.Merchants["MER-1"].Name != "Demo" {
		t.Errorf("manifest = %+v", m)
	}
}

// TestEveryDemoMerchantHasImagery ties the imagery to the seed: the seed
// inserts merchant ids, and a demo where half of them have no logo looks
// broken.
func TestEveryDemoMerchantHasImagery(t *testing.T) {
	m, err := assets.LoadManifest()
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	scripts, err := seedSQL()
	if err != nil {
		t.Fatalf("read seed: %v", err)
	}
	for id := range m.Merchants {
		if !strings.Contains(scripts, id) {
			t.Errorf("manifest has imagery for %s, which the seed never inserts", id)
		}
	}
	for _, id := range demoMerchantIDs(scripts) {
		if _, ok := m.Merchants[id]; !ok {
			t.Errorf("seed inserts %s, which has no demo imagery", id)
		}
	}
}

func TestURLBuildsAnAbsoluteAddress(t *testing.T) {
	got := assets.URL("https://api.goklay.com/", "merchants/MER-DEMO-0001-logo.png")
	want := "https://api.goklay.com/static/demo/merchants/MER-DEMO-0001-logo.png"
	if got != want {
		t.Errorf("URL = %q, want %q", got, want)
	}
	if got := assets.URL("http://localhost:8080", "/categories/food.png"); got != "http://localhost:8080/static/demo/categories/food.png" {
		t.Errorf("URL = %q", got)
	}
}
