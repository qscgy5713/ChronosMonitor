package router

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"

	"chronosmonitor/internal/broker"
	"chronosmonitor/internal/config"
	"chronosmonitor/internal/db"
	"chronosmonitor/internal/handlers"
	"chronosmonitor/internal/store"
)

func newTestRouter(t *testing.T, assets fstest.MapFS) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	conn, err := db.Open(config.Config{
		DBDriver: "sqlite",
		DBPath:   filepath.Join(t.TempDir(), "test.db"),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	hub := broker.New()
	taskHandler := handlers.NewTaskHandler(store.NewTaskStore(conn), hub)
	streamHandler := handlers.NewStreamHandler(hub)

	return New(taskHandler, streamHandler, assets)
}

func fakeAssets() fstest.MapFS {
	return fstest.MapFS{
		"index.html":    &fstest.MapFile{Data: []byte("<html><body>dashboard</body></html>")},
		"assets/app.js": &fstest.MapFile{Data: []byte("console.log('hi')")},
	}
}

func get(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestServesEmbeddedIndexAtRoot(t *testing.T) {
	r := newTestRouter(t, fakeAssets())

	w := get(t, r, "/")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "dashboard") {
		t.Errorf("body = %q, want it to contain the embedded index.html content", w.Body.String())
	}
}

func TestServesKnownStaticAsset(t *testing.T) {
	r := newTestRouter(t, fakeAssets())

	w := get(t, r, "/assets/app.js")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if w.Body.String() != "console.log('hi')" {
		t.Errorf("body = %q, want the embedded asset content", w.Body.String())
	}
}

// Regression test: an earlier implementation rewrote unknown paths to
// "/index.html" and handed them to http.FileServer, which 301-redirects any
// request ending in "index.html" to "./" — breaking every deep link and
// browser refresh. The fallback must serve the content directly with 200.
func TestUnknownPathFallsBackToIndexWithoutRedirect(t *testing.T) {
	r := newTestRouter(t, fakeAssets())

	w := get(t, r, "/some/deep/link")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (no redirect)", w.Code, http.StatusOK)
	}
	if loc := w.Header().Get("Location"); loc != "" {
		t.Errorf("Location header = %q, want no redirect", loc)
	}
	if !strings.Contains(w.Body.String(), "dashboard") {
		t.Errorf("body = %q, want the SPA index.html content", w.Body.String())
	}
}

func TestUnknownAPIPathReturnsJSON404NotIndexHTML(t *testing.T) {
	r := newTestRouter(t, fakeAssets())

	w := get(t, r, "/api/v1/does-not-exist")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
	if strings.Contains(w.Body.String(), "dashboard") {
		t.Error("body contains the SPA index.html; API 404s must not fall back to it")
	}
}

func TestKnownAPIRoutesAreNotShadowedByStaticFallback(t *testing.T) {
	r := newTestRouter(t, fakeAssets())

	w := get(t, r, "/healthz")
	if w.Code != http.StatusOK {
		t.Fatalf("/healthz status = %d, want %d", w.Code, http.StatusOK)
	}

	w = get(t, r, "/api/v1/tasks")
	if w.Code != http.StatusOK {
		t.Fatalf("/api/v1/tasks status = %d, want %d", w.Code, http.StatusOK)
	}
}
