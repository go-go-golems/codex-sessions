package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunOrderedWorkerPool_PreservesOrder(t *testing.T) {
	items := make([]int, 10)
	for i := range items {
		items[i] = i
	}

	emitted := make([]int, 0, len(items))
	err := runOrderedWorkerPool(context.Background(), items, 3,
		func(ctx context.Context, item int) (int, error) {
			// Intentionally finish later indices first.
			time.Sleep(time.Duration(len(items)-item) * 5 * time.Millisecond)
			return item, nil
		},
		func(v int) error {
			emitted = append(emitted, v)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("runOrderedWorkerPool: %v", err)
	}
	if len(emitted) != len(items) {
		t.Fatalf("expected %d emitted, got %d: %#v", len(items), len(emitted), emitted)
	}
	for i := range items {
		if emitted[i] != items[i] {
			t.Fatalf("expected ordering preserved, got emitted=%v", emitted)
		}
	}
}

func TestRunOrderedWorkerPool_RespectsMaxWorkers(t *testing.T) {
	items := make([]int, 25)
	for i := range items {
		items[i] = i
	}

	var active int32
	var maxSeen int32

	err := runOrderedWorkerPool(context.Background(), items, 4,
		func(ctx context.Context, item int) (int, error) {
			cur := atomic.AddInt32(&active, 1)
			for {
				prev := atomic.LoadInt32(&maxSeen)
				if cur <= prev || atomic.CompareAndSwapInt32(&maxSeen, prev, cur) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			atomic.AddInt32(&active, -1)
			return item, nil
		},
		func(v int) error { return nil },
	)
	if err != nil {
		t.Fatalf("runOrderedWorkerPool: %v", err)
	}
	if maxSeen > 4 {
		t.Fatalf("expected max concurrency <= 4, got %d", maxSeen)
	}
}
