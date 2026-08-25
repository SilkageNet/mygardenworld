package store

import (
	"bytes"
	"path/filepath"
	"sync"
	"testing"
)

func TestLoadOrCreateCredentialKeyIsAtomicAcrossCallers(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "garden.db")
	const callers = 32
	start := make(chan struct{})
	keys := make(chan []byte, callers)
	errs := make(chan error, callers)

	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			key, err := loadOrCreateCredentialKey(dbPath)
			if err != nil {
				errs <- err
				return
			}
			keys <- key
		}()
	}
	close(start)
	wg.Wait()
	close(keys)
	close(errs)

	for err := range errs {
		t.Fatal(err)
	}
	winner, err := readCredentialKey(dbPath + ".key")
	if err != nil {
		t.Fatal(err)
	}
	for key := range keys {
		if !bytes.Equal(key, winner) {
			t.Fatal("caller returned a key different from the published key")
		}
	}
}
