package state

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofrs/flock"
	"github.com/stretchr/testify/require"
)

// TestOCIChartFileLock tests that the file locking mechanism works correctly
// to prevent race conditions when multiple processes/goroutines try to access
// the same chart cache path concurrently.
func TestOCIChartFileLock(t *testing.T) {
	t.Run("concurrent lock acquisition is serialized", func(t *testing.T) {
		// Create a temporary directory for the test
		tempDir, err := os.MkdirTemp("", "helmfile-lock-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		lockFilePath := filepath.Join(tempDir, "test-chart.lock")

		// Track the order of lock acquisition
		var lockOrder []int
		var mu sync.Mutex
		var wg sync.WaitGroup

		// Number of concurrent goroutines trying to acquire the lock
		numGoroutines := 5

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				fileLock := flock.New(lockFilePath)
				err := fileLock.Lock()
				require.NoError(t, err)

				// Record the order this goroutine acquired the lock
				mu.Lock()
				lockOrder = append(lockOrder, id)
				mu.Unlock()

				// Simulate some work while holding the lock
				time.Sleep(10 * time.Millisecond)

				err = fileLock.Unlock()
				require.NoError(t, err)
			}(i)
		}

		wg.Wait()

		// Verify all goroutines acquired the lock exactly once
		require.Len(t, lockOrder, numGoroutines, "all goroutines should have acquired the lock")
	})

	t.Run("lock prevents concurrent writes", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "helmfile-lock-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		lockFilePath := filepath.Join(tempDir, "test-chart.lock")
		dataFilePath := filepath.Join(tempDir, "data.txt")

		var wg sync.WaitGroup
		var writeCount atomic.Int32

		// Multiple goroutines try to write to the same file
		numGoroutines := 10
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				fileLock := flock.New(lockFilePath)
				err := fileLock.Lock()
				require.NoError(t, err)
				defer fileLock.Unlock()

				// Check if file exists (like double-check locking pattern)
				if _, err := os.Stat(dataFilePath); os.IsNotExist(err) {
					// Only first goroutine to acquire lock should write
					err = os.WriteFile(dataFilePath, []byte("written"), 0644)
					require.NoError(t, err)
					writeCount.Add(1)
				}
			}()
		}

		wg.Wait()

		// Only one goroutine should have written the file
		require.Equal(t, int32(1), writeCount.Load(), "only one goroutine should write when using double-check locking")

		// Verify file exists with correct content
		content, err := os.ReadFile(dataFilePath)
		require.NoError(t, err)
		require.Equal(t, "written", string(content))
	})

	t.Run("lock file is created in correct directory", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "helmfile-lock-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		// Simulate nested chart path structure
		chartPath := filepath.Join(tempDir, "registry", "charts", "myapp", "1.0.0")
		lockFilePath := chartPath + ".lock"

		// Create parent directories (like in getOCIChart)
		lockFileDir := filepath.Dir(lockFilePath)
		err = os.MkdirAll(lockFileDir, 0755)
		require.NoError(t, err)

		// Acquire and release lock
		fileLock := flock.New(lockFilePath)
		err = fileLock.Lock()
		require.NoError(t, err)

		// Verify lock file was created
		_, err = os.Stat(lockFilePath)
		require.NoError(t, err, "lock file should be created")

		err = fileLock.Unlock()
		require.NoError(t, err)
	})

	t.Run("TryLockContext respects timeout", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "helmfile-lock-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		lockFilePath := filepath.Join(tempDir, "test-chart.lock")

		// First goroutine acquires lock and holds it
		fileLock1 := flock.New(lockFilePath)
		err = fileLock1.Lock()
		require.NoError(t, err)

		// Second goroutine tries to acquire with short timeout
		done := make(chan bool)
		go func() {
			fileLock2 := flock.New(lockFilePath)
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			locked, err := fileLock2.TryLockContext(ctx, 10*time.Millisecond)
			// TryLockContext returns error when context times out
			// Either locked is false, or we get a context deadline exceeded error
			if err != nil {
				require.ErrorIs(t, err, context.DeadlineExceeded, "should timeout with deadline exceeded")
			} else {
				require.False(t, locked, "should not acquire lock within timeout")
			}
			done <- true
		}()

		select {
		case <-done:
			// Success - timeout worked
		case <-time.After(2 * time.Second):
			t.Fatal("timeout test took too long")
		}

		// Release the lock
		err = fileLock1.Unlock()
		require.NoError(t, err)
	})
}

// TestOCIChartDoubleCheckLocking verifies the double-check locking pattern
// works correctly to avoid unnecessary work when the cache is populated
// by another process while waiting for the lock.
func TestOCIChartDoubleCheckLocking(t *testing.T) {
	t.Run("second waiter uses cache populated by first", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "helmfile-double-check-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		chartPath := filepath.Join(tempDir, "myrepo", "mychart", "1.0.0")
		lockFilePath := chartPath + ".lock"

		// Create lock directory
		err = os.MkdirAll(filepath.Dir(lockFilePath), 0755)
		require.NoError(t, err)

		var pullCount atomic.Int32
		var wg sync.WaitGroup

		// Simulate two processes trying to download the same chart
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				fileLock := flock.New(lockFilePath)
				err := fileLock.Lock()
				require.NoError(t, err)
				defer fileLock.Unlock()

				// Double-check: after acquiring lock, check if directory exists
				if _, err := os.Stat(chartPath); os.IsNotExist(err) {
					// Simulate chart pull
					time.Sleep(50 * time.Millisecond) // Simulate download time
					err = os.MkdirAll(chartPath, 0755)
					require.NoError(t, err)
					err = os.WriteFile(filepath.Join(chartPath, "Chart.yaml"), []byte("name: mychart"), 0644)
					require.NoError(t, err)
					pullCount.Add(1)
				}
				// If directory exists, skip pull (use cached)
			}()
		}

		wg.Wait()

		// Only one process should have actually pulled the chart
		require.Equal(t, int32(1), pullCount.Load(), "only one process should pull the chart")

		// Chart should exist
		_, err = os.Stat(filepath.Join(chartPath, "Chart.yaml"))
		require.NoError(t, err, "chart should exist in cache")
	})
}
