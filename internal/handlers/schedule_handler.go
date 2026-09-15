package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"chronosmonitor/internal/models"
	"chronosmonitor/internal/store"
)

type ScheduleHandler struct {
	store *store.ScheduleStore
}

func NewScheduleHandler(s *store.ScheduleStore) *ScheduleHandler {
	return &ScheduleHandler{store: s}
}

// Register creates or updates a schedule expectation: "task_name should
// start at least once every expected_interval_seconds (+ grace_period_seconds)".
func (h *ScheduleHandler) Register(c *gin.Context) {
	var req models.RegisterScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.ExpectedIntervalSeconds <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expected_interval_seconds must be positive"})
		return
	}
	if req.GracePeriodSeconds < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "grace_period_seconds must not be negative"})
		return
	}

	if err := h.store.Upsert(req.TaskName, req.ExpectedIntervalSeconds, req.GracePeriodSeconds, time.Now().UTC()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register schedule"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ScheduleHandler) List(c *gin.Context) {
	schedules, err := h.store.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list schedules"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"schedules": schedules})
}

func (h *ScheduleHandler) Delete(c *gin.Context) {
	if err := h.store.Delete(c.Param("taskName")); err != nil {
		respondStoreErr(c, err, "schedule not found")
		return
	}
	c.Status(http.StatusNoContent)
}
