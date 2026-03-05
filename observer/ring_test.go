package observer

import (
	"fmt"
	"sync"
	"testing"
)

func TestRingBasic(t *testing.T) {
	r := NewRing(3)

	// Empty buffer
	if got := r.ReadAll(); got != nil {
		t.Errorf("empty ReadAll = %v, want nil", got)
	}
	if got := r.Len(); got != 0 {
		t.Errorf("empty Len = %d, want 0", got)
	}

	// Add items within capacity
	r.Write("a")
	r.Write("b")
	assertLines(t, r.ReadAll(), []string{"a", "b"})

	// Fill to capacity
	r.Write("c")
	assertLines(t, r.ReadAll(), []string{"a", "b", "c"})

	// Overflow — oldest dropped
	r.Write("d")
	assertLines(t, r.ReadAll(), []string{"b", "c", "d"})

	r.Write("e")
	r.Write("f")
	assertLines(t, r.ReadAll(), []string{"d", "e", "f"})
}

func TestRingReadN(t *testing.T) {
	r := NewRing(5)
	for i := 0; i < 5; i++ {
		r.Write(fmt.Sprintf("%d", i))
	}

	assertLines(t, r.ReadN(3), []string{"2", "3", "4"})
	assertLines(t, r.ReadN(1), []string{"4"})
	assertLines(t, r.ReadN(10), []string{"0", "1", "2", "3", "4"}) // clamped
}

func TestRingDrain(t *testing.T) {
	r := NewRing(3)
	r.Write("a")
	r.Write("b")

	drained := r.Drain()
	assertLines(t, drained, []string{"a", "b"})

	// After drain, buffer is empty
	if got := r.Len(); got != 0 {
		t.Errorf("after drain Len = %d, want 0", got)
	}
	if got := r.ReadAll(); got != nil {
		t.Errorf("after drain ReadAll = %v, want nil", got)
	}
}

func TestRingConcurrent(t *testing.T) {
	r := NewRing(100)
	var wg sync.WaitGroup

	// Concurrent writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				r.Write(fmt.Sprintf("w%d-%d", id, j))
			}
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				r.ReadAll()
				r.ReadN(10)
			}
		}()
	}

	wg.Wait()

	// Should have exactly 100 items (ring size)
	if got := r.Len(); got != 100 {
		t.Errorf("after concurrent writes Len = %d, want 100", got)
	}
}

func assertLines(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("len = %d, want %d\n  got:  %v\n  want: %v", len(got), len(want), got, want)
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
