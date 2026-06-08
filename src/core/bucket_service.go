package core

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/distributed-file-storage/service/src/domain"
	"github.com/distributed-file-storage/service/src/infrastructure/auth"
	"github.com/distributed-file-storage/service/src/infrastructure/database"
)

type BucketService struct {
	bucketRepo *database.BucketRepository
	fileRepo   *database.FileRepository
	auditRepo  *database.AuditLogRepository
	rbac       *auth.RBACEngine
	metrics    interface {
		SetBucketCount(count int64)
	}
}

func NewBucketService(
	bucketRepo *database.BucketRepository,
	fileRepo *database.FileRepository,
	auditRepo *database.AuditLogRepository,
	rbac *auth.RBACEngine,
	metrics interface {
		SetBucketCount(count int64)
	},
) *BucketService {
	return &BucketService{
		bucketRepo: bucketRepo,
		fileRepo:   fileRepo,
		auditRepo:  auditRepo,
		rbac:       rbac,
		metrics:    metrics,
	}
}

func (s *BucketService) CreateBucket(name, owner, region string, maxSizeBytes int64, tags map[string]string) (*domain.Bucket, error) {
	if err := auth.ValidateBucketName(name); err != nil {
		return nil, err
	}

	existing, err := s.bucketRepo.GetByName(name)
	if err == nil && existing != nil {
		return nil, domain.NewAlreadyExists(fmt.Sprintf("bucket '%s'", name))
	}

	now := time.Now().UTC()
	bucket := &domain.Bucket{
		ID:               uuid.New().String(),
		Name:             name,
		Owner:            owner,
		Region:           region,
		Versioning:       domain.VersioningDisabled,
		ObjectLockEnabled: false,
		StorageClass:     "STANDARD",
		MaxSizeBytes:     maxSizeBytes,
		CurrentSizeBytes: 0,
		ObjectCount:      0,
		Tags:             tags,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := s.bucketRepo.Create(bucket); err != nil {
		return nil, err
	}

	s.metrics.SetBucketCount(1)

	s.logAudit(owner, "CREATE_BUCKET", "bucket", bucket.ID, map[string]interface{}{
		"name":   name,
		"region": region,
	})

	return bucket, nil
}

func (s *BucketService) GetBucket(id string) (*domain.Bucket, error) {
	return s.bucketRepo.GetByID(id)
}

func (s *BucketService) GetBucketByName(name string) (*domain.Bucket, error) {
	return s.bucketRepo.GetByName(name)
}

func (s *BucketService) ListBuckets(owner string, offset, limit int) ([]*domain.Bucket, error) {
	return s.bucketRepo.List(owner, offset, limit)
}

func (s *BucketService) DeleteBucket(id string) error {
	bucket, err := s.bucketRepo.GetByID(id)
	if err != nil {
		return err
	}

	count, _, err := s.fileRepo.CountByBucket(id)
	if err != nil {
		return fmt.Errorf("failed to count files in bucket: %w", err)
	}

	if count > 0 {
		return domain.NewInvalidInput("bucket not empty")
	}

	if err := s.bucketRepo.SoftDelete(id); err != nil {
		return err
	}

	s.logAudit(bucket.Owner, "DELETE_BUCKET", "bucket", id, map[string]interface{}{
		"name": bucket.Name,
	})

	return nil
}

func (s *BucketService) UpdateBucketVersioning(id string, versioning domain.BucketVersioning) error {
	bucket, err := s.bucketRepo.GetByID(id)
	if err != nil {
		return err
	}
	bucket.Versioning = versioning
	bucket.UpdatedAt = time.Now().UTC()
	return s.bucketRepo.Update(bucket)
}

func (s *BucketService) SetBucketPolicy(bucketID, policyJSON string) error {
	return nil
}

func (s *BucketService) GetBucketPolicy(bucketID string) (*domain.BucketPolicy, error) {
	return nil, nil
}

func (s *BucketService) ListAllBuckets() ([]*domain.Bucket, error) {
	return s.bucketRepo.List("", 0, 1000)
}

func (s *BucketService) logAudit(userID, action, resourceType, resourceID string, details map[string]interface{}) {
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
