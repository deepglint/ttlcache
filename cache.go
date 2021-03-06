package ttlcache

import (
	"sync"
	"time"
)

// Cache is a synchronised map of items that auto-expire once stale
type Cache struct {
	mutex         sync.RWMutex
	ttl           time.Duration
	items         map[string]*Item
	Length        int
	FinishedItems chan string
}

// Set is a thread-safe way to add new items to the map
func (cache *Cache) Set(key string, data string) {
	cache.mutex.Lock()
	item := &Item{data: data}
	item.touch(cache.ttl)
	cache.items[key] = item
	cache.mutex.Unlock()
}

// Get is a thread-safe way to lookup items
// Every lookup, also touches the item, hence extending it's life
func (cache *Cache) Get(key string) (data string, found bool) {
	cache.mutex.Lock()
	item, exists := cache.items[key]
	if !exists || item.expired() {
		data = ""
		found = false
	} else {
		item.touch(cache.ttl)
		data = item.data
		found = true
	}
	cache.mutex.Unlock()
	return
}

// Delete is a thread-safe way to delete an item
func (cache *Cache) Delete(key string) {
	cache.mutex.Lock()
	delete(cache.items, key)
	cache.mutex.Unlock()
}

// Count returns the number of items in the cache
// (helpful for tracking memory leaks)
func (cache *Cache) Count() int {
	cache.mutex.RLock()
	count := len(cache.items)
	cache.mutex.RUnlock()
	return count
}

func (cache *Cache) cleanup() {
	cache.mutex.Lock()
	for key, item := range cache.items {
		if item.expired() {
			delete(cache.items, key)
			if len(cache.FinishedItems) == cache.Length {
				<-cache.FinishedItems
			}
			cache.FinishedItems <- item.data
		}
	}
	cache.mutex.Unlock()
}

func (cache *Cache) startCleanupTimer() {
	duration := cache.ttl
	if duration < time.Second {
		duration = time.Second
	}
	ticker := time.Tick(duration)
	go (func() {
		for {
			select {
			case <-ticker:
				cache.cleanup()
			}
		}
	})()
}

// NewCache is a helper to create instance of the Cache struct
func NewCache(duration time.Duration) *Cache {
	cache := &Cache{
		ttl:    duration,
		items:  map[string]*Item{},
		Length: 10,
	}
	cache.FinishedItems = make(chan string, cache.Length)
	cache.startCleanupTimer()
	return cache
}
