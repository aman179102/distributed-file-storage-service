package handlers

import (
	"encoding/json"
	"net/http"
	"time"
)

type HealthHandler struct {
	startTime time.Time
	version   string
}

func NewHealthHandler(version string) *HealthHandler {
	return &HealthHandler{
		startTime: time.Now(),
		version:   version,
	}
}

type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Uptime  int64  `json:"uptime"`
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	uptime := int64(time.Since(h.startTime).Seconds())

	resp := HealthResponse{
		Status:  "ok",
		Version: h.version,
		Uptime:  uptime,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
