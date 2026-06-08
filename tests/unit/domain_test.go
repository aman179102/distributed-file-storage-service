package unit

import (
	"testing"
	"time"

	"github.com/distributed-file-storage/service/src/domain"
)

func TestFileInfo_IsExpired(t *testing.T) {
	future := time.Now().Add(1 * time.Hour)
	past := time.Now().Add(-1 * time.Hour)

	tests := []struct {
		name     string
		file     *domain.FileInfo
		expected bool
	}{
		{
			name: "not expired when expires_at is nil",
			file: &domain.FileInfo{ExpiresAt: nil},
			expected: false,
		},
		{
			name: "not expired when expires_at is in the future",
			file: &domain.FileInfo{ExpiresAt: &future},
			expected: false,
		},
		{
			name: "expired when expires_at is in the past",
			file: &domain.FileInfo{ExpiresAt: &past},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.file.IsExpired(); got != tt.expected {
				t.Errorf("IsExpired() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFileInfo_HasTTL(t *testing.T) {
	duration := 5 * time.Minute
	zeroDuration := time.Duration(0)

	tests := []struct {
		name     string
		file     *domain.FileInfo
		expected bool
	}{
		{
			name:     "no TTL when nil",
			file:     &domain.FileInfo{TTL: nil},
			expected: false,
		},
		{
			name:     "has TTL when set to positive duration",
			file:     &domain.FileInfo{TTL: &duration},
			expected: true,
		},
		{
			name:     "no TTL when zero duration",
			file:     &domain.FileInfo{TTL: &zeroDuration},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.file.HasTTL(); got != tt.expected {
				t.Errorf("HasTTL() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFileInfo_ComputeStorageEfficiency(t *testing.T) {
	tests := []struct {
		name     string
		file     *domain.FileInfo
		expected float64
	}{
		{
			name:     "zero size returns 0",
			file:     &domain.FileInfo{Size: 0, StorageSize: 100},
			expected: 0,
		},
		{
			name:     "perfect efficiency",
			file:     &domain.FileInfo{Size: 100, StorageSize: 100},
			expected: 1.0,
		},
		{
			name:     "with overhead",
			file:     &domain.FileInfo{Size: 100, StorageSize: 150},
			expected: 100.0 / 150.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.file.ComputeStorageEfficiency(); got != tt.expected {
				t.Errorf("ComputeStorageEfficiency() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestBucket_IsVersioned(t *testing.T) {
	tests := []struct {
		name     string
		bucket   *domain.Bucket
		expected bool
	}{
		{
			name:     "versioning enabled",
			bucket:   &domain.Bucket{Versioning: domain.VersioningEnabled},
			expected: true,
		},
		{
			name:     "versioning disabled",
			bucket:   &domain.Bucket{Versioning: domain.VersioningDisabled},
			expected: false,
		},
		{
			name:     "versioning suspended",
			bucket:   &domain.Bucket{Versioning: domain.VersioningSuspended},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.bucket.IsVersioned(); got != tt.expected {
				t.Errorf("IsVersioned() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestBucket_AvailableBytes(t *testing.T) {
	tests := []struct {
		name     string
		bucket   *domain.Bucket
		expected int64
	}{
		{
			name:     "no limit when max is 0",
			bucket:   &domain.Bucket{MaxSizeBytes: 0, CurrentSizeBytes: 100},
			expected: -1,
		},
		{
			name:     "full bucket",
			bucket:   &domain.Bucket{MaxSizeBytes: 100, CurrentSizeBytes: 100},
			expected: 0,
		},
		{
			name:     "partially used",
			bucket:   &domain.Bucket{MaxSizeBytes: 1000, CurrentSizeBytes: 300},
			expected: 700,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.bucket.AvailableBytes(); got != tt.expected {
				t.Errorf("AvailableBytes() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestBucket_UsagePercent(t *testing.T) {
	tests := []struct {
		name     string
		bucket   *domain.Bucket
		expected float64
	}{
		{
			name:     "no max returns 0",
			bucket:   &domain.Bucket{MaxSizeBytes: 0, CurrentSizeBytes: 100},
			expected: 0,
		},
		{
			name:     "half full",
			bucket:   &domain.Bucket{MaxSizeBytes: 1000, CurrentSizeBytes: 500},
			expected: 50.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.bucket.UsagePercent(); got != tt.expected {
				t.Errorf("UsagePercent() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestUser_HasRole(t *testing.T) {
	user := &domain.User{Role: "admin"}
	if !user.HasRole("admin") {
		t.Error("admin user should have admin role")
	}
	if user.HasRole("user") {
		t.Error("admin user should not have user role")
	}
}

func TestUser_Can(t *testing.T) {
	tests := []struct {
		name         string
		user         *domain.User
		requiredRole string
		expected     bool
	}{
		{name: "admin can do everything", user: &domain.User{Role: "admin"}, requiredRole: "readonly", expected: true},
		{name: "readonly cannot do admin", user: &domain.User{Role: "readonly"}, requiredRole: "admin", expected: false},
		{name: "unknown role returns false", user: &domain.User{Role: "unknown"}, requiredRole: "user", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.user.Can(tt.requiredRole); got != tt.expected {
				t.Errorf("Can() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDomainError(t *testing.T) {
	err := domain.NewNotFound("test")
	if err.Code != domain.ErrNotFound {
		t.Errorf("expected NOT_FOUND, got %s", err.Code)
	}
	if err.Error() != "[NOT_FOUND] test not found" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestPolicyEvaluation(t *testing.T) {
	pe := &domain.PolicyEvaluation{Allowed: true, Reason: "admin access"}
	if !pe.Allowed {
		t.Error("expected allowed")
	}
	if pe.Reason != "admin access" {
		t.Errorf("unexpected reason: %s", pe.Reason)
	}
}
