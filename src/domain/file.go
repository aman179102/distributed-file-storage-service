package domain

import (
	"time"
)

type FileStatus string

const (
	FileStatusActive     FileStatus = "active"
	FileStatusExpired    FileStatus = "expired"
	FileStatusPending    FileStatus = "pending"
	FileStatusDeleted    FileStatus = "deleted"
)

type FileInfo struct {
	ID            string            `json:"id"`
	BucketID      string            `json:"bucket_id"`
	Key           string            `json:"key"`
	Size          int64             `json:"size"`
	ETag          string            `json:"etag"`
	ContentType   string            `json:"content_type"`
	Metadata      map[string]string `json:"metadata"`
	ChecksumSHA256 string           `json:"checksum_sha256"`
	StorageClass  string            `json:"storage_class"`
	VersionID     string            `json:"version_id"`
	Status        FileStatus        `json:"status"`
	TTL           *time.Duration    `json:"ttl,omitempty"`
	ExpiresAt     *time.Time        `json:"expires_at,omitempty"`
	Owner         string            `json:"owner"`
	ChunkCount    int               `json:"chunk_count"`
	StorageSize   int64             `json:"storage_size"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	DeletedAt     *time.Time        `json:"deleted_at,omitempty"`
}

type FileChunk struct {
	ID           string `json:"id"`
	FileID       string `json:"file_id"`
	ChunkIndex   int    `json:"chunk_index"`
	Size         int64  `json:"size"`
	Checksum     string `json:"checksum"`
	StorageNode  string `json:"storage_node"`
	StoragePath  string `json:"storage_path"`
	IsParity     bool   `json:"is_parity"`
}

type FilePart struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
	Size       int64  `json:"size"`
	Checksum   string `json:"checksum"`
	UploadID   string `json:"upload_id"`
}

type MultipartUpload struct {
	ID              string            `json:"id"`
	BucketID        string            `json:"bucket_id"`
	Key             string            `json:"key"`
	UploadedParts   []FilePart        `json:"uploaded_parts"`
	Initiator       string            `json:"initiator"`
	ContentType     string            `json:"content_type"`
	Metadata        map[string]string `json:"metadata"`
	StorageClass    string            `json:"storage_class"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

func (f *FileInfo) IsExpired() bool {
	if f.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*f.ExpiresAt)
}

func (f *FileInfo) HasTTL() bool {
	return f.TTL != nil && *f.TTL > 0
}

func (f *FileInfo) ComputeStorageEfficiency() float64 {
	if f.Size == 0 {
		return 0
	}
	return float64(f.Size) / float64(f.StorageSize)
}
