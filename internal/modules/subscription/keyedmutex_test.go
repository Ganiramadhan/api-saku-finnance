package subscription

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestKeyedMutexSerializesSameKey(t *testing.T) {
	km := newKeyedMutex()
	id := uuid.New()

	var active int32
	var maxConcurrent int32
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock := km.Lock(id)
			defer unlock()

			n := atomic.AddInt32(&active, 1)
			for {
				max := atomic.LoadInt32(&maxConcurrent)
				if n <= max || atomic.CompareAndSwapInt32(&maxConcurrent, max, n) {
					break
				}
			}
			time.Sleep(time.Millisecond)
			atomic.AddInt32(&active, -1)
		}()
	}
	wg.Wait()

	if maxConcurrent != 1 {
		t.Fatalf("expected exactly 1 goroutine in the critical section at a time for the same key, got max concurrency %d", maxConcurrent)
	}
}

func TestKeyedMutexAllowsDifferentKeysConcurrently(t *testing.T) {
	km := newKeyedMutex()
	idA, idB := uuid.New(), uuid.New()

	bothInFlight := make(chan struct{})
	release := make(chan struct{})
	var wg sync.WaitGroup

	var entered int32
	hold := func(id uuid.UUID) {
		defer wg.Done()
		unlock := km.Lock(id)
		defer unlock()
		if atomic.AddInt32(&entered, 1) == 2 {
			close(bothInFlight)
		}
		<-release
	}

	wg.Add(2)
	go hold(idA)
	go hold(idB)

	select {
	case <-bothInFlight:
		// Good: two different keys ran concurrently without waiting on each other.
	case <-time.After(2 * time.Second):
		t.Fatal("locks for different keys blocked each other — expected independent locking per key")
	}
	close(release)
	wg.Wait()
}
