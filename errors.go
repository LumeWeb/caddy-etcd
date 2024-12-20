package etcd

import (
	"fmt"
	"io/fs"
	"time"
)

// Error types for specific failure cases
var (
	ErrLockTimeout   = fmt.Errorf("lock timeout exceeded")
	ErrNoConnection  = fmt.Errorf("etcd cluster is not available")
	ErrClusterDown   = fmt.Errorf("etcd cluster is not available")
	ErrInvalidConfig = fmt.Errorf("invalid configuration")
)

// NotExist is returned when a key lookup fails when calling Load or Metadata
type NotExist struct {
	Key string
}

func (e NotExist) Error() string {
	return fmt.Sprintf("key %s does not exist", e.Key)
}

func (e NotExist) Is(target error) bool {
	return target == fs.ErrNotExist
}

// IsNotExistError checks to see if error is of type NotExist
// IsNotExistError checks if the given error indicates a key does not exist.
// Returns true if the error is of type NotExist, false otherwise.
func IsNotExistError(e error) bool {
	switch e.(type) {
	case NotExist:
		return true
	default:
		return false
	}
}

// FailedChecksum error is returned when the data returned by Load does not match the
// SHA1 checksum stored in its metadata node
type FailedChecksum struct {
	Key string
}

func (e FailedChecksum) Error() string {
	return fmt.Sprintf("data corruption detected: checksum mismatch for key %s", e.Key)
}

// IsFailedChecksumError checks to see if error is of type FailedChecksum
// IsFailedChecksumError checks if the given error indicates a checksum validation failure.
// Returns true if the error is of type FailedChecksum, false otherwise.
func IsFailedChecksumError(e error) bool {
	switch e.(type) {
	case FailedChecksum:
		return true
	default:
		return false
	}
}

// LockError represents a locking-related error
type LockError struct {
	Key     string
	Timeout time.Duration
	Err     error
}

func (e LockError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("failed to acquire lock for key %s (timeout %v) %v", e.Key, e.Timeout, e.Err)
	}
	return fmt.Sprintf("failed to acquire lock for key %s (timeout %v)", e.Key, e.Timeout)
}

func (e LockError) Unwrap() error {
	return e.Err
}

// IsLockError checks if the error is a LockError
// IsLockError checks if the given error is related to lock acquisition failure.
// Returns true if the error is of type LockError, false otherwise.
func IsLockError(e error) bool {
	_, ok := e.(LockError)
	return ok
}

// ConnectionError represents connection-related errors
type ConnectionError struct {
	Endpoints []string
	Err       error
}

func (e ConnectionError) Error() string {
	return fmt.Sprintf("failed to connect to etcd cluster at %v: %v", e.Endpoints, e.Err)
}

func (e ConnectionError) Unwrap() error {
	return e.Err
}

// IsConnectionError checks if the error is a ConnectionError
// IsConnectionError checks if the given error is related to etcd connection issues.
// Returns true if the error is of type ConnectionError, false otherwise.
func IsConnectionError(e error) bool {
	_, ok := e.(ConnectionError)
	return ok
}
