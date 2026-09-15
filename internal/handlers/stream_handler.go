package handlers

import (
	"io"

	"github.com/gin-gonic/gin"

	"chronosmonitor/internal/broker"
)

type StreamHandler struct {
	hub *broker.Hub
}

func NewStreamHandler(hub *broker.Hub) *StreamHandler {
	return &StreamHandler{hub: hub}
}

// Stream serves task lifecycle events as Server-Sent Events, so a dashboard
// can react to state changes without polling.
func (h *StreamHandler) Stream(c *gin.Context) {
	ch, unsubscribe := h.hub.Subscribe()
	defer unsubscribe()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	c.Stream(func(w io.Writer) bool {
		select {
		case event, ok := <-ch:
			if !ok {
				return false
			}
			c.SSEvent(event.Type, event.Task)
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}
