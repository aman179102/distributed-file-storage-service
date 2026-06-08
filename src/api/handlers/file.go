package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/distributed-file-storage/service/src/api"
	"github.com/distributed-file-storage/service/src/core"
	"github.com/distributed-file-storage/service/src/domain"
)

type FileHandler struct {
	fileService *core.FileService
}

func NewFileHandler(fileService *core.FileService) *FileHandler {
	return &FileHandler{fileService: fileService}
}

func (h *FileHandler) Put(w http.ResponseWriter, r *http.Request) {
	bucketName := r.PathValue("bucket")
	key := r.PathValue("key")

	if bucketName == "" || key == "" {
		api.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "bucket and key are required")
		return
	}

	bucketID := bucketName
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	metadata := make(map[string]string)
	for k, v := range r.Header {
		if strings.HasPrefix(strings.ToLower(k), "x-amz-meta-") {
			metadata[strings.TrimPrefix(strings.ToLower(k), "x-amz-meta-")] = v[0]
		}
	}

	owner := api.GetUsername(r.Context())

	file, err := h.fileService.PutObject(bucketID, key, contentType, owner, metadata, r.Body, nil)
	if err != nil {
		if domainErr, ok := err.(*domain.DomainError); ok {
			status := http.StatusInternalServerError
			switch domainErr.Code {
			case domain.ErrStorageQuota:
				status = http.StatusInsufficientStorage
			case domain.ErrEntityTooLarge:
				status = http.StatusRequestEntityTooLarge
			case domain.ErrInvalidInput:
				status = http.StatusBadRequest
			}
			api.WriteError(w, status, string(domainErr.Code), domainErr.Message)
			return
		}
		slog.Error("failed to put object", "error", err)
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to store object")
		return
	}

	w.Header().Set("ETag", file.ETag)
	w.Header().Set("Content-Type", file.ContentType)
	api.WriteJSON(w, http.StatusCreated, file)
}

func (h *FileHandler) Get(w http.ResponseWriter, r *http.Request) {
	bucketName := r.PathValue("bucket")
	key := r.PathValue("key")
	versionID := r.URL.Query().Get("versionId")

	if bucketName == "" || key == "" {
		api.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "bucket and key are required")
		return
	}

	bucketID := bucketName
	rangeHeader := r.Header.Get("Range")

	if rangeHeader != "" {
		var offset, length int64
		_, err := fmt.Sscanf(rangeHeader, "bytes=%d-%d", &offset, &length)
		if err != nil {
			_, err = fmt.Sscanf(rangeHeader, "bytes=%d-", &offset)
			if err != nil {
				api.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "invalid range header")
				return
			}
			length = -1
		}
		if length > 0 {
			length = length - offset + 1
		}

		file, reader, contentLength, err := h.fileService.GetObjectRange(bucketID, key, versionID, offset, length)
		if err != nil {
			api.WriteError(w, http.StatusRangeNotSatisfiable, "RANGE_NOT_SATISFIABLE", "requested range not available")
			return
		}
		defer reader.Close()

		w.Header().Set("Content-Type", file.ContentType)
		w.Header().Set("ETag", file.ETag)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", offset, offset+contentLength-1, file.Size))
		w.Header().Set("Accept-Ranges", "bytes")
		w.WriteHeader(http.StatusPartialContent)
		io.Copy(w, reader)
		return
	}

	file, reader, err := h.fileService.GetObject(bucketID, key, versionID)
	if err != nil {
		if domainErr, ok := err.(*domain.DomainError); ok {
			api.WriteError(w, http.StatusNotFound, string(domainErr.Code), domainErr.Message)
			return
		}
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get object")
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(file.Size, 10))
	w.Header().Set("ETag", file.ETag)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Last-Modified", file.UpdatedAt.UTC().Format(http.TimeFormat))

	if file.Metadata != nil {
		for k, v := range file.Metadata {
			w.Header().Set("X-Amz-Meta-"+k, v)
		}
	}

	w.WriteHeader(http.StatusOK)
	io.Copy(w, reader)
}

func (h *FileHandler) Head(w http.ResponseWriter, r *http.Request) {
	bucketName := r.PathValue("bucket")
	key := r.PathValue("key")

	bucketID := bucketName
	file, _, err := h.fileService.GetObject(bucketID, key, "")
	if err != nil {
		api.WriteError(w, http.StatusNotFound, "NOT_FOUND", "object not found")
		return
	}

	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(file.Size, 10))
	w.Header().Set("ETag", file.ETag)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Last-Modified", file.UpdatedAt.UTC().Format(http.TimeFormat))
	w.WriteHeader(http.StatusOK)
}

func (h *FileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	bucketName := r.PathValue("bucket")
	key := r.PathValue("key")

	bucketID := bucketName
	if err := h.fileService.DeleteObject(bucketID, key); err != nil {
		if domainErr, ok := err.(*domain.DomainError); ok {
			api.WriteError(w, http.StatusNotFound, string(domainErr.Code), domainErr.Message)
			return
		}
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete object")
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]string{"message": "object deleted"})
}

func (h *FileHandler) List(w http.ResponseWriter, r *http.Request) {
	bucketName := r.PathValue("bucket")
	prefix := r.URL.Query().Get("prefix")
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	bucketID := bucketName
	files, err := h.fileService.ListObjects(bucketID, prefix, offset, limit)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list objects")
		return
	}

	type objectResponse struct {
		Key          string `json:"key"`
		Size         int64  `json:"size"`
		ETag         string `json:"etag"`
		ContentType  string `json:"content_type"`
		StorageClass string `json:"storage_class"`
		Owner        string `json:"owner"`
		LastModified string `json:"last_modified"`
	}

	var objects []objectResponse
	for _, f := range files {
		objects = append(objects, objectResponse{
			Key:          f.Key,
			Size:         f.Size,
			ETag:         f.ETag,
			ContentType:  f.ContentType,
			StorageClass: f.StorageClass,
			Owner:        f.Owner,
			LastModified: f.UpdatedAt.UTC().Format(http.TimeFormat),
		})
	}

	total := len(objects)
	api.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"objects": objects,
		"meta": api.Meta{
			Page:    offset/limit + 1,
			PerPage: limit,
			Total:   total,
		},
	})
}

func (h *FileHandler) StartMultipartUpload(w http.ResponseWriter, r *http.Request) {
	bucketName := r.PathValue("bucket")
	key := r.URL.Query().Get("key")

	if bucketName == "" || key == "" {
		api.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "bucket and key are required")
		return
	}

	var req struct {
		ContentType string            `json:"content_type"`
		Metadata    map[string]string `json:"metadata"`
	}
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req)
	}
	if req.ContentType == "" {
		req.ContentType = "application/octet-stream"
	}

	initiator := api.GetUsername(r.Context())
	upload, err := h.fileService.StartMultipartUpload(bucketName, key, req.ContentType, initiator, req.Metadata)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to start multipart upload")
		return
	}

	api.WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"upload_id":  upload.ID,
		"bucket":     bucketName,
		"key":        key,
		"initiator":  initiator,
	})
}

func (h *FileHandler) UploadPart(w http.ResponseWriter, r *http.Request) {
	uploadID := r.PathValue("uploadID")
	partNumberStr := r.URL.Query().Get("partNumber")
	partNumber, err := strconv.Atoi(partNumberStr)
	if err != nil || partNumber < 1 || partNumber > 10000 {
		api.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "invalid part number (1-10000)")
		return
	}

	part, err := h.fileService.UploadPart(uploadID, partNumber, r.Body)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to upload part")
		return
	}

	w.Header().Set("ETag", part.ETag)
	api.WriteJSON(w, http.StatusOK, part)
}

func (h *FileHandler) CompleteMultipartUpload(w http.ResponseWriter, r *http.Request) {
	uploadID := r.PathValue("uploadID")
	bucketName := r.PathValue("bucket")
	key := r.URL.Query().Get("key")

	var req struct {
		Parts []struct {
			PartNumber int    `json:"part_number"`
			ETag       string `json:"etag"`
		} `json:"parts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "invalid request body")
		return
	}

	var parts []domain.FilePart
	for _, p := range req.Parts {
		parts = append(parts, domain.FilePart{
			PartNumber: p.PartNumber,
			ETag:       p.ETag,
		})
	}

	owner := api.GetUsername(r.Context())
	file, err := h.fileService.CompleteMultipartUpload(uploadID, bucketName, key, owner, parts)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to complete multipart upload")
		return
	}

	api.WriteJSON(w, http.StatusOK, file)
}

func (h *FileHandler) AbortMultipartUpload(w http.ResponseWriter, r *http.Request) {
	uploadID := r.PathValue("uploadID")

	if err := h.fileService.AbortMultipartUpload(uploadID); err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to abort multipart upload")
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]string{"message": "upload aborted"})
}

func (h *FileHandler) ListMultipartUploads(w http.ResponseWriter, r *http.Request) {
	bucketName := r.PathValue("bucket")
	uploads, err := h.fileService.ListMultipartUploads(bucketName, 0, 100)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list multipart uploads")
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"uploads": uploads,
	})
}
