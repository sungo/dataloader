package main

import (
	"context"
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

	//lint:ignore SA1029 using a string here is fine for demo code
	doTheThing(context.WithValue(context.Background(), "loader", loader))
}

func doTheThing(ctx context.Context) {
	var wg sync.WaitGroup

	loader := ctx.Value("loader").(*dataloader.Loader[string, result])

	for i := 0; i < 10; i++ {
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
	}

	wg.Wait()
}
