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
	return newTestRouterWithAPIKey(t, assets, "")
}

func newTestRouterWithAPIKey(t *testing.T, assets fstest.MapFS, apiKey string) *gin.Engine {
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
	scheduleStore := store.NewScheduleStore(conn)
	taskHandler := handlers.NewTaskHandler(store.NewTaskStore(conn), scheduleStore, hub)
	streamHandler := handlers.NewStreamHandler(hub)
	scheduleHandler := handlers.NewScheduleHandler(scheduleStore)

	return New(taskHandler, streamHandler, scheduleHandler, assets, apiKey)
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

func TestAPIRoutesAreOpenWhenNoAPIKeyConfigured(t *testing.T) {
	r := newTestRouterWithAPIKey(t, fakeAssets(), "")

	w := get(t, r, "/api/v1/tasks")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (auth disabled by default)", w.Code, http.StatusOK)
	}
}

func TestAPIRoutesRejectMissingOrWrongKey(t *testing.T) {
	r := newTestRouterWithAPIKey(t, fakeAssets(), "correct-key")

	w := get(t, r, "/api/v1/tasks")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("no key: status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("wrong key: status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAPIRoutesAcceptKeyViaAuthorizationHeader(t *testing.T) {
	r := newTestRouterWithAPIKey(t, fakeAssets(), "correct-key")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	req.Header.Set("Authorization", "Bearer correct-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAPIRoutesAcceptKeyViaQueryParam(t *testing.T) {
	// EventSource can't set custom headers, so the SSE route must also accept
	// the key as a query param.
	r := newTestRouterWithAPIKey(t, fakeAssets(), "correct-key")

	w := get(t, r, "/api/v1/tasks?token=correct-key")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHealthzAndDashboardStayOpenWhenAPIKeyConfigured(t *testing.T) {
	r := newTestRouterWithAPIKey(t, fakeAssets(), "correct-key")

	w := get(t, r, "/healthz")
	if w.Code != http.StatusOK {
		t.Errorf("/healthz status = %d, want %d (must stay open for health checks)", w.Code, http.StatusOK)
	}

	w = get(t, r, "/")
	if w.Code != http.StatusOK {
		t.Errorf("/ status = %d, want %d (dashboard shell must load before it can prompt for a key)", w.Code, http.StatusOK)
	}
}

func TestScheduleRoutesAreProtectedByAPIKeyLikeEverythingElse(t *testing.T) {
	r := newTestRouterWithAPIKey(t, fakeAssets(), "correct-key")

	w := get(t, r, "/api/v1/schedules")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("/api/v1/schedules without key: status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/schedules", nil)
	req.Header.Set("Authorization", "Bearer correct-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("/api/v1/schedules with key: status = %d, want %d", w.Code, http.StatusOK)
	}
}
