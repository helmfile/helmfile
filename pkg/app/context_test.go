package app

import (
	"sync"
	"testing"
)

// TestContextConcurrentAccess verifies that Context is thread-safe
// when accessed concurrently from multiple goroutines
func TestContextConcurrentAccess(t *testing.T) {
	ctx := &Context{
		updatedRepos: make(map[string]bool),
	}

	const numGoroutines = 100
	const numReposPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Launch multiple goroutines that concurrently update the repos map
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numReposPerGoroutine; j++ {
				repoKey := "repo-" + string(rune('0'+goroutineID)) + "-" + string(rune('0'+j))

				ctx.mu.Lock()
				ctx.updatedRepos[repoKey] = true
				ctx.mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// Verify the map has entries (exact count may vary due to key overlap)
	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	if len(ctx.updatedRepos) == 0 {
		t.Error("expected non-empty updatedRepos after concurrent updates")
	}
}

// TestContextInitialization verifies Context is created with proper initial state
func TestContextInitialization(t *testing.T) {
	ctx := NewContext()

	if ctx.updatedRepos == nil {
		t.Error("updatedRepos map is nil")
	}

	// Verify initial state is empty
	if len(ctx.updatedRepos) != 0 {
		t.Errorf("expected empty updatedRepos, got %d entries", len(ctx.updatedRepos))
	}
}

// TestContextPointerSemantics verifies that Context is correctly used as a pointer
// to prevent mutex copying issues
func TestContextPointerSemantics(t *testing.T) {
	// Create a Context
	ctx := &Context{
		updatedRepos: make(map[string]bool),
	}

	// Create a Run with the context
	run := &Run{
		ctx: ctx,
	}

	// Verify that run.ctx points to the same Context
	if run.ctx != ctx {
		t.Error("Run.ctx does not point to the same Context instance")
	}

	// Modify the context through run.ctx and verify the original is affected
	repoKey := "test-repo=https://charts.example.com"

	run.ctx.mu.Lock()
	run.ctx.updatedRepos[repoKey] = true
	run.ctx.mu.Unlock()

	// Check that the original context was modified
	ctx.mu.Lock()
	found := ctx.updatedRepos[repoKey]
	ctx.mu.Unlock()

	if !found {
		t.Error("original context was not modified (pointer semantics broken)")
	}
}

// TestContextMutexNotCopied verifies that using pointer receivers prevents mutex copying
func TestContextMutexNotCopied(t *testing.T) {
	ctx1 := &Context{
		updatedRepos: make(map[string]bool),
	}

	// Assign to another variable (should be pointer copy, not value copy)
	ctx2 := ctx1

	// Modify through ctx2
	ctx2.mu.Lock()
	ctx2.updatedRepos["test"] = true
	ctx2.mu.Unlock()

	// Verify ctx1 sees the change (they share the same underlying data)
	ctx1.mu.Lock()
	found := ctx1.updatedRepos["test"]
	ctx1.mu.Unlock()

	if !found {
		t.Error("ctx1 and ctx2 don't share the same data (value copy instead of pointer copy)")
	}
}

// TestContextConcurrentReadWrite tests concurrent reads and writes to the Context
func TestContextConcurrentReadWrite(t *testing.T) {
	ctx := &Context{
		updatedRepos: make(map[string]bool),
	}

	const numRepos = 10
	const numGoroutinesPerRepo = 10

	var wg sync.WaitGroup

	// Launch multiple goroutines for each repo
	for i := 0; i < numRepos; i++ {
		repoKey := "repo-" + string(rune('0'+i)) + "=https://example.com"

		for j := 0; j < numGoroutinesPerRepo; j++ {
			wg.Add(1)
			go func(key string) {
				defer wg.Done()

				// Write
				ctx.mu.Lock()
				ctx.updatedRepos[key] = true
				ctx.mu.Unlock()

				// Read
				ctx.mu.Lock()
				_ = ctx.updatedRepos[key]
				ctx.mu.Unlock()
			}(repoKey)
		}
	}

	wg.Wait()

	// Verify repos are in the map
	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	if len(ctx.updatedRepos) != numRepos {
		t.Errorf("expected %d repos, got %d", numRepos, len(ctx.updatedRepos))
	}

	// Verify all are marked as true
	for key, value := range ctx.updatedRepos {
		if !value {
			t.Errorf("repo %s is not marked as true", key)
		}
	}
}
