package debrid

import (
	"context"
	"errors"
	"fmt"
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

// A refusal or a download limit is the account's answer, not the cache's:
// the second account of the same service is still asked.
func TestRefusalAndLimitStillTrySameServiceAccount(t *testing.T) {
	for name, fail := range map[string]error{
		"refused": fmt.Errorf("fake: %w", ErrAddRefused),
		"limit":   fmt.Errorf("fake: %w", core.ErrDownloadLimit),
	} {
		clk := newClock()
		s, provs := quotaService(t, clk, "alldebrid", "alldebrid")
		provs["alldebrid-0"].failErr = fail
		if _, err := s.Acquire(context.Background(), torrentRelease("a")); err != nil {
			t.Fatalf("%s: second account not used: %v", name, err)
		}
		if n := provs["alldebrid-1"].callCount("acquire"); n != 1 {
			t.Fatalf("%s: second account asked %d times, want 1", name, n)
		}
	}
}

// Second account paused for its quota after "not cached" on the first: the
// answer stays a plain "not cached" (that account could not change it).
func TestNotCachedWithPausedSameServiceAccount(t *testing.T) {
	clk := newClock()
	s, provs := quotaService(t, clk, "alldebrid", "alldebrid")
	delete(provs["alldebrid-0"].cached, testHash)
	s.observe(s.active()[1], ErrQuotaExceeded)
	_, err := s.Acquire(context.Background(), torrentRelease("a"))
	if !errors.Is(err, core.ErrNotCached) || errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("want plain ErrNotCached, got %v", err)
	}
}
