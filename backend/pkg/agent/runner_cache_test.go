package agent

import (
	"fmt"
	"sync"
	"testing"

	"trpc.group/trpc-go/trpc-agent-go/runner"
)

// TestRunnerCacheEvictsLeastRecentlyUsed keeps the cache bounded while retaining recently used sessions.
func TestRunnerCacheEvictsLeastRecentlyUsed(t *testing.T) {
	cache := newRunnerCache(2)
	a, b, c := newDelayedRunner(), newDelayedRunner(), newDelayedRunner()

	cache.add("a", a)
	cache.add("b", b)
	if _, ok := cache.get("a"); !ok {
		t.Fatal("expected a to be cached")
	}
	cache.add("c", c)

	if _, ok := cache.get("b"); ok {
		t.Fatal("expected b, the least recently used entry, to be evicted")
	}
	if got, ok := cache.get("a"); !ok || got != a {
		t.Fatal("expected recently used a to survive eviction")
	}
	if got, ok := cache.get("c"); !ok || got != c {
		t.Fatal("expected newest c to be cached")
	}
	if cache.len() != 2 {
		t.Fatalf("expected 2 cached runners, got %d", cache.len())
	}
}

// TestRunnerCacheAddKeepsFirstWriter returns the runner that won a concurrent construction race.
func TestRunnerCacheAddKeepsFirstWriter(t *testing.T) {
	cache := newRunnerCache(4)
	first, second := newDelayedRunner(), newDelayedRunner()

	if got := cache.add("k", first); got != first {
		t.Fatal("expected first add to store its runner")
	}
	if got := cache.add("k", second); got != first {
		t.Fatal("expected later add to return the already cached runner")
	}
}

func TestRunnerCacheClear(t *testing.T) {
	cache := newRunnerCache(4)
	cache.add("k", newDelayedRunner())
	cache.clear()
	if _, ok := cache.get("k"); ok {
		t.Fatal("expected clear to drop cached runners")
	}
	if cache.len() != 0 {
		t.Fatalf("expected empty cache, got %d", cache.len())
	}
}

// TestRunnerCacheConcurrentAccess exercises get/add/clear together under the race detector.
func TestRunnerCacheConcurrentAccess(t *testing.T) {
	cache := newRunnerCache(8)
	runners := make([]runner.Runner, 16)
	for i := range runners {
		runners[i] = newDelayedRunner()
	}
	var wg sync.WaitGroup
	for i := range runners {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("k%d", i%10)
			cache.add(key, runners[i])
			cache.get(key)
			if i%7 == 0 {
				cache.clear()
			}
		}(i)
	}
	wg.Wait()
	if cache.len() > 8 {
		t.Fatalf("cache exceeded capacity: %d", cache.len())
	}
}
