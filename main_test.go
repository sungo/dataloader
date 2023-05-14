package dataloader_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"git.sr.ht/~sungo/dataloader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fetch(keys []string) (map[string]string, error) {
	results := make(map[string]string)
	for idx := range keys {
		key := keys[idx]
		results[key] = "result " + key
	}
	return results, nil
}

func TestLoad(t *testing.T) {
	loader, err := dataloader.New(fetch)
	require.Nil(t, err)
	require.NotNil(t, loader)

	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		j := i
		t.Run(fmt.Sprintf("fetch %d", j), func(t *testing.T) {
			var (
				key      = fmt.Sprintf("wat %d", j)
				expected = "result " + key
			)
			defer wg.Done()
			result, err := loader.Load(key)
			require.Nil(t, err)
			assert.Equal(t, expected, result)
		})
	}

	wg.Wait()
}

func TestLoadMany(t *testing.T) {
	loader, err := dataloader.New(fetch)
	require.Nil(t, err)
	require.NotNil(t, loader)

	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)

		j := i
		t.Run(fmt.Sprintf("fetch %d", j), func(t *testing.T) {
			defer wg.Done()

			var (
				key      = fmt.Sprintf("watMany %d", j)
				many     = make([]string, 0)
				expected = make(map[string]string)
			)

			for k := 0; k < 5; k++ {
				subkey := fmt.Sprintf("%s - %d", key, k)
				many = append(many, subkey)
				expected[subkey] = "result " + subkey
			}

			result, err := loader.LoadMany(many...)
			require.Nil(t, err)

			assert.Equal(t, result, expected)
		})
	}

	wg.Wait()
}

func TestForceBatches(t *testing.T) {
	loader, err := dataloader.New(fetch)
	require.Nil(t, err)
	require.NotNil(t, loader)

	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		j := i
		t.Run(fmt.Sprintf("fetch %d", j), func(t *testing.T) {
			var (
				key      = fmt.Sprintf("wat %d", j)
				expected = "result " + key
			)
			defer wg.Done()

			// batch wait time is 5ms so this will force the requests into multiple batches
			time.Sleep(time.Duration(j) * time.Millisecond)

			result, err := loader.Load(key)
			require.Nil(t, err)

			assert.Equal(t, expected, result)
		})
	}

	wg.Wait()
}
