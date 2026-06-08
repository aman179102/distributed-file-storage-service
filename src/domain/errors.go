package domain

import "fmt"

type ErrorCode string

const (
	ErrNotFound           ErrorCode = "NOT_FOUND"
	ErrAlreadyExists      ErrorCode = "ALREADY_EXISTS"
	ErrInvalidInput       ErrorCode = "INVALID_INPUT"
	ErrUnauthorized       ErrorCode = "UNAUTHORIZED"
	ErrForbidden          ErrorCode = "FORBIDDEN"
	ErrInternal           ErrorCode = "INTERNAL_ERROR"
	ErrStorageQuota       ErrorCode = "STORAGE_QUOTA_EXCEEDED"
	ErrObjectLocked       ErrorCode = "OBJECT_LOCKED"
	ErrPreconditionFailed ErrorCode = "PRECONDITION_FAILED"
	ErrRangeNotSatisfiable ErrorCode = "RANGE_NOT_SATISFIABLE"
	ErrEntityTooLarge     ErrorCode = "ENTITY_TOO_LARGE"
	ErrBadDigest          ErrorCode = "BAD_DIGEST"
	ErrIncompleteBody     ErrorCode = "INCOMPLETE_BODY"
	ErrNoSuchUpload       ErrorCode = "NO_SUCH_UPLOAD"
	ErrBucketNotEmpty     ErrorCode = "BUCKET_NOT_EMPTY"
)

type DomainError struct {
	Code    ErrorCode
	Message string
	Details map[string]interface{}
}

func (e *DomainError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func NewNotFound(resource string) *DomainError {
	return &DomainError{Code: ErrNotFound, Message: fmt.Sprintf("%s not found", resource)}
}

func NewAlreadyExists(resource string) *DomainError {
	return &DomainError{Code: ErrAlreadyExists, Message: fmt.Sprintf("%s already exists", resource)}
}

func NewInvalidInput(msg string) *DomainError {
	return &DomainError{Code: ErrInvalidInput, Message: msg}
}

func NewUnauthorized(msg string) *DomainError {
	return &DomainError{Code: ErrUnauthorized, Message: msg}
}

func NewForbidden(msg string) *DomainError {
	return &DomainError{Code: ErrForbidden, Message: msg}
}

func NewInternal(msg string) *DomainError {
	return &DomainError{Code: ErrInternal, Message: msg}
}
