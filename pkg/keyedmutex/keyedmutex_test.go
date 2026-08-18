package keyedmutex

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestKeyedMutexSerializesSameKey(t *testing.T) {
	km := New[string]()
	const key = "user-a"

	var active int32
	var maxConcurrent int32
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock := km.Lock(key)
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
	km := New[string]()

	bothInFlight := make(chan struct{})
	release := make(chan struct{})
	var wg sync.WaitGroup
	var entered int32

	hold := func(key string) {
		defer wg.Done()
		unlock := km.Lock(key)
		defer unlock()
		if atomic.AddInt32(&entered, 1) == 2 {
			close(bothInFlight)
		}
		<-release
	}

	wg.Add(2)
	go hold("user-a")
	go hold("user-b")

	select {
	case <-bothInFlight:
	case <-time.After(2 * time.Second):
		t.Fatal("locks for different keys blocked each other — expected independent locking per key")
	}
	close(release)
	wg.Wait()
}
