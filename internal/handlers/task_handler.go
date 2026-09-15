package handlers

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"chronosmonitor/internal/broker"
	"chronosmonitor/internal/models"
	"chronosmonitor/internal/store"
)

type TaskHandler struct {
	store         *store.TaskStore
	scheduleStore *store.ScheduleStore
	hub           *broker.Hub
}

func NewTaskHandler(s *store.TaskStore, scheduleStore *store.ScheduleStore, hub *broker.Hub) *TaskHandler {
	return &TaskHandler{store: s, scheduleStore: scheduleStore, hub: hub}
}

func (h *TaskHandler) Start(c *gin.Context) {
	var req models.StartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	runID := req.RunID
	if runID == "" {
		runID = uuid.NewString()
	}

	task := models.TaskRun{
		RunID:      runID,
		TaskName:   req.TaskName,
		Source:     req.Source,
		Status:     models.StatusRunning,
		StartedAt:  time.Now().UTC(),
		TTLSeconds: req.TTLSeconds,
	}

	if err := h.store.Create(task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record task start"})
		return
	}

	task.LastHeartbeatAt = &task.StartedAt
	h.hub.Publish(broker.Event{Type: broker.EventStarted, Task: task})

	// Best-effort: if this task_name has a registered schedule, this start
	// clears any "missed" state and resets the deadline. A failure here
	// shouldn't fail the request — the run was already recorded.
	if err := h.scheduleStore.Touch(req.TaskName, task.StartedAt); err != nil {
		log.Printf("failed to touch schedule for task_name=%q: %v", req.TaskName, err)
	}

	c.JSON(http.StatusCreated, gin.H{"run_id": runID})
}

func (h *TaskHandler) Heartbeat(c *gin.Context) {
	var req models.HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.store.Heartbeat(req.RunID, time.Now().UTC()); err != nil {
		respondStoreErr(c, err, "task run not found")
		return
	}

	h.publishLatest(req.RunID, broker.EventHeartbeat)
	c.Status(http.StatusNoContent)
}

func (h *TaskHandler) Success(c *gin.Context) {
	var req models.SuccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.store.Finish(req.RunID, models.StatusSuccess, "", time.Now().UTC()); err != nil {
		respondStoreErr(c, err, "task run not found")
		return
	}

	h.publishLatest(req.RunID, broker.EventSucceeded)
	c.Status(http.StatusNoContent)
}

func (h *TaskHandler) Failed(c *gin.Context) {
	var req models.FailedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.store.Finish(req.RunID, models.StatusFailed, req.ErrorMessage, time.Now().UTC()); err != nil {
		respondStoreErr(c, err, "task run not found")
		return
	}

	h.publishLatest(req.RunID, broker.EventFailed)
	c.Status(http.StatusNoContent)
}

func (h *TaskHandler) Get(c *gin.Context) {
	task, err := h.store.Get(c.Param("runID"))
	if err != nil {
		respondStoreErr(c, err, "task run not found")
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *TaskHandler) List(c *gin.Context) {
	tasks, err := h.store.List(c.Query("status"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list tasks"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}

// publishLatest re-reads the task's current state and broadcasts it. Errors
// are swallowed: a broadcast failure shouldn't fail the API request that
// already succeeded in the store.
func (h *TaskHandler) publishLatest(runID, eventType string) {
	task, err := h.store.Get(runID)
	if err != nil {
		return
	}
	h.hub.Publish(broker.Event{Type: eventType, Task: *task})
}

func respondStoreErr(c *gin.Context, err error, notFoundMsg string) {
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": notFoundMsg})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
}
