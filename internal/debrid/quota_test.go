package debrid

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/braedonsaunders/ether/internal/clusterbus"
	"github.com/braedonsaunders/ether/internal/config"
	"github.com/braedonsaunders/ether/internal/store"
)

func quotaEntry() *entry {
	return &entry{cfg: config.DebridProvider{ID: "ad", Name: "AllDebrid", Enabled: true}, sem: newLimiter(1)}
}

func TestQuotaMessageClassified(t *testing.T) {
	for _, msg := range []string{"You have reached your daily download limit", "Daily quota exceeded"} {
		for _, status := range []int{502, 403, 429} {
			err := statusError("alldebrid", status, msg)
			if !errors.Is(err, ErrQuotaExceeded) || errors.Is(err, ErrAuth) {
				t.Errorf("%d %q: %v", status, msg, err)
			}
		}
	}
	if err := statusError("alldebrid", 502, "bad gateway"); errors.Is(err, ErrQuotaExceeded) {
		t.Errorf("plain 502 taken for a quota: %v", err)
	}
	if err := adError(200, "SOME_CODE", "You have reached your daily download limit"); !errors.Is(err, ErrQuotaExceeded) {
		t.Errorf("adError: %v", err)
	}
}

func TestQuotaPausesAnHourThenSingleProbe(t *testing.T) {
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	e := quotaEntry()
	qerr := fmt.Errorf("alldebrid: HTTP 502: daily limit: %w", ErrQuotaExceeded)
	ev := e.observe(now, qerr)
	if ev == nil || ev.State != stateQuota || e.state != stateQuota || !e.until.Equal(now.Add(quotaPause)) {
		t.Fatalf("pause: ev=%+v state=%q until=%v", ev, e.state, e.until)
	}
	err := e.blocked(now.Add(59 * time.Minute))
	if err == nil || !errors.Is(err, ErrQuotaExceeded) || !strings.Contains(err.Error(), "11:00") {
		t.Fatalf("blocked during pause: %v", err)
	}

	// At expiry exactly one request is let through; the others wait.
	at := now.Add(quotaPause)
	var passed atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if e.blocked(at) == nil {
				passed.Add(1)
			}
		}()
	}
	wg.Wait()
	if passed.Load() != 1 {
		t.Fatalf("%d concurrent requests passed, want 1", passed.Load())
	}

	// The test hits the quota again: another hour.
	ev = e.observe(at.Add(time.Second), qerr)
	if ev == nil || !e.until.Equal(at.Add(time.Second+quotaPause)) || e.blocked(at.Add(time.Minute)) == nil {
		t.Fatalf("second pause: ev=%+v until=%v", ev, e.until)
	}

	// Next window: the test succeeds and the provider is normal again.
	at = e.until
	if e.blocked(at) != nil {
		t.Fatal("no test request after the second pause")
	}
	if e.blocked(at) == nil {
		t.Fatal("second concurrent request passed during the test")
	}
	ev = e.observe(at.Add(time.Second), nil)
	if ev == nil || ev.State != "" || e.state != "" {
		t.Fatalf("recovery: ev=%+v state=%q", ev, e.state)
	}
	if e.blocked(at.Add(2*time.Second)) != nil || e.blocked(at.Add(2*time.Second)) != nil {
		t.Fatal("still blocked after recovery")
	}
}

func TestQuotaProbeLeaseAndOtherFailure(t *testing.T) {
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	e := quotaEntry()
	e.observe(now, ErrQuotaExceeded)
	at := now.Add(quotaPause)
	if e.blocked(at) != nil || e.blocked(at.Add(time.Second)) == nil {
		t.Fatal("single test not enforced")
	}
	// A test whose caller never answered does not block the provider forever.
	if e.blocked(at.Add(quotaProbeLease)) != nil {
		t.Fatal("stuck test never expires")
	}
	// A non-quota failure of the test keeps the quota state and allows a new test.
	e.observe(at.Add(quotaProbeLease+time.Second), errors.New("network down"))
	if e.state != stateQuota || e.blocked(at.Add(quotaProbeLease+2*time.Second)) != nil {
		t.Fatalf("state=%q", e.state)
	}
}

func TestQuotaPauseReachesOtherNodes(t *testing.T) {
	db := store.OpenTest(t)
	hub := clusterbus.NewHub()
	clkA, clkB := newClock(), newClock()
	prov := newFakeProvider("realdebrid", newMediaServer(t, 1<<10), movieFiles(1<<10))
	a := clusterNodeSvc(t, db, hub.Node("a"), prov, clkA.now)
	b := clusterNodeSvc(t, db, hub.Node("b"), prov, clkB.now)
	ea, eb := a.active()[0], b.active()[0]
	a.observe(ea, ErrQuotaExceeded)
	if st := b.Stats()[0]; st.State != stateQuota {
		t.Fatalf("B state %+v", st)
	}
	if err := eb.blocked(clkB.now().Add(59 * time.Minute)); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("B not paused for the hour: %v", err)
	}
	eb.applyRemote(clkB.now(), backoffEvent{Provider: "rd", State: stateQuota, Until: clkB.now().Add(48 * time.Hour).UnixMilli()})
	if eb.until.After(clkB.now().Add(quotaPause)) {
		t.Fatal("remote quota pause not capped")
	}
}
