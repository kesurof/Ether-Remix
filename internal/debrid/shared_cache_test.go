package debrid

import (
	"context"
	"errors"
	"testing"

	"github.com/braedonsaunders/ether/internal/core"
)

// Issue #7: a debrid service's cache is shared by its accounts. "Not cached"
// on the first AllDebrid account is not asked again of the second.
func TestNotCachedNotAskedAgainOfSameService(t *testing.T) {
	clk := newClock()
	s, provs := quotaService(t, clk, "alldebrid", "alldebrid")
	for _, p := range provs {
		delete(p.cached, testHash)
	}
	_, err := s.Acquire(context.Background(), torrentRelease("a"))
	if !errors.Is(err, core.ErrNotCached) {
		t.Fatalf("want ErrNotCached, got %v", err)
	}
	if n := provs["alldebrid-0"].callCount("acquire"); n != 1 {
		t.Fatalf("first account asked %d times, want 1", n)
	}
	if n := provs["alldebrid-1"].callCount("acquire"); n != 0 {
		t.Fatalf("second account of the same service asked %d times, want 0", n)
	}
}

// An account error (quota) on the first account still moves on to the
// second account of the same service.
func TestAccountErrorStillTriesSameServiceAccount(t *testing.T) {
	clk := newClock()
	s, provs := quotaService(t, clk, "alldebrid", "alldebrid")
	provs["alldebrid-0"].quotaFail = true
	if _, err := s.Acquire(context.Background(), torrentRelease("a")); err != nil {
		t.Fatalf("second account not used after a quota: %v", err)
	}
	if n := provs["alldebrid-1"].callCount("acquire"); n != 1 {
		t.Fatalf("second account asked %d times, want 1", n)
	}
}

// Another service has its own cache: it is still asked.
func TestNotCachedStillAsksOtherService(t *testing.T) {
	clk := newClock()
	s, provs := quotaService(t, clk, "alldebrid", "torbox")
	delete(provs["alldebrid-0"].cached, testHash)
	if _, err := s.Acquire(context.Background(), torrentRelease("a")); err != nil {
		t.Fatalf("other service not used: %v", err)
	}
	if n := provs["torbox-1"].callCount("acquire"); n != 1 {
		t.Fatalf("other service asked %d times, want 1", n)
	}
}
