package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":         {Data: []byte("<html>app</html>")},
		"favicon.svg":        {Data: []byte("<svg/>")},
		"assets/app-1a2b.js": {Data: []byte("console.log(1)")},
	}
}

func get(t *testing.T, h http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestHandler_ServesFiles(t *testing.T) {
	h := Handler(testFS(), true)

	rec := get(t, h, http.MethodGet, "/assets/app-1a2b.js")
	if rec.Code != http.StatusOK || rec.Body.String() != "console.log(1)" {
		t.Fatalf("asset: code=%d body=%q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Errorf("asset Cache-Control = %q, want immutable", got)
	}

	rec = get(t, h, http.MethodGet, "/favicon.svg")
	if rec.Code != http.StatusOK {
		t.Fatalf("favicon: code=%d", rec.Code)
	}
}

func TestHandler_FallsBackToIndex(t *testing.T) {
	h := Handler(testFS(), true)
	for _, target := range []string{"/", "/nodes/gpu-01", "/assets"} {
		rec := get(t, h, http.MethodGet, target)
		if rec.Code != http.StatusOK || rec.Body.String() != "<html>app</html>" {
			t.Errorf("%s: code=%d body=%q, want index.html", target, rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
			t.Errorf("%s: Cache-Control = %q, want no-cache", target, got)
		}
	}
}

func TestHandler_MissingAssetIs404(t *testing.T) {
	h := Handler(testFS(), true)
	rec := get(t, h, http.MethodGet, "/assets/missing.js")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404", rec.Code)
	}
}

func TestHandler_RejectsOtherMethods(t *testing.T) {
	h := Handler(testFS(), true)
	rec := get(t, h, http.MethodPost, "/")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code = %d, want 405", rec.Code)
	}
}

func TestHandler_PlaceholderWithoutUI(t *testing.T) {
	h := Handler(nil, false)
	rec := get(t, h, http.MethodGet, "/nodes/gpu-01")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "make web-build") {
		t.Fatalf("code=%d body=%q", rec.Code, rec.Body.String())
	}
}
