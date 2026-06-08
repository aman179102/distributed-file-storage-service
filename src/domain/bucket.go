package domain

import (
	"time"
)

type BucketVersioning string

const (
	VersioningDisabled   BucketVersioning = "disabled"
	VersioningEnabled    BucketVersioning = "enabled"
	VersioningSuspended  BucketVersioning = "suspended"
)

type Bucket struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Owner            string            `json:"owner"`
	Region           string            `json:"region"`
	Versioning       BucketVersioning  `json:"versioning"`
	ObjectLockEnabled bool             `json:"object_lock_enabled"`
	StorageClass     string            `json:"storage_class"`
	MaxSizeBytes     int64             `json:"max_size_bytes"`
	CurrentSizeBytes int64             `json:"current_size_bytes"`
	ObjectCount      int64             `json:"object_count"`
	Tags             map[string]string `json:"tags"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
	DeletedAt        *time.Time        `json:"deleted_at,omitempty"`
}

type BucketPolicy struct {
	ID        string    `json:"id"`
	BucketID  string    `json:"bucket_id"`
	Policy    string    `json:"policy"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (b *Bucket) IsVersioned() bool {
	return b.Versioning == VersioningEnabled
}

func (b *Bucket) AvailableBytes() int64 {
	if b.MaxSizeBytes <= 0 {
		return -1
	}
	available := b.MaxSizeBytes - b.CurrentSizeBytes
	if available < 0 {
		return 0
	}
	return available
}

func (b *Bucket) UsagePercent() float64 {
	if b.MaxSizeBytes <= 0 {
		return 0
	}
	return float64(b.CurrentSizeBytes) / float64(b.MaxSizeBytes) * 100
}
