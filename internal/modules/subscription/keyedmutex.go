package subscription

import (
	"sync"

	"github.com/google/uuid"
)

// keyedMutex hands out one *sync.Mutex per key, so callers only serialize
// against operations on the SAME key (here: the same user's subscription/
// payment state) instead of blocking every user in the system behind a single
// global lock. This is what replaced the old service-wide paymentMu.
//
// The lock map only ever grows (one entry per distinct user ID ever seen),
// never shrinks — for a user base in the thousands-to-low-millions range this
// is a few dozen bytes each, a non-issue. It's not correct across multiple
// API instances/replicas (each process has its own map) — that would need a
// DB-level lock (e.g. SELECT ... FOR UPDATE) instead. Fine for a single-
// instance deployment; worth revisiting if this API is ever horizontally
// scaled behind a load balancer.
type keyedMutex struct {
	mu    sync.Mutex
	locks map[uuid.UUID]*sync.Mutex
}

func newKeyedMutex() *keyedMutex {
	return &keyedMutex{locks: make(map[uuid.UUID]*sync.Mutex)}
}

// Lock blocks until the lock for id is acquired and returns the matching
// Unlock func — call it via defer, same as a plain sync.Mutex.
func (k *keyedMutex) Lock(id uuid.UUID) func() {
	k.mu.Lock()
	l, ok := k.locks[id]
	if !ok {
		l = &sync.Mutex{}
		k.locks[id] = l
	}
	k.mu.Unlock()

	l.Lock()
	return l.Unlock
}
