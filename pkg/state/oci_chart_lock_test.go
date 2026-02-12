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
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/remote"
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

// TestOCIChartSharedExclusiveLocks tests the reader-writer lock pattern
// where shared locks allow concurrent reads and exclusive locks are used for writes.
func TestOCIChartSharedExclusiveLocks(t *testing.T) {
	t.Run("multiple shared locks can be acquired simultaneously", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "helmfile-shared-lock-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		lockFilePath := filepath.Join(tempDir, "test-chart.lock")

		// Number of concurrent readers
		numReaders := 5
		var wg sync.WaitGroup
		var activeReaders atomic.Int32
		var maxConcurrentReaders atomic.Int32

		for i := 0; i < numReaders; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				fileLock := flock.New(lockFilePath)
				// Acquire shared (read) lock
				err := fileLock.RLock()
				require.NoError(t, err)

				// Track how many readers are active simultaneously
				current := activeReaders.Add(1)

				// Update max concurrent readers
				for {
					max := maxConcurrentReaders.Load()
					if current <= max {
						break
					}
					if maxConcurrentReaders.CompareAndSwap(max, current) {
						break
					}
				}

				// Simulate reading while holding shared lock
				time.Sleep(50 * time.Millisecond)

				activeReaders.Add(-1)
				err = fileLock.Unlock()
				require.NoError(t, err)
			}()
		}

		wg.Wait()

		// Multiple readers should have been active at the same time
		require.Greater(t, maxConcurrentReaders.Load(), int32(1),
			"multiple shared locks should be held concurrently")
	})

	t.Run("exclusive lock blocks shared locks", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "helmfile-excl-block-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		lockFilePath := filepath.Join(tempDir, "test-chart.lock")

		// First, acquire exclusive lock
		writerLock := flock.New(lockFilePath)
		err = writerLock.Lock()
		require.NoError(t, err)

		// Try to acquire shared lock with timeout - should fail
		readerResult := make(chan bool)
		go func() {
			readerLock := flock.New(lockFilePath)
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			locked, err := readerLock.TryRLockContext(ctx, 10*time.Millisecond)
			if err != nil {
				readerResult <- false
				return
			}
			if locked {
				readerLock.Unlock()
			}
			readerResult <- locked
		}()

		select {
		case acquired := <-readerResult:
			require.False(t, acquired, "shared lock should not be acquired while exclusive lock is held")
		case <-time.After(2 * time.Second):
			t.Fatal("test timed out")
		}

		// Release exclusive lock
		err = writerLock.Unlock()
		require.NoError(t, err)
	})

	t.Run("shared locks block exclusive lock", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "helmfile-shared-block-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		lockFilePath := filepath.Join(tempDir, "test-chart.lock")

		// First, acquire shared lock
		readerLock := flock.New(lockFilePath)
		err = readerLock.RLock()
		require.NoError(t, err)

		// Try to acquire exclusive lock with timeout - should fail
		writerResult := make(chan bool)
		go func() {
			writerLock := flock.New(lockFilePath)
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			locked, err := writerLock.TryLockContext(ctx, 10*time.Millisecond)
			if err != nil {
				writerResult <- false
				return
			}
			if locked {
				writerLock.Unlock()
			}
			writerResult <- locked
		}()

		select {
		case acquired := <-writerResult:
			require.False(t, acquired, "exclusive lock should not be acquired while shared lock is held")
		case <-time.After(2 * time.Second):
			t.Fatal("test timed out")
		}

		// Release shared lock
		err = readerLock.Unlock()
		require.NoError(t, err)
	})

	t.Run("exclusive lock acquired after shared lock released", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "helmfile-lock-release-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		lockFilePath := filepath.Join(tempDir, "test-chart.lock")

		// Acquire shared lock
		readerLock := flock.New(lockFilePath)
		err = readerLock.RLock()
		require.NoError(t, err)

		// Start writer waiting for lock
		writerDone := make(chan bool)
		go func() {
			writerLock := flock.New(lockFilePath)
			// Use longer timeout
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			locked, err := writerLock.TryLockContext(ctx, 10*time.Millisecond)
			if err == nil && locked {
				writerLock.Unlock()
				writerDone <- true
				return
			}
			writerDone <- false
		}()

		// Release shared lock after a short delay
		time.Sleep(50 * time.Millisecond)
		err = readerLock.Unlock()
		require.NoError(t, err)

		// Writer should now succeed
		select {
		case success := <-writerDone:
			require.True(t, success, "writer should acquire lock after reader releases")
		case <-time.After(3 * time.Second):
			t.Fatal("test timed out waiting for writer")
		}
	})
}

// TestChartLockResultRelease tests the Release method of chartLockResult
// handles both shared and exclusive locks correctly.
func TestChartLockResultRelease(t *testing.T) {
	t.Run("release exclusive lock", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "helmfile-release-excl-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		lockFilePath := filepath.Join(tempDir, "test.lock")
		fileLock := flock.New(lockFilePath)
		err = fileLock.Lock()
		require.NoError(t, err)

		rwMutex := &sync.RWMutex{}
		rwMutex.Lock()

		result := &chartLockResult{
			fileLock:       fileLock,
			inProcessMutex: rwMutex,
			isExclusive:    true,
			chartPath:      lockFilePath,
		}

		// Release should not panic
		result.Release(nil)

		// Verify lock is released - should be able to acquire again
		fileLock2 := flock.New(lockFilePath)
		locked, err := fileLock2.TryLock()
		require.NoError(t, err)
		require.True(t, locked, "should be able to acquire lock after release")
		fileLock2.Unlock()
	})

	t.Run("release shared lock", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "helmfile-release-shared-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		lockFilePath := filepath.Join(tempDir, "test.lock")
		fileLock := flock.New(lockFilePath)
		err = fileLock.RLock()
		require.NoError(t, err)

		rwMutex := &sync.RWMutex{}
		rwMutex.RLock()

		result := &chartLockResult{
			fileLock:       fileLock,
			inProcessMutex: rwMutex,
			isExclusive:    false,
			chartPath:      lockFilePath,
		}

		// Release should not panic
		result.Release(nil)

		// Verify lock is released - should be able to acquire exclusive lock
		fileLock2 := flock.New(lockFilePath)
		locked, err := fileLock2.TryLock()
		require.NoError(t, err)
		require.True(t, locked, "should be able to acquire exclusive lock after shared release")
		fileLock2.Unlock()
	})

	t.Run("release nil result is safe", func(t *testing.T) {
		var result *chartLockResult
		// Should not panic
		result.Release(nil)
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

// TestIsSharedCachePath tests the isSharedCachePath function to ensure it correctly
// identifies paths within the shared cache directory.
func TestIsSharedCachePath(t *testing.T) {
	t.Run("path in shared cache is detected", func(t *testing.T) {
		// Create a HelmState with test logger
		logger := createTestLogger(t)
		st := &HelmState{
			logger: logger,
			fs:     filesystem.DefaultFileSystem(),
		}

		// Get the shared cache directory
		sharedCacheDir := remote.CacheDir()

		// Test path inside shared cache
		chartPath := filepath.Join(sharedCacheDir, "envoyproxy", "gateway-helm", "1.6.2", "gateway-helm")
		require.True(t, st.isSharedCachePath(chartPath), "path in shared cache should return true")
	})

	t.Run("path outside shared cache is not detected", func(t *testing.T) {
		logger := createTestLogger(t)
		st := &HelmState{
			logger: logger,
			fs:     filesystem.DefaultFileSystem(),
		}

		// Test path outside shared cache (temp directory)
		tempDir, err := os.MkdirTemp("", "helmfile-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		chartPath := filepath.Join(tempDir, "mychart")
		require.False(t, st.isSharedCachePath(chartPath), "path outside shared cache should return false")
	})

	t.Run("relative path handling", func(t *testing.T) {
		logger := createTestLogger(t)
		st := &HelmState{
			logger: logger,
			fs:     filesystem.DefaultFileSystem(),
		}

		// Test relative path
		require.False(t, st.isSharedCachePath("./relative/path"), "relative path should return false")
	})

	t.Run("shared cache path as prefix only", func(t *testing.T) {
		logger := createTestLogger(t)
		st := &HelmState{
			logger: logger,
			fs:     filesystem.DefaultFileSystem(),
		}

		// Test path that has shared cache dir as a prefix but not as the actual parent
		// e.g., /home/user/.cache/helmfile-other/path should not match
		sharedCacheDir := remote.CacheDir()
		nonSubpath := sharedCacheDir + "-other/chart"
		require.False(t, st.isSharedCachePath(nonSubpath), "path with shared cache as prefix but not subdirectory should return false")
	})
}

func createTestLogger(t *testing.T) *zap.SugaredLogger {
	t.Helper()
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	return logger.Sugar()
}
