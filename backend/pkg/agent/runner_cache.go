package agent

import (
	"container/list"
	"sync"

	"trpc.group/trpc-go/trpc-agent-go/runner"
)

// maxCachedRunners bounds the session-scoped runner cache. Tools capture their
// session at construction, so each session needs its own runner; without a bound
// every session ever opened would keep its model client and tools alive.
const maxCachedRunners = 32

type runnerCacheEntry struct {
	key    string
	runner runner.Runner
}

// runnerCache is a concurrency-safe LRU of constructed runners.
type runnerCache struct {
	mu       sync.Mutex
	capacity int
	order    *list.List
	entries  map[string]*list.Element
}

// newRunnerCache creates an empty cache holding at most capacity runners.
func newRunnerCache(capacity int) *runnerCache {
	return &runnerCache{
		capacity: capacity,
		order:    list.New(),
		entries:  make(map[string]*list.Element),
	}
}

// get returns a cached runner and marks it most recently used.
func (c *runnerCache) get(key string) (runner.Runner, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	elem, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	c.order.MoveToFront(elem)
	return elem.Value.(*runnerCacheEntry).runner, true
}

// add stores a runner unless another caller won the race, returning the cached one.
func (c *runnerCache) add(key string, created runner.Runner) runner.Runner {
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.entries[key]; ok {
		c.order.MoveToFront(elem)
		return elem.Value.(*runnerCacheEntry).runner
	}
	c.entries[key] = c.order.PushFront(&runnerCacheEntry{key: key, runner: created})
	for c.order.Len() > c.capacity {
		oldest := c.order.Back()
		c.order.Remove(oldest)
		delete(c.entries, oldest.Value.(*runnerCacheEntry).key)
	}
	return created
}

// clear drops every cached runner, e.g. after tool dependencies change.
func (c *runnerCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.order.Init()
	clear(c.entries)
}

// len reports how many runners are cached.
func (c *runnerCache) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}
