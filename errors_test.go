package etcd

import (
	"io"
	"io/fs"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestErrors(t *testing.T) {
	t.Run("basic error types", func(t *testing.T) {
		e1 := NotExist{"/test/path"}
		e2 := FailedChecksum{"/test/path"}
		assert.True(t, IsNotExistError(e1))
		assert.True(t, IsFailedChecksumError(e2))
		assert.Contains(t, e1.Error(), "/test/path")
		assert.Contains(t, e2.Error(), "/test/path")
	})

	t.Run("lock error", func(t *testing.T) {
		timeout := 5 * time.Second
		baseErr := errors.New("lock failed")
		e := LockError{
			Key:     "/test/lock",
			Timeout: timeout,
			Err:     baseErr,
		}

		assert.True(t, IsLockError(e))
		assert.Contains(t, e.Error(), "/test/lock")
		assert.Contains(t, e.Error(), timeout.String())
		assert.Equal(t, baseErr, e.Unwrap())

		// Test without wrapped error
		e2 := LockError{
			Key:     "/test/lock",
			Timeout: timeout,
		}
		assert.NotContains(t, e2.Error(), ":")
	})

	t.Run("connection error", func(t *testing.T) {
		endpoints := []string{"http://localhost:2379"}
		baseErr := ErrClusterDown
		e := ConnectionError{
			Endpoints: endpoints,
			Err:       baseErr,
		}

		assert.True(t, IsConnectionError(e))
		assert.Contains(t, e.Error(), endpoints[0])
		assert.Equal(t, baseErr, e.Unwrap())
	})

	t.Run("standard errors", func(t *testing.T) {
		assert.Error(t, ErrLockTimeout)
		assert.Error(t, ErrNoConnection)
		assert.Error(t, ErrClusterDown)
		assert.Error(t, ErrInvalidConfig)
	})

	t.Run("error wrapping", func(t *testing.T) {
		baseErr := errors.New("original error")
		connErr := ConnectionError{
			Endpoints: []string{"http://localhost:2379"},
			Err:       baseErr,
		}

		var target ConnectionError
		assert.True(t, errors.As(connErr, &target))
		assert.Equal(t, baseErr, errors.Unwrap(connErr))
	})
}
func TestNotExistError(t *testing.T) {
	err := NotExist{Key: "test/key"}

	// Should match fs.ErrNotExist
	assert.True(t, errors.Is(err, fs.ErrNotExist))

	// Should not match other errors
	assert.False(t, errors.Is(err, io.EOF))

	// Error message should contain the key
	assert.Contains(t, err.Error(), "test/key")
	assert.Contains(t, err.Error(), "does not exist")
}
