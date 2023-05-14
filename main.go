/*
Package dataloader implements the Facebook data loader pattern to optimize data fetches, using the power of generics

When Facebook gave the world GraphQL, it also gave us a data loading problem.

Consider this GraphQL schema:

	type Query {
		user(id: ID!): User
		users(ids: [ID!]!): [User!]!
	}

	type User {
		id: ID!
		name: String
	}

Under the hood, you'll typically have a resolver that loads an individual user
by ID, used in this case by the 'user' query. The 'users' query probably also
makes use of the 'user' loader, under the hood. And here's the problem. If
someone makes a call to 'users' and asks for 40 user ids, your GraphQL code will
make 40 database calls. They ask for 80000, you'll be making 80000 database
calls. That's... bad.

So what do? That's where this library comes in. The dataloader wraps those
requests in goroutines that batch the calls and run them concurrently. By
default, the max batch size is 1000 so those 80000 requests become 80, loaded in
parallel. (Typically, your fetch function will do something like a `select *
from wat where id in (?)`)

	func fetch(keys []string) (map[string]MyThing, error) {
		// ....
	}

	func main() {
		loader, err := dataloader.New(fetch)
		result, err := loader.Load("wat")

		results, err := loader.LoadMany("wat", "foo")
		for key, res := range results {}
	}

This library also works pretty well as the second stage for a search process.
The first stage would identify the relevant records, return the ids, and the
second stage would use this library to load the full records in parallel.

	ids := DoSearch("my * search string")
	results, err := loader.LoadMany(ids...)

See examples/simple/main.go for a fully formed example.

# Generics

The examples all use strings but since we're using generics, the keys are
anything comparable and values can be anything at all. The `fetch` function
needs to return a map of those keys to those values.

# Deduplication

Under the hood, we deduplicate the keys to be fetched in a particular batch. If
the loader is asked to fetch the same ID fifty times, that ID will only get
fetched once and handed back fifty times.

Let's say you're fetching the details of the place a user lives. The query will
ask for details on Los Angeles millions of times, but the library will only
bother to fetch it once. You are freed from having to deduplicate that yourself.

One super big caveat here. If the multiple requests span multiple batches, the
data will get fetched multiple times. We do no caching. So if batch size is 1000
and the details for Los Angeles are requested 10k times, the data will be
fetched 10 times. It's a little unintuitive but 10 is still better than 10k.

# Shared Loader

One way to use this library is to build a loader for each query. A GraphQL query
for 'users' happens, we create a loader and do the thing. This helps us, sure,
but we can take it further.

dataloader does no internal caching and is thread-safe. To gain the benefits of
batched loading and deduplication for everyone, you can create a loader at start
time, stick it in a context, and use it wherever. All loads from everywhere will
be batched together and benefits conferred.

This does require a little dance with type instantiation.

See examples/context/main.go for an example.
*/
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

// Loader represents an individual loader. BatchSize represents the breakpoint
// when the library will fork off a batch of fetches. Delay represents the time
// boundary at which point the batches are divided up.
//
// Plays out like this. When Load() is called for the first time, a Delay timer
// starts. When that timer fires, the request queue is examined, divided up into
// BatchSize bundles, and executed.
//
// If you want to customize BatchSize and Delay, do so immediately after calling
// New() so every new load will use those values.
type Loader[K comparable, V any] struct {
	BatchSize int           // defaults to 1000
	Delay     time.Duration // defaults to 5 milliseconds
	fn        FetchFunc[K, V]

	mut          sync.Mutex
	currentBatch *batch[K, V]
}

// New generates a new Loader with default BatchSize and Delay.
func New[K comparable, V any](fn FetchFunc[K, V]) (*Loader[K, V], error) {
	return &Loader[K, V]{
		BatchSize: defaultBatchSize,
		Delay:     defaultDelay,
		fn:        fn,
	}, nil
}

// Load returns the value V for key K, as determined by the fetch function
func (loader *Loader[K, V]) Load(key K) (V, error) {
	var empty V
	results, err := loader.LoadMany(key)
	if err != nil {
		return empty, err
	}

	return results[key], nil
}

// LoadMany returns a map of values V for keys K, as determined by the fetch function.
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
