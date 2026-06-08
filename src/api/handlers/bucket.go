package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/distributed-file-storage/service/src/api"
	"github.com/distributed-file-storage/service/src/core"
	"github.com/distributed-file-storage/service/src/domain"
)

type BucketHandler struct {
	bucketService *core.BucketService
}

func NewBucketHandler(bucketService *core.BucketService) *BucketHandler {
	return &BucketHandler{bucketService: bucketService}
}

func (h *BucketHandler) List(w http.ResponseWriter, r *http.Request) {
	owner := r.URL.Query().Get("owner")
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	buckets, err := h.bucketService.ListBuckets(owner, offset, limit)
	if err != nil {
		slog.Error("failed to list buckets", "error", err)
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list buckets")
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"buckets": buckets,
	})
}

func (h *BucketHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string            `json:"name"`
		Region       string            `json:"region"`
		MaxSizeBytes int64             `json:"max_size_bytes"`
		Tags         map[string]string `json:"tags"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "invalid request body")
		return
	}

	if req.Name == "" {
		api.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "bucket name is required")
		return
	}

	if req.Region == "" {
		req.Region = "us-east-1"
	}

	owner := api.GetUsername(r.Context())

	bucket, err := h.bucketService.CreateBucket(req.Name, owner, req.Region, req.MaxSizeBytes, req.Tags)
	if err != nil {
		if domainErr, ok := err.(*domain.DomainError); ok {
			status := http.StatusInternalServerError
			switch domainErr.Code {
			case domain.ErrAlreadyExists:
				status = http.StatusConflict
			case domain.ErrInvalidInput:
				status = http.StatusBadRequest
			}
			api.WriteError(w, status, string(domainErr.Code), domainErr.Message)
			return
		}
		slog.Error("failed to create bucket", "error", err)
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create bucket")
		return
	}

	api.WriteJSON(w, http.StatusCreated, bucket)
}

func (h *BucketHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		api.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "bucket id is required")
		return
	}

	bucket, err := h.bucketService.GetBucket(id)
	if err != nil {
		if domainErr, ok := err.(*domain.DomainError); ok {
			status := http.StatusNotFound
			if domainErr.Code == domain.ErrNotFound {
				status = http.StatusNotFound
			}
			api.WriteError(w, status, string(domainErr.Code), domainErr.Message)
			return
		}
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get bucket")
		return
	}

	api.WriteJSON(w, http.StatusOK, bucket)
}

func (h *BucketHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		api.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "bucket id is required")
		return
	}

	if err := h.bucketService.DeleteBucket(id); err != nil {
		if domainErr, ok := err.(*domain.DomainError); ok {
			status := http.StatusInternalServerError
			switch domainErr.Code {
			case domain.ErrNotFound:
				status = http.StatusNotFound
			case domain.ErrInvalidInput:
				status = http.StatusBadRequest
			}
			api.WriteError(w, status, string(domainErr.Code), domainErr.Message)
			return
		}
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete bucket")
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]string{"message": "bucket deleted"})
}

func (h *BucketHandler) SetVersioning(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		Versioning string `json:"versioning"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "invalid request body")
		return
	}

	versioning := domain.BucketVersioning(req.Versioning)
	if versioning != domain.VersioningEnabled && versioning != domain.VersioningDisabled && versioning != domain.VersioningSuspended {
		api.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "versioning must be 'enabled', 'disabled', or 'suspended'")
		return
	}

	if err := h.bucketService.UpdateBucketVersioning(id, versioning); err != nil {
		if domainErr, ok := err.(*domain.DomainError); ok {
			api.WriteError(w, http.StatusNotFound, string(domainErr.Code), domainErr.Message)
			return
		}
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update versioning")
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]string{"message": "versioning updated"})
}
