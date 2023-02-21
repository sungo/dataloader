package dataloader

import (
	"sync"
	"time"
)

const (
	defaultBatchSize = 1000
	defaultDelay     = 5 * time.Millisecond
)

type FetchFunc[K comparable, V any] func([]K) (map[K]V, error)

type batch[K comparable, V any] struct {
	batchSize int
	ch        chan bool
	mut       sync.RWMutex
	fn        FetchFunc[K, V]

	err     error
	keys    map[K]bool
	results map[K]V
}

type Loader[K comparable, V any] struct {
	BatchSize int
	Delay     time.Duration
	fn        FetchFunc[K, V]

	mut          sync.Mutex
	currentBatch *batch[K, V]
}

func New[K comparable, V any](fn FetchFunc[K, V]) (*Loader[K, V], error) {
	return &Loader[K, V]{
		BatchSize: defaultBatchSize,
		Delay:     defaultDelay,
		fn:        fn,
	}, nil
}

func (loader *Loader[K, V]) Load(key K) (V, error) {
	var empty V
	results, err := loader.LoadMany(key)
	if err != nil {
		return empty, err
	}

	return results[key], nil
}

func (loader *Loader[K, V]) LoadMany(keys ...K) (map[K]V, error) {
	loader.mut.Lock()
	if loader.currentBatch == nil {
		loader.currentBatch = &batch[K, V]{
			batchSize: loader.BatchSize,
			keys:      make(map[K]bool),
			results:   make(map[K]V),
			ch:        make(chan bool),
			fn:        loader.fn,
		}
		go loader.run()
	}

	bat := loader.currentBatch
	loader.mut.Unlock()

	bat.mut.Lock()
	for _, key := range keys {
		bat.keys[key] = true
	}

	ch := bat.ch
	bat.mut.Unlock()

	<-ch

	bat.mut.RLock()
	results := make(map[K]V)
	if bat.err != nil {
		return results, bat.err
	}

	for _, key := range keys {
		results[key] = bat.results[key]
	}
	bat.mut.RUnlock()

	return results, nil
}

func (loader *Loader[K, V]) run() {
	time.Sleep(loader.Delay)

	loader.mut.Lock()
	bat := loader.currentBatch
	loader.currentBatch = nil
	loader.mut.Unlock()

	bat.mut.Lock()
	defer bat.mut.Unlock()

	keys := make([]K, 0)
	for key := range bat.keys {
		keys = append(keys, key)
	}

	var (
		chunks     = chunk(keys, bat.batchSize)
		wgChan     = make(chan bool)
		errChan    = make(chan error)
		resultChan = make(chan map[K]V)
	)

	go func() {
		var wg sync.WaitGroup
		for idx := range chunks {
			wg.Add(1)
			go func(chunk []K) {
				defer wg.Done()
				results, err := bat.fn(chunk)
				if err != nil {
					errChan <- err
					return
				}
				resultChan <- results
			}(chunks[idx])
		}

		wg.Wait()
		close(wgChan)
	}()

loop:
	for {
		select {
		case results := <-resultChan:
			for key := range results {
				bat.results[key] = results[key]
			}
		case err := <-errChan:
			bat.err = err
			break loop
		case <-wgChan:
			break loop
		}
	}

	close(bat.ch)
}

func chunk[T any](items []T, size int) [][]T {
	chunks := make([][]T, 0, (len(items)/size)+1)
	for size < len(items) {
		items, chunks = items[size:], append(chunks, items[0:size:size])
	}
	return append(chunks, items)
}
