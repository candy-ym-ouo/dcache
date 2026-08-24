package metrics

import (
	"sync"
	"testing"
)

func TestBug006ResetIsRaceFree(t *testing.T) {
	m := New()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5000; j++ {
				m.Request()
				m.Hit()
				m.Miss()
				if j%7 == 0 {
					m.Reset()
				}
			}
		}()
	}
	wg.Wait()
}
