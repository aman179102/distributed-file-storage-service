package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/distributed-file-storage/service/src/config"
	"github.com/distributed-file-storage/service/src/domain"
	"github.com/distributed-file-storage/service/src/infrastructure/database"
	"github.com/distributed-file-storage/service/src/infrastructure/storage"
)

type FileService struct {
	cfg        *config.Config
	fileRepo   *database.FileRepository
	bucketRepo *database.BucketRepository
	diskStore  *storage.DiskStore
	chunker    *storage.Chunker
	codec      *storage.ReedSolomonCodec
	replicator *storage.Replicator
	metrics    interface {
		RecordStorageOp(operation string, duration time.Duration, err error)
		SetStorageBytes(typeName string, bytes int64)
		SetObjectCount(status string, count int64)
	}
	auditRepo *database.AuditLogRepository
}

func NewFileService(
	cfg *config.Config,
	fileRepo *database.FileRepository,
	bucketRepo *database.BucketRepository,
	diskStore *storage.DiskStore,
	chunker *storage.Chunker,
	codec *storage.ReedSolomonCodec,
	replicator *storage.Replicator,
	metrics interface {
		RecordStorageOp(operation string, duration time.Duration, err error)
		SetStorageBytes(typeName string, bytes int64)
		SetObjectCount(status string, count int64)
	},
	auditRepo *database.AuditLogRepository,
) *FileService {
	return &FileService{
		cfg:        cfg,
		fileRepo:   fileRepo,
		bucketRepo: bucketRepo,
		diskStore:  diskStore,
		chunker:    chunker,
		codec:      codec,
		replicator: replicator,
		metrics:    metrics,
		auditRepo:  auditRepo,
	}
}

func (s *FileService) PutObject(bucketID, key, contentType, owner string, metadata map[string]string, data io.Reader, ttl *time.Duration) (*domain.FileInfo, error) {
	start := time.Now()

	bucket, err := s.bucketRepo.GetByID(bucketID)
	if err != nil {
		return nil, err
	}

	rawData, err := io.ReadAll(data)
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	if int64(len(rawData)) > s.cfg.Storage.MaxFileSize {
		return nil, domain.ErrEntityTooLarge
	}

	if bucket.MaxSizeBytes > 0 && bucket.CurrentSizeBytes+int64(len(rawData)) > bucket.MaxSizeBytes {
		return nil, domain.ErrStorageQuota
	}

	checksum := sha256.Sum256(rawData)
	etag := hex.EncodeToString(checksum[:])

	chunkResult, err := s.chunker.Split(bytes.NewReader(rawData))
	if err != nil {
		return nil, fmt.Errorf("failed to split file: %w", err)
	}

	var erasureChunks []*domain.FileChunk
	var erasureData [][]byte
	if s.cfg.Storage.ParityShards > 0 {
		erasureChunks, erasureData, err = s.codec.Encode(chunkResult.Data)
		if err != nil {
			return nil, fmt.Errorf("erasure coding failed: %w", err)
		}
	} else {
		erasureChunks = chunkResult.Chunks
		erasureData = chunkResult.Data
	}

	now := time.Now().UTC()
	fileID := uuid.New().String()
	versionID := uuid.New().String()

	for i, c := range erasureChunks {
		c.ID = uuid.New().String()
		c.FileID = fileID
		var chunkData []byte
		if i < len(erasureData) {
			chunkData = erasureData[i]
		}
		chunkPath := fmt.Sprintf("objects/%s/%s/%d", bucketID, fileID, c.ChunkIndex)
		c.StoragePath = chunkPath

		if _, _, err := s.diskStore.Write(chunkPath, bytes.NewReader(chunkData)); err != nil {
			return nil, fmt.Errorf("failed to write chunk %d: %w", i, err)
		}
	}

	storageSize := int64(len(rawData))
	if s.cfg.Storage.ParityShards > 0 {
		totalShards := s.cfg.Storage.DataShards + s.cfg.Storage.ParityShards
		storageSize = int64(len(rawData)) * int64(totalShards) / int64(s.cfg.Storage.DataShards)
	}

	if metadata == nil {
		metadata = map[string]string{}
	}

	var expiresAt *time.Time
	if ttl != nil {
		t := now.Add(*ttl)
		expiresAt = &t
	}

	fileInfo := &domain.FileInfo{
		ID:             fileID,
		BucketID:       bucketID,
		Key:            key,
		Size:           int64(len(rawData)),
		ETag:           etag,
		ContentType:    contentType,
		Metadata:       metadata,
		ChecksumSHA256: etag,
		StorageClass:   "STANDARD",
		VersionID:      versionID,
		Status:         domain.FileStatusActive,
		TTL:            ttl,
		ExpiresAt:      expiresAt,
		Owner:          owner,
		ChunkCount:     len(erasureChunks),
		StorageSize:    storageSize,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.fileRepo.Create(fileInfo); err != nil {
		return nil, fmt.Errorf("failed to create file record: %w", err)
	}

	for _, chunk := range erasureChunks {
		if err := s.fileRepo.CreateChunk(chunk); err != nil {
			slog.Warn("failed to save chunk record", "chunk_index", chunk.ChunkIndex, "error", err)
		}
	}

	if s.replicator != nil {
		if err := s.replicator.Replicate(fileID, erasureChunks, erasureData); err != nil {
			slog.Warn("replication partially failed", "file_id", fileID, "error", err)
		}
	}

	bucket.CurrentSizeBytes += int64(len(rawData))
	bucket.ObjectCount++
	_ = s.bucketRepo.Update(bucket)

	s.metrics.RecordStorageOp("put", time.Since(start), nil)
	s.metrics.SetStorageBytes("active", bucket.CurrentSizeBytes)
	s.metrics.SetObjectCount("active", bucket.ObjectCount)

	s.logAudit(owner, "PUT_OBJECT", "file", fileID, map[string]interface{}{
		"bucket_id": bucketID,
		"key":       key,
		"size":      len(rawData),
	})

	return fileInfo, nil
}

func (s *FileService) GetObject(bucketID, key, versionID string) (*domain.FileInfo, io.ReadCloser, error) {
	start := time.Now()
	defer func() {
		s.metrics.RecordStorageOp("get", time.Since(start), nil)
	}()

	file, err := s.fileRepo.GetByBucketAndKey(bucketID, key, versionID)
	if err != nil {
		return nil, nil, err
	}

	if file.IsExpired() {
		return nil, nil, domain.NewNotFound("file (expired)")
	}

	chunks, err := s.fileRepo.ListChunks(file.ID)
	if err != nil {
		slog.Warn("chunks not found in db, reading from disk")
	}

	var rawData []byte
	if len(chunks) > 0 {
		var chunksData [][]byte
		for _, chunk := range chunks {
			var buf bytes.Buffer
			if _, err := s.diskStore.Read(chunk.StoragePath, &buf); err != nil {
				return nil, nil, fmt.Errorf("failed to read chunk %d: %w", chunk.ChunkIndex, err)
			}
			chunksData = append(chunksData, buf.Bytes())
		}

		if s.cfg.Storage.ParityShards > 0 {
			decoded, err := s.codec.Decode(chunks, chunksData)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to decode erasure data: %w", err)
			}
			rawData = decoded
		} else {
			merged, err := s.chunker.Merge(chunksData)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to merge chunks: %w", err)
			}
			rawData = merged
		}
	} else {
		return nil, nil, domain.NewNotFound("file chunks")
	}

	s.logAudit("", "GET_OBJECT", "file", file.ID, map[string]interface{}{
		"bucket_id": bucketID,
		"key":       key,
	})

	return file, io.NopCloser(bytes.NewReader(rawData)), nil
}

func (s *FileService) GetObjectRange(bucketID, key, versionID string, offset, length int64) (*domain.FileInfo, io.ReadCloser, int64, error) {
	_, reader, err := s.GetObject(bucketID, key, versionID)
	if err != nil {
		return nil, nil, 0, err
	}
	defer reader.Close()

	rawData, err := io.ReadAll(reader)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to read object: %w", err)
	}

	if offset < 0 || offset >= int64(len(rawData)) {
		return nil, nil, 0, domain.NewInvalidInput("invalid range offset")
	}

	end := offset + length
	if length <= 0 || end > int64(len(rawData)) {
		end = int64(len(rawData))
	}

	file, _ := s.fileRepo.GetByBucketAndKey(bucketID, key, versionID)
	rangeData := rawData[offset:end]

	return file, io.NopCloser(bytes.NewReader(rangeData)), end - offset, nil
}

func (s *FileService) DeleteObject(bucketID, key string) error {
	start := time.Now()

	file, err := s.fileRepo.GetByBucketAndKey(bucketID, key, "")
	if err != nil {
		return err
	}

	chunks, err := s.fileRepo.ListChunks(file.ID)
	if err == nil {
		for _, chunk := range chunks {
			if err := s.diskStore.Delete(chunk.StoragePath); err != nil {
				slog.Warn("failed to delete chunk from disk", "path", chunk.StoragePath, "error", err)
			}
		}
	}

	if err := s.fileRepo.SoftDelete(file.ID); err != nil {
		return err
	}

	bucket, err := s.bucketRepo.GetByID(bucketID)
	if err == nil {
		bucket.CurrentSizeBytes -= file.Size
		if bucket.CurrentSizeBytes < 0 {
			bucket.CurrentSizeBytes = 0
		}
		bucket.ObjectCount--
		if bucket.ObjectCount < 0 {
			bucket.ObjectCount = 0
		}
		_ = s.bucketRepo.Update(bucket)
	}

	s.metrics.RecordStorageOp("delete", time.Since(start), nil)

	s.logAudit("", "DELETE_OBJECT", "file", file.ID, map[string]interface{}{
		"bucket_id": bucketID,
		"key":       key,
	})

	return nil
}

func (s *FileService) ListObjects(bucketID, prefix string, offset, limit int) ([]*domain.FileInfo, error) {
	return s.fileRepo.List(bucketID, prefix, offset, limit)
}

func (s *FileService) StartMultipartUpload(bucketID, key, contentType, initiator string, metadata map[string]string) (*domain.MultipartUpload, error) {
	upload := &domain.MultipartUpload{
		ID:          uuid.New().String(),
		BucketID:    bucketID,
		Key:         key,
		Initiator:   initiator,
		ContentType: contentType,
		Metadata:    metadata,
		UploadedParts: []domain.FilePart{},
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := s.createUploadRecord(upload); err != nil {
		return nil, err
	}

	return upload, nil
}

func (s *FileService) UploadPart(uploadID string, partNumber int, data io.Reader) (*domain.FilePart, error) {
	partData, err := io.ReadAll(data)
	if err != nil {
		return nil, fmt.Errorf("failed to read part data: %w", err)
	}

	checksum := sha256.Sum256(partData)
	etag := hex.EncodeToString(checksum[:])
	partETag := fmt.Sprintf("%s-%d", etag, partNumber)

	part := &domain.FilePart{
		PartNumber: partNumber,
		ETag:       partETag,
		Size:       int64(len(partData)),
		Checksum:   etag,
		UploadID:   uploadID,
	}

	partPath := fmt.Sprintf("multipart/%s/%d", uploadID, partNumber)
	if _, _, err := s.diskStore.Write(partPath, bytes.NewReader(partData)); err != nil {
		return nil, fmt.Errorf("failed to write part: %w", err)
	}

	if err := s.savePartRecord(part); err != nil {
		slog.Warn("failed to save part record", "error", err)
	}

	return part, nil
}

func (s *FileService) CompleteMultipartUpload(uploadID, bucketID, key, owner string, parts []domain.FilePart) (*domain.FileInfo, error) {
	var partsData [][]byte
	for _, part := range parts {
		partPath := fmt.Sprintf("multipart/%s/%d", uploadID, part.PartNumber)
		var buf bytes.Buffer
		if _, err := s.diskStore.Read(partPath, &buf); err != nil {
			return nil, fmt.Errorf("failed to read part %d: %w", part.PartNumber, err)
		}
		partsData = append(partsData, buf.Bytes())
	}

	rawData := storage.JoinParts(partsData)

	return s.PutObject(bucketID, key, "application/octet-stream", owner, nil, bytes.NewReader(rawData), nil)
}

func (s *FileService) ListMultipartUploads(bucketID string, offset, limit int) ([]*domain.MultipartUpload, error) {
	return s.listUploads(bucketID, offset, limit)
}

func (s *FileService) ListParts(uploadID string) ([]*domain.FilePart, error) {
	return s.listParts(uploadID)
}

func (s *FileService) AbortMultipartUpload(uploadID string) error {
	parts, err := s.listParts(uploadID)
	if err == nil {
		for _, part := range parts {
			partPath := fmt.Sprintf("multipart/%s/%d", uploadID, part.PartNumber)
			_ = s.diskStore.Delete(partPath)
		}
	}
	return s.deleteUpload(uploadID)
}

func (s *FileService) createUploadRecord(upload *domain.MultipartUpload) error {
	return nil
}

func (s *FileService) savePartRecord(part *domain.FilePart) error {
	return nil
}

func (s *FileService) listUploads(bucketID string, offset, limit int) ([]*domain.MultipartUpload, error) {
	return []*domain.MultipartUpload{}, nil
}

func (s *FileService) listParts(uploadID string) ([]*domain.FilePart, error) {
	return []*domain.FilePart{}, nil
}

func (s *FileService) deleteUpload(uploadID string) error {
	return nil
}

func (s *FileService) CleanupExpiredObjects() (int, error) {
	expiredFiles, err := s.fileRepo.ListExpired(1000)
	if err != nil {
		return 0, fmt.Errorf("failed to list expired files: %w", err)
	}

	count := 0
	for _, f := range expiredFiles {
		if err := s.fileRepo.SoftDelete(f.ID); err != nil {
			slog.Warn("failed to delete expired file", "id", f.ID, "error", err)
			continue
		}
		s.metrics.SetObjectCount("expired", 1)
		count++
	}

	if count > 0 {
		slog.Info("cleaned up expired objects", "count", count)
	}
	return count, nil
}

func (s *FileService) logAudit(userID, action, resourceType, resourceID string, details map[string]interface{}) {
	if s.auditRepo == nil {
		return
	}
	entry := &database.AuditLogEntry{
		ID:           uuid.New().String(),
		UserID:       userID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Details:      details,
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.auditRepo.Log(entry); err != nil {
		slog.Warn("failed to log audit entry", "error", err)
	}
}
