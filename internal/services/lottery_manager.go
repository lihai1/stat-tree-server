package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	lotterytree "github.com/lihai1/stat-tree-server/internal/lottery-tree"
	"github.com/lihai1/stat-tree-server/internal/repository"
)

// defaultArchiveFrom is the start of the current Israeli Lotto format, whose
// regular numbers range 1-37. Older archive rows contain numbers up to 49 and
// must not be mixed into the default modern archive. The UI uses the same
// date (2004-02-12) as its default window start.
const defaultArchiveFrom = "2004-02-12"

// maxCacheEntries is the upper bound on how many date-window archives the
// manager keeps resident. Eight is enough for the few windows the UI and
// ad-hoc requests use simultaneously without holding the whole history in
// memory for every possible range.
const maxCacheEntries = 8

// LotteryManager owns a small LRU cache of LotteryArchive objects keyed by
// normalized (from, to) date window. The archive for a window — draws,
// frequency trees, and the winning-combination set — is built lazily on the
// first request that needs it and then reused by every later RPC for the
// same window. Concurrent misses for the same key are coordinated via
// per-key single-flight loads so the archive is built once, not once per
// racing caller.
//
// Scraper and prize-backfill updates invalidate only the cache windows whose
// date range overlaps the affected draws, leaving unrelated windows valid.
type LotteryManager struct {
	repo *repository.LotteryResultRepository

	mu    sync.Mutex
	cache map[string]*cacheEntry
	head  *cacheEntry // most recently used
	tail  *cacheEntry // least recently used
}

type cacheEntry struct {
	key    string
	from   time.Time
	to     time.Time
	arch   *lotterytree.LotteryArchive
	prev   *cacheEntry
	next   *cacheEntry

	// load coordinates concurrent construction for the same key. A non-nil
	// done channel means a load is in flight; the waiters read the result
	// from arch once done is closed.
	done chan struct{}
	err  error
}

// NewLotteryManager returns a manager wired to the given repository. A nil
// repository is tolerated so unit tests without a database can construct a
// manager; every archive then resolves to an empty archive.
func NewLotteryManager(repo *repository.LotteryResultRepository) *LotteryManager {
	return &LotteryManager{
		repo:  repo,
		cache: make(map[string]*cacheEntry),
	}
}

// Archive returns the cached archive for the (from, to) window, building it
// lazily on the first request. When from is zero it is replaced with the
// default modern-format start (2004-02-12); when to is zero it is left open
// (no upper bound). The returned archive is read-only and safe for
// concurrent use.
func (m *LotteryManager) Archive(ctx context.Context, from, to time.Time) (*lotterytree.LotteryArchive, error) {
	from, to = normalizeWindow(from, to)
	key := windowKey(from, to)

	m.mu.Lock()
	if e, ok := m.cache[key]; ok {
		m.mu.Unlock()
		// Wait for an in-flight load, or reuse the ready archive.
		if e.done != nil {
			select {
			case <-e.done:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			if e.err != nil {
				return nil, e.err
			}
		}
		m.mu.Lock()
		m.moveToFront(e)
		m.mu.Unlock()
		return e.arch, nil
	}

	// Miss: start a single-flight load.
	e := &cacheEntry{key: key, from: from, to: to, done: make(chan struct{})}
	m.cache[key] = e
	m.addToFront(e)
	if len(m.cache) > maxCacheEntries {
		victim := m.tail
		if victim != nil && victim.done == nil {
			m.remove(victim)
			delete(m.cache, victim.key)
		}
	}
	m.mu.Unlock()

	// Build outside the cache lock so other keys stay usable.
	arch, err := m.build(ctx, from, to)

	m.mu.Lock()
	e.arch = arch
	e.err = err
	close(e.done)
	e.done = nil
	if err != nil {
		// Drop failed entries so the next request retries.
		m.remove(e)
		delete(m.cache, key)
	}
	m.mu.Unlock()
	return arch, err
}

// build loads the draws for the window from the repository and constructs the
// immutable archive. With a nil repository it returns an empty archive so
// tests without a database get well-defined zero-value behavior.
func (m *LotteryManager) build(ctx context.Context, from, to time.Time) (*lotterytree.LotteryArchive, error) {
	if m.repo == nil {
		return lotterytree.NewLotteryArchive(lotterytree.Lottery, from, to, nil), nil
	}
	draws, err := m.repo.GetByDateRange(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("lottery manager: load draws %s..%s: %w",
			from.Format("2006-01-02"), to.Format("2006-01-02"), err)
	}
	return lotterytree.NewLotteryArchive(lotterytree.Lottery, from, to, draws), nil
}

// InvalidateRange drops every cached window whose [from, to] overlaps the
// given [affectedFrom, affectedTo] range. Windows entirely outside the
// affected range stay resident. An open affected bound (zero) is treated as
// "no limit" on that side, so InvalidateRange(zero, zero) clears everything.
func (m *LotteryManager) InvalidateRange(affectedFrom, affectedTo time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, e := range m.cache {
		if rangesOverlap(e.from, e.to, affectedFrom, affectedTo) {
			m.remove(e)
			delete(m.cache, key)
		}
	}
}

// InvalidateAll clears the entire cache. Used by the force-seed path which
// rewrites the whole table.
func (m *LotteryManager) InvalidateAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cache = make(map[string]*cacheEntry)
	m.head = nil
	m.tail = nil
}

// DefaultFrom returns the default archive start date (2004-02-12).
func (m *LotteryManager) DefaultFrom() time.Time {
	t, err := time.Parse("2006-01-02", defaultArchiveFrom)
	if err != nil {
		return time.Time{}
	}
	return t
}

// normalizeWindow applies the default start date when from is zero and leaves
// an open upper bound when to is zero.
func normalizeWindow(from, to time.Time) (time.Time, time.Time) {
	if from.IsZero() {
		if t, err := time.Parse("2006-01-02", defaultArchiveFrom); err == nil {
			from = t
		}
	}
	return from, to
}

// windowKey is the stable cache key for a normalized window.
func windowKey(from, to time.Time) string {
	fromStr := "open"
	if !from.IsZero() {
		fromStr = from.Format("2006-01-02")
	}
	toStr := "open"
	if !to.IsZero() {
		toStr = to.Format("2006-01-02")
	}
	return fromStr + "|" + toStr
}

// rangesOverlap reports whether [aFrom, aTo] overlaps [bFrom, bTo]. A zero
// bound means "no limit" on that side.
func rangesOverlap(aFrom, aTo, bFrom, bTo time.Time) bool {
	// Treat each range as [lo, hi] with zero meaning unbounded.
	aLo := aFrom
	aHi := aTo
	bLo := bFrom
	bHi := bTo
	if !aHi.IsZero() && !bLo.IsZero() && aHi.Before(bLo) {
		return false
	}
	if !bHi.IsZero() && !aLo.IsZero() && bHi.Before(aLo) {
		return false
	}
	return true
}

// addToFront inserts e at the head of the LRU list. The caller must hold mu.
func (m *LotteryManager) addToFront(e *cacheEntry) {
	e.prev = nil
	e.next = m.head
	if m.head != nil {
		m.head.prev = e
	}
	m.head = e
	if m.tail == nil {
		m.tail = e
	}
}

// remove unlinks e from the LRU list. The caller must hold mu.
func (m *LotteryManager) remove(e *cacheEntry) {
	if e.prev != nil {
		e.prev.next = e.next
	} else {
		m.head = e.next
	}
	if e.next != nil {
		e.next.prev = e.prev
	} else {
		m.tail = e.prev
	}
	e.prev = nil
	e.next = nil
}

// moveToFront moves e to the head of the LRU list. The caller must hold mu.
func (m *LotteryManager) moveToFront(e *cacheEntry) {
	if m.head == e {
		return
	}
	m.remove(e)
	m.addToFront(e)
}

// CacheSize returns the number of resident archive entries (for tests).
func (m *LotteryManager) CacheSize() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.cache)
}

func init() {
	// Silence unused-warning if log is not otherwise referenced in a build
	// that strips the per-key load path; keeping the import stable.
	_ = log.Printf
}
