package router

import (
	"crypto/subtle"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"chronosmonitor/internal/handlers"
)

// New builds the HTTP router. When apiKey is non-empty, every /api/v1/*
// request must present it as "Authorization: Bearer <key>" or "?token=<key>"
// (the latter exists because the browser's native EventSource API can't set
// custom headers, so the SSE stream has nothing else to authenticate with).
// /healthz and the embedded dashboard stay unauthenticated: health checks
// need to work regardless, and the SPA shell has to load before it can even
// prompt for a key.
func New(taskHandler *handlers.TaskHandler, streamHandler *handlers.StreamHandler, assets fs.FS, apiKey string) *gin.Engine {
	r := gin.Default()

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")
	v1.Use(requireAPIKey(apiKey))
	{
		events := v1.Group("/events")
		events.POST("/start", taskHandler.Start)
		events.POST("/heartbeat", taskHandler.Heartbeat)
		events.POST("/success", taskHandler.Success)
		events.POST("/failed", taskHandler.Failed)

		v1.GET("/tasks", taskHandler.List)
		v1.GET("/tasks/:runID", taskHandler.Get)

		v1.GET("/stream", streamHandler.Stream)
	}

	r.NoRoute(serveEmbeddedUI(assets))

	return r
}

// requireAPIKey rejects any request that doesn't present apiKey via the
// standard bearer-token header or (for clients that can't set headers, like
// EventSource) a "token" query param. An empty apiKey disables auth entirely
// — the default, so existing deployments aren't broken by upgrading.
func requireAPIKey(apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if apiKey == "" {
			c.Next()
			return
		}

		if !constantTimeEqual(bearerToken(c), apiKey) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or missing API key"})
			return
		}
		c.Next()
	}
}

func bearerToken(c *gin.Context) string {
	if auth := c.GetHeader("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return c.Query("token")
}

func constantTimeEqual(a, b string) bool {
	return len(a) == len(b) && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// serveEmbeddedUI serves the embedded Vue dashboard build for any request
// that didn't match an API route, falling back to index.html for unknown
// paths so the SPA can handle deep links and browser refreshes.
func serveEmbeddedUI(assets fs.FS) gin.HandlerFunc {
	fileServer := http.FileServer(http.FS(assets))

	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		path := strings.TrimPrefix(c.Request.URL.Path, "/")
		if path != "" {
			if _, err := fs.Stat(assets, path); err == nil {
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
		}

		// Unknown path (or "/"): serve index.html directly rather than
		// rewriting the request path and letting http.FileServer handle it —
		// FileServer 301-redirects any request whose path ends in
		// "index.html" to "./", which would break this fallback.
		index, err := fs.ReadFile(assets, "index.html")
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	}
}
