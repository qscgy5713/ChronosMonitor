package router

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"chronosmonitor/internal/handlers"
)

func New(taskHandler *handlers.TaskHandler, streamHandler *handlers.StreamHandler, assets fs.FS) *gin.Engine {
	r := gin.Default()

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")
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
