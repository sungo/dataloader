package main

import (
	"fmt"
	"sync"
	"time"

	"git.sr.ht/~sungo/dataloader"
)

type result string

func fetch(keys []string) (map[string]result, error) {
	fmt.Printf("Batch: %d records requested\n", len(keys))
	results := make(map[string]result)
	for idx := range keys {
		key := keys[idx]
		results[key] = result("result " + key)
	}
	return results, nil
}

func main() {
	loader, err := dataloader.New(fetch)
	if err != nil {
		panic(err)
	}

	var (
		wg sync.WaitGroup
		i  = 0
	)

	for i < 10 {
		wg.Add(1)
		go func(j int) {
			defer wg.Done()
			// Force things into multiple batches
			time.Sleep(time.Duration(j) * time.Millisecond)
			result, err := loader.Load(fmt.Sprintf("wat%d", j))
			if err != nil {
				panic(err)
			}

			fmt.Printf("worker %d : %s\n", j, result)
		}(i)

		i++
	}

	wg.Wait()
}
