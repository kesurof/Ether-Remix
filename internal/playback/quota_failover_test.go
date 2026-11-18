package playback

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/braedonsaunders/ether/internal/core"
)

func torrentReleases(h *harness, ids ...string) {
	for i := range h.resolver.candidates {
		for _, id := range ids {
			if h.resolver.candidates[i].ID == id {
				h.resolver.candidates[i].Protocol = core.ProtocolTorrent
			}
		}
	}
}

func failingMKVs(err error, ids ...string) []*fakeRelease {
	var out []*fakeRelease
	for _, id := range ids {
		r := mkv(id, 100_000, time.Minute)
		r.OpenErr = err
		out = append(out, r)
	}
	return out
}

func TestFailoverAttemptsCapOpensAtMostThree(t *testing.T) {
	ids := []string{"A", "B", "C", "D", "E", "F"}
	h := newHarness(t, failingMKVs(errors.New("broken"), ids...), ids)
	if _, err := h.m.Prepare(context.Background(), owner, movieKey, defaultCaps("c1")); err == nil {
		t.Fatal("prepare succeeded with every release broken")
	}
	if got := h.engine.openCount(); got != 3 {
		t.Fatalf("%d releases opened, want the default cap of 3", got)
	}

	h = newHarness(t, failingMKVs(errors.New("broken"), ids...), ids, func(h *harness, _ *Options) { h.cfg.Resolver.FailoverAttempts = 2 })
	if _, err := h.m.Prepare(context.Background(), owner, movieKey, defaultCaps("c1")); err == nil {
		t.Fatal("prepare succeeded with every release broken")
	}
	if got := h.engine.openCount(); got != 2 {
		t.Fatalf("%d releases opened, want the configured 2", got)
	}
}

func TestDebridQuotaStopsDebridAttempts(t *testing.T) {
	quota := fmt.Errorf("alldebrid: HTTP 502: daily limit: %w", core.ErrDebridQuota)
	ids := []string{"A", "B", "C", "D"}
	h := newHarness(t, failingMKVs(quota, ids...), ids)
	torrentReleases(h, ids...)
	_, err := h.m.Prepare(context.Background(), owner, movieKey, defaultCaps("c1"))
	if !errors.Is(err, core.ErrDebridQuota) || !errors.Is(err, core.ErrNotAvailable) {
		t.Fatalf("error does not name the quota: %v", err)
	}
	if got := h.engine.openCount(); got != 1 {
		t.Fatalf("%d releases opened after the account quota error, want 1", got)
	}
}

func TestDebridQuotaStillTriesUsenet(t *testing.T) {
	quota := fmt.Errorf("alldebrid: daily limit: %w", core.ErrDebridQuota)
	a := failingMKVs(quota, "A", "B")
	rels := append(a, mkv("C", 100_000, time.Minute))
	h := newHarness(t, rels, []string{"A", "B", "C"})
	torrentReleases(h, "A", "B")
	ps, err := h.m.Prepare(context.Background(), owner, movieKey, defaultCaps("c1"))
	if err != nil || ps.Resolution.Release.ID != "C" {
		t.Fatalf("usenet release not reached: %v %+v", err, ps)
	}
	for _, id := range h.engine.opens {
		if id == "B" {
			t.Fatal("a second debrid release was opened after the quota error")
		}
	}
}

func TestRepeatedPlayRequestsShareTheRecentFailure(t *testing.T) {
	ids := []string{"A", "B"}
	h := newHarness(t, failingMKVs(errors.New("broken"), ids...), ids)
	caps := defaultCaps("c1")
	_, err1 := h.m.Prepare(context.Background(), owner, movieKey, caps)
	if !errors.Is(err1, core.ErrNotAvailable) {
		t.Fatalf("first prepare: %v", err1)
	}
	calls, opens := h.resolver.callCount(), h.engine.openCount()
	for range 5 {
		if _, err := h.m.Prepare(context.Background(), owner, movieKey, caps); !errors.Is(err, core.ErrNotAvailable) {
			t.Fatalf("repeat: %v", err)
		}
	}
	if h.resolver.callCount() != calls || h.engine.openCount() != opens {
		t.Fatalf("repeats resolved again: calls %d->%d opens %d->%d", calls, h.resolver.callCount(), opens, h.engine.openCount())
	}
	// Another client is a different request.
	if _, err := h.m.Prepare(context.Background(), owner, movieKey, defaultCaps("c2")); err == nil || h.resolver.callCount() == calls {
		t.Fatalf("other client was answered from the first one's failure")
	}
	// After the window the title is resolved again.
	h.clock.Advance(retryFailureWindow + time.Second)
	calls = h.resolver.callCount()
	_, _ = h.m.Prepare(context.Background(), owner, movieKey, caps)
	if h.resolver.callCount() == calls {
		t.Fatal("expired failure still answers")
	}
}

func TestRepeatedFailoverSharesTheRecentFailure(t *testing.T) {
	a, b := mkv("A", 100_000, time.Minute), mkv("B", 100_000, time.Minute)
	b.OpenErr = errors.New("broken")
	h := newHarness(t, []*fakeRelease{a, b}, []string{"A", "B"})
	ps := h.prepare(owner, movieKey, "c1")
	if _, err := h.m.Failover(context.Background(), ps.ID, time.Second); err == nil {
		t.Fatal("failover found a release")
	}
	calls, opens := h.resolver.callCount(), h.engine.openCount()
	for range 4 {
		if _, err := h.m.Failover(context.Background(), ps.ID, time.Second); err == nil {
			t.Fatal("repeated failover found a release")
		}
	}
	if h.resolver.callCount() != calls || h.engine.openCount() != opens {
		t.Fatalf("repeats searched again: calls %d->%d opens %d->%d", calls, h.resolver.callCount(), opens, h.engine.openCount())
	}
}

func TestVerifiedFileReplacesRememberedAddonChoice(t *testing.T) {
	verified := core.Release{ID: core.VerifiedFileIDPrefix + "x", GUID: core.VerifiedFileIDPrefix + "x", Name: "Alko", Size: 100_000, Protocol: core.ProtocolHTTP, StreamDebrid: "alldebrid", StreamURL: "https://alldebrid.com/f/abc"}
	if !core.IsVerifiedFile(verified) {
		t.Fatal("fixture is not a verified file")
	}
	addon := core.Release{ID: "A", Protocol: core.ProtocolHTTP}
	h := newHarness(t, []*fakeRelease{mkv("A", 100_000, time.Minute), mkv(verified.ID, 100_000, time.Minute)}, []string{"A"})
	h.resolver.candidates[0].Protocol, h.resolver.candidates[0].StreamURL = core.ProtocolHTTP, "https://addon.example/stream"
	if !h.m.verifiedFileOutranks(verified, addon) || h.m.verifiedFileOutranks(addon, verified) || h.m.verifiedFileOutranks(verified, verified) {
		t.Fatal("verifiedFileOutranks")
	}
	for _, mode := range []string{"off", "tie"} {
		cfg := h.cfg
		cfg.Resolver.VerifiedFiles = mode
		h.m.SetConfig(cfg)
		if h.m.verifiedFileOutranks(verified, addon) {
			t.Fatalf("mode %q must keep the remembered choice", mode)
		}
	}
	cfg := h.cfg
	cfg.Resolver.VerifiedFiles = "first"
	h.m.SetConfig(cfg)

	// The addon stream was played and remembered; the verified file appears
	// later. A stale picked version falls back: the file leads.
	caps := defaultCaps("c1")
	first, err := h.m.Prepare(context.Background(), owner, movieKey, caps)
	if err != nil || first.Resolution.Release.ID != "A" {
		t.Fatalf("first play: %v %+v", err, first)
	}
	h.m.waitResumeSaves()
	h.m.Stop(first.ID)
	h.resolver.mu.Lock()
	h.resolver.candidates = append([]core.Release{verified}, h.resolver.candidates...)
	h.resolver.mu.Unlock()
	ps, err := h.m.PrepareVariant(context.Background(), owner, movieKey, caps, "gone")
	if err != nil {
		t.Fatal(err)
	}
	if ps.Resolution.Release.ID != verified.ID {
		t.Fatalf("played %s, want the verified file", ps.Resolution.Release.ID)
	}
}
