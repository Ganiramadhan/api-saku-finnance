// Package keyedmutex provides a lock that serializes callers by key instead
// of globally — e.g. "one user's checkout/change-password can't proceed
// concurrently with itself" without blocking every other user in the system
// behind the same lock.
package keyedmutex

import "sync"

type KeyedMutex[K comparable] struct {
	mu    sync.Mutex
	locks map[K]*sync.Mutex
}

// New creates an empty KeyedMutex for key type K.
func New[K comparable]() *KeyedMutex[K] {
	return &KeyedMutex[K]{locks: make(map[K]*sync.Mutex)}
}
func (k *KeyedMutex[K]) Lock(key K) func() {
	k.mu.Lock()
	l, ok := k.locks[key]
	if !ok {
		l = &sync.Mutex{}
		k.locks[key] = l
	}
	k.mu.Unlock()

	l.Lock()
	return l.Unlock
}
