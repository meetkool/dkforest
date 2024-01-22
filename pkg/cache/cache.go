package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"dkforest/pkg/clockwork"
)

const (
	NoExpiration      time.Duration = -1
	DefaultExpiration time.Duration = 0
)

var ErrItemAlreadyExists error = errors.New("item already exists")
var ErrItemNotFound error = errors.New("item does not exists")

// Item wrap the user provided value and add data to it
type Item[V any] struct {
	value      V
	expiration int64
}

type Cache[K comparable, V any] struct {
	ctx               context.Context    // Context is used to stop the auto-cleanup thread
	cancel            context.CancelFunc // Cancel the context and stop the auto-cleanup thread
	mtx               sync.RWMutex       // This mutex should only be used in exported methods
	defaultExpiration time.Duration      // Default expiration for items in cache
	items             map[K]Item[V]      // Hashmap that contains all items in the cache
	clock             clockwork.Clock    // Clock object for time related features
}

// Creates a cache with K as string
func New[V any](defaultExpiration, cleanupInterval time.Duration) *Cache[string, V] {
	return newCache[string, V](context.Background(), defaultExpiration, cleanupInterval)
}

// Creates a cache with a context provided by the user
func NewWithContext[V any](ctx context.Context, defaultExpiration, cleanupInterval time.Duration) *Cache[string, V] {
	return newCache[string, V](ctx, defaultExpiration, cleanupInterval)
}

// Create a cache with a custom comparable K provided by the user
func NewWithKey[K comparable, V any](defaultExpiration, cleanupInterval time.Duration) *Cache[K, V] {
	return newCache[K, V](context.Background(), defaultExpiration, cleanupInterval)
}

// Destroy the cache object, cleanup all resources
func (c *Cache[K, V]) Destroy() {
	c.cancel()
	c = nil
}

// Has returns either or not the key is present in the cache
func (c *Cache[K, V]) Has(k K) bool {
	c.mtx.RLock()
	found := c.has(k)
	c.mtx.RUnlock()
	return found
}

// Get an value associated to the given key
func (c *Cache[K, V]) Get(k K) (V, bool) {
	c.mtx.RLock()
	value, found := c.get(k)
	c.mtx.RUnlock()
	return value, found
}

// GetWithExpiration a value and it's expiration
func (c *Cache[K, V]) GetWithExpiration(k K) (V, time.Time, bool) {
	c.mtx.RLock()
	value, expiration, found := c.getWithExpiration(k)
	c.mtx.RUnlock()
	return value, expiration, found
}

// Set a key/value pair in the cache
func (c *Cache[K, V]) Set(k K, v V, d time.Duration) {
	c.mtx.Lock()
	c.set(k, v, d)
	c.mtx.Unlock()
}

// SetD same as Set, but use the DefaultExpiration automatically
func (c *Cache[K, V]) SetD(k K, v V) {
	c.mtx.Lock()
	c.set(k, v, c.defaultExpiration)
	c.mtx.Unlock()
}

// Add an item to the cache only if an item doesn't already exist for the given
// key, or if the existing item has expired. Returns an error otherwise.
func (c *Cache[K, V]) Add(k K, v V, d time.Duration) error {
	c.mtx.Lock()
	err := c.add(k, v, d)
	c.mtx.Unlock()
	return err
}

// Replace set a new value for the cache key only if it already exists, and the existing
// item hasn't expired. Returns an error otherwise.
func (c *Cache[K, V]) Replace(k K, v V, d time.Duration) error {
	c.mtx.Lock()
	err := c.replace(k, v, d)
	c.mtx.Unlock()
	return err
}

func (c *Cache[K, V]) Update(k K, v V) error {
	c.mtx.Lock()
	err := c.update(k, v)
	c.mtx.Unlock()
	return err
}

// Delete an item from the cache
func (c *Cache[K, V]) Delete(k K) {
	c.mtx.Lock()
	c.delete(k)
	c.mtx.Unlock()
}

// DeleteExpired deletes all expired items from the cache
func (c *Cache[K, V]) DeleteExpired() {
	c.mtx.Lock()
	c.deleteExpired()
	c.mtx.Unlock()
}

// DeleteAll deletes all items from the cache
func (c *Cache[K, V]) DeleteAll() {
	c.mtx.Lock()
	c.deleteAll()
	c.mtx.Unlock()
}

// Len returns the number of items in the cache. This may include items that have
// expired, but have not yet been cleaned up.
func (c *Cache[K, V]) Len() int {
	c.mtx.Lock()
	n := len(c.items)
	c.mtx.Unlock()
	return n
}

// Items copies all unexpired items in the cache into a new map and returns it.
func (c *Cache[K, V]) Items() map[K]Item[V] {
	c.mtx.RLock()
	items := c.getItems()
	c.mtx.RUnlock()
	return items
}

// SetClock set the clock object
func (c *Cache[K, V]) SetClock(clock clockwork.Clock) {
	c.clock = clock
}

func newCache[K comparable, V any](ctx context.Context, defaultExpiration, cleanupInterval time.Duration) *Cache[K, V] {
	items := make(map[K]Item[V])
	c := new(Cache[K, V])
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.clock = clockwork.NewRealClock()
	c.defaultExpiration = defaultExpiration
	c.items = items
	if cleanupInterval > 0 {
		go c.autoCleanup(cleanupInterval)
	}
	return c
}

func (c *Cache[K, V]) autoCleanup(cleanupInterval time.Duration) {
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-time.After(cleanupInterval):
		}
		// Important to call the exported method to lock the mutex
		c.DeleteExpired()
	}
}

func (c *Cache[K, V]) has(k K) bool {
	_, found := c.get(k)
	return found
}

func (c *Cache[K, V]) getWithExpiration(k K) (V, time.Time, bool) {
	var zero V
	now := c.clock.Now().UnixNano()
	item, found := c.items[k]
	if !found {
		return zero, time.Time{}, false
	}
	e := time.Time{}
	if item.expiration > 0 {
		if item.expiration < now {
			return zero, time.Time{}, false
		}
		e = time.Unix(0, item.expiration)
	}
	return item.value, e, found
}

func (c *Cache[K, V]) get(k K) (V, bool) {
	value, _, found := c.getWithExpiration(k)
	return value, found
}

func (c *Cache[K, V]) set(k K, v V, d time.Duration) {
	e := int64(NoExpiration)
	if d == DefaultExpiration {
		d = c.defaultExpiration
	}
	e = c.clock.Now().Add(d).UnixNano()
	c.items[k] = Item[V]{value: v, expiration: e}
}

func (c *Cache[K, V]) add(k K, v V, d time.Duration) error {
	if _, found := c.get(k); found {
		return ErrItemAlreadyExists
	}
	c.set(k, v, d)
	return nil
}

func (c *Cache[K, V]) replace(k K, v V, d time.Duration) error {
	if _, found := c.get(k); !found {
		return ErrItemNotFound
	}
	c.set(k, v, d)
	return nil
}

func (c *Cache[K, V]) update(k K, v V) error {
	vv, found := c.items[k]
	if !found || vv.IsExpired() {
		return fmt.Errorf("item %v not found", k)
	}
	vv.value = v
	c.items[k] = vv
	return nil
}

func (c *Cache[K, V]) deleteAll() {
	c.items = make(map[K]Item[V])
}

func (c *Cache[K, V]) delete(k K) {
	delete(c.items, k)
}

func (c *Cache[K, V]) deleteExpired() {
	now := c.clock.Now().UnixNano()
	for k := range c.items {
		item := c.items[k]
		if item.isExpired(now) {
			c.delete(k)
		}
	}
}

func (c *Cache[K, V]) getItems() map[K]Item[V] {
	now := c.clock.Now().UnixNano()
	m := make(map[K]Item[V], len(c.items))
	for k, v := range c.items {
		if !v.isExpired(now) {
			m[k] = v
		}
	}
	return m
}

// Value returns the value contained by the item
func (i Item[V]) Value() V {
	return i.value
}

// Expiration returns the expiration time
func (i Item[V]) Expiration() time.Time {
	return time.Unix(0, i.expiration)
}

// IsExpired returns either or not the item is expired right now
func (i Item[V]) IsExpired() bool {
	now := time.Now().UnixNano()
	return i.isExpired(now)
}

// Given a unix (nano) timestamp, return either or not the item is expired
func (i Item[V]) isExpired(ts int64) bool {
	return i.expiration > 0 && i.expiration < ts
}
