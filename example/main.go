package main

import (
	"fmt"

	"git.sr.ht/~sungo/dataloader"
)

type result string

func fetch(keys []string) (map[string]result, error) {
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

	result, err := loader.Load("wat")
	if err != nil {
		panic(err)
	}

	fmt.Printf("%s\n", result)
}
