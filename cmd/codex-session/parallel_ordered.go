package main

import (
	"context"
	"sync"
)

type orderedJob[T any] struct {
	idx  int
	item T
}

type orderedResult[R any] struct {
	idx int
	val R
	err error
}

// runOrderedWorkerPool runs work on items in parallel (bounded by maxWorkers) and emits results
// in the same order as items.
func runOrderedWorkerPool[T any, R any](
	ctx context.Context,
	items []T,
	maxWorkers int,
	work func(context.Context, T) (R, error),
	emit func(R) error,
) error {
	if maxWorkers < 1 {
		maxWorkers = 1
	}
	if len(items) == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan orderedJob[T])
	results := make(chan orderedResult[R])

	var wg sync.WaitGroup
	worker := func() {
		defer wg.Done()
		for job := range jobs {
			val, err := work(ctx, job.item)
			select {
			case results <- orderedResult[R]{idx: job.idx, val: val, err: err}:
			case <-ctx.Done():
				return
			}
		}
	}

	wg.Add(maxWorkers)
	for i := 0; i < maxWorkers; i++ {
		go worker()
	}

	go func() {
		defer close(jobs)
		for idx, item := range items {
			select {
			case jobs <- orderedJob[T]{idx: idx, item: item}:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	got := make([]bool, len(items))
	values := make([]R, len(items))
	next := 0

	for res := range results {
		if res.err != nil {
			cancel()
			return res.err
		}
		values[res.idx] = res.val
		got[res.idx] = true

		for next < len(items) && got[next] {
			if err := emit(values[next]); err != nil {
				cancel()
				return err
			}
			next++
		}
	}

	return nil
}
