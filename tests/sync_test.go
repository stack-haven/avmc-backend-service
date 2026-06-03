package main

import (
	"sync"
	"testing"
)

func Test_WaitGroup(t *testing.T) {
	var wg sync.WaitGroup
	const jobs = 10
	wg.Add(jobs)
	for i := 0; i < jobs; i++ {
		go func() {
			defer wg.Done()
		}()
	}
	wg.Wait()
}
