package playback

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/braedonsaunders/ether/internal/core"
)

func TestFailoverAttemptsCap(t *testing.T) {
	h := newHarness(t, nil, nil)
	if got := h.m.failoverAttempts(); got != 3 {
		t.Fatalf("défaut = %d, attendu 3", got)
	}
	cfg := h.cfg
	cfg.Resolver.FailoverAttempts = 2
	h.m.SetConfig(cfg)
	if got := h.m.failoverAttempts(); got != 2 {
		t.Fatalf("configuré = %d, attendu 2", got)
	}
	cfg.Resolver.FailoverAttempts = 10
	h.m.SetConfig(cfg)
	if got := h.m.failoverAttempts(); got != 10 {
		t.Fatalf("réglage 10 appliqué tel quel: %d", got)
	}
}

func TestOpenInitialStopsAtAttemptCap(t *testing.T) {
	var rels []*fakeRelease
	ids := []string{"a", "b", "c", "d", "e", "f"}
	for _, id := range ids {
		rels = append(rels, &fakeRelease{ID: id, Name: id + ".mkv", Size: 1 << 20, Duration: time.Minute, OpenErr: fmt.Errorf("boom %s: %w", id, core.ErrSourceUnavailable)})
	}
	h := newHarness(t, rels, ids)
	if _, err := h.m.Prepare(context.Background(), owner, movieKey, defaultCaps("c")); err == nil {
		t.Fatal("attendu un échec")
	}
	if n := h.engine.openCount(); n != 3 {
		t.Fatalf("%d releases essayées, plafond 3", n)
	}
}

func TestAccountErrorStopsDebridReleases(t *testing.T) {
	quota := fmt.Errorf("alldebrid: daily limit: %w", core.ErrDebridQuota)
	rels := []*fakeRelease{
		{ID: "t1", Name: "t1.mkv", Size: 1 << 20, Duration: time.Minute, OpenErr: quota},
		{ID: "t2", Name: "t2.mkv", Size: 1 << 20, Duration: time.Minute},
		{ID: "t3", Name: "t3.mkv", Size: 1 << 20, Duration: time.Minute},
	}
	h := newHarness(t, rels, []string{"t1", "t2", "t3"})
	for i := range h.resolver.candidates {
		h.resolver.candidates[i].Protocol = core.ProtocolTorrent
	}
	_, err := h.m.Prepare(context.Background(), owner, movieKey, defaultCaps("c"))
	if !errors.Is(err, core.ErrDebridQuota) {
		t.Fatalf("err = %v, attendu ErrDebridQuota", err)
	}
	if n := h.engine.openCount(); n != 1 {
		t.Fatalf("%d releases ouvertes après une erreur de quota, attendu 1", n)
	}
}

func TestAccountErrorKeepsUsenetReleases(t *testing.T) {
	quota := fmt.Errorf("alldebrid: %w", core.ErrDebridQuota)
	rels := []*fakeRelease{
		{ID: "t1", Name: "t1.mkv", Size: 1 << 20, Duration: time.Minute, OpenErr: quota},
		{ID: "u1", Name: "u1.mkv", Size: 1 << 20, Duration: time.Minute},
	}
	h := newHarness(t, rels, []string{"t1", "u1"})
	h.resolver.candidates[0].Protocol = core.ProtocolTorrent
	if _, err := h.m.Prepare(context.Background(), owner, movieKey, defaultCaps("c")); err != nil {
		t.Fatalf("la release Usenet doit passer: %v", err)
	}
}

func TestDependsOnDebrid(t *testing.T) {
	if dependsOnDebrid(core.Release{}) || !dependsOnDebrid(core.Release{Protocol: core.ProtocolTorrent}) ||
		!dependsOnDebrid(core.Release{Protocol: core.ProtocolHTTP, StreamDebrid: "alldebrid"}) || dependsOnDebrid(core.Release{Protocol: core.ProtocolHTTP}) {
		t.Fatal("classement des releases dépendant du debrid")
	}
	if got := withoutDebrid([]core.Release{{ID: "a"}, {ID: "b", Protocol: core.ProtocolTorrent}}); len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("withoutDebrid = %+v", got)
	}
}

func TestRepeatedFailedPrepareIsGrouped(t *testing.T) {
	h := newHarness(t, nil, nil)
	for range 3 {
		if _, err := h.m.Prepare(context.Background(), owner, movieKey, defaultCaps("c")); !errors.Is(err, core.ErrNotAvailable) {
			t.Fatalf("err = %v", err)
		}
	}
	if n := h.resolver.callCount(); n != 1 {
		t.Fatalf("%d résolutions pour 3 demandes rapprochées, attendu 1", n)
	}
	// Another client is another request.
	if _, err := h.m.Prepare(context.Background(), owner, movieKey, defaultCaps("other")); err == nil {
		t.Fatal("attendu un échec")
	}
	if n := h.resolver.callCount(); n != 2 {
		t.Fatalf("autre client: %d résolutions", n)
	}
	// After the window the title is resolved again.
	h.clock.Advance(retryFailureWindow + time.Second)
	_, _ = h.m.Prepare(context.Background(), owner, movieKey, defaultCaps("c"))
	if n := h.resolver.callCount(); n != 3 {
		t.Fatalf("après la fenêtre: %d résolutions", n)
	}
	// A settings change forgets the memory.
	_, _ = h.m.Prepare(context.Background(), owner, movieKey, defaultCaps("c"))
	h.m.SetConfig(h.cfg)
	_, _ = h.m.Prepare(context.Background(), owner, movieKey, defaultCaps("c"))
	if n := h.resolver.callCount(); n != 4 {
		t.Fatalf("après changement de réglages: %d résolutions", n)
	}
}

func TestSessionFailoverFailureWindow(t *testing.T) {
	s := &session{}
	g, other := &generation{}, &generation{}
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	boom := fmt.Errorf("x: %w", core.ErrNotAvailable)
	s.noteFailoverFailure(g, boom, now)
	if s.recentFailoverFailure(g, now.Add(time.Second)) == nil {
		t.Fatal("échec récent non rappelé")
	}
	if s.recentFailoverFailure(other, now.Add(time.Second)) != nil {
		t.Fatal("autre génération concernée")
	}
	if s.recentFailoverFailure(g, now.Add(retryFailureWindow)) != nil {
		t.Fatal("fenêtre non expirée")
	}
	s.noteFailoverFailure(g, context.Canceled, now)
	if retryableByWindow(context.Canceled) {
		t.Fatal("annulation mémorisée")
	}
}

func TestVerifiedFileOutranksRemembered(t *testing.T) {
	h := newHarness(t, nil, nil)
	vf := core.Release{
		ID: core.VerifiedFileIDPrefix + "x", Size: 1 << 20, Protocol: core.ProtocolHTTP, StreamDebrid: "alldebrid",
		StreamURL: "https://alldebrid.com/f/abc",
	}
	if !core.IsVerifiedFile(vf) {
		t.Skip("fixture de fichier vérifié invalide")
	}
	addon := core.Release{ID: "addon", Protocol: core.ProtocolHTTP, StreamURL: "https://x/y"}
	if !h.m.verifiedFileOutranks(vf, addon) {
		t.Fatal("fichier vérifié non prioritaire sur un flux d'addon (défaut first)")
	}
	if h.m.verifiedFileOutranks(addon, vf) || h.m.verifiedFileOutranks(vf, vf) {
		t.Fatal("priorité inversée")
	}
	cfg := h.cfg
	cfg.Resolver.VerifiedFiles = "off"
	h.m.SetConfig(cfg)
	if h.m.verifiedFileOutranks(vf, addon) {
		t.Fatal("verifiedFiles=off ignoré")
	}
}

// A debrid release skipped after an account error does not spend an attempt:
// a Usenet release behind several of them is still reached.
func TestSkippedDebridReleasesDoNotSpendAttempts(t *testing.T) {
	quota := fmt.Errorf("alldebrid: %w", core.ErrDebridQuota)
	rels := []*fakeRelease{
		{ID: "t1", Name: "t1.mkv", Size: 1 << 20, Duration: time.Minute, OpenErr: quota},
		{ID: "t2", Name: "t2.mkv", Size: 1 << 20, Duration: time.Minute},
		{ID: "t3", Name: "t3.mkv", Size: 1 << 20, Duration: time.Minute},
		{ID: "t4", Name: "t4.mkv", Size: 1 << 20, Duration: time.Minute},
		{ID: "u1", Name: "u1.mkv", Size: 1 << 20, Duration: time.Minute},
	}
	h := newHarness(t, rels, []string{"t1", "t2", "t3", "t4", "u1"})
	for i := range h.resolver.candidates {
		if h.resolver.candidates[i].ID != "u1" {
			h.resolver.candidates[i].Protocol = core.ProtocolTorrent
		}
	}
	ps, err := h.m.Prepare(context.Background(), owner, movieKey, defaultCaps("c"))
	if err != nil || ps.Resolution.Release.ID != "u1" {
		t.Fatalf("la release Usenet doit être atteinte: %v %+v", err, ps)
	}
	if n := h.engine.openCount(); n != 2 {
		t.Fatalf("%d ouvertures, attendu 2 (t1 puis u1)", n)
	}
}

// With verifiedFiles "first", a warm-up or resume on an add-on stream gives
// way to a fresh ranking even without a picked version.
func TestRememberedAddonWithoutPickGivesWayToVerifiedFile(t *testing.T) {
	verified := core.Release{ID: core.VerifiedFileIDPrefix + "y", GUID: core.VerifiedFileIDPrefix + "y", Name: "Alko", Size: 100_000, Protocol: core.ProtocolHTTP, StreamDebrid: "alldebrid", StreamURL: "https://alldebrid.com/f/def"}
	h := newHarness(t, []*fakeRelease{mkv("A", 100_000, time.Minute), mkv(verified.ID, 100_000, time.Minute)}, []string{"A"})
	h.resolver.candidates[0].Protocol, h.resolver.candidates[0].StreamURL = core.ProtocolHTTP, "https://addon.example/stream"
	caps := defaultCaps("c1")
	first, err := h.m.Prepare(context.Background(), owner, movieKey, caps)
	if err != nil || first.Resolution.Release.ID != "A" {
		t.Fatalf("first play: %v", err)
	}
	h.m.waitResumeSaves()
	h.m.Stop(first.ID)
	h.resolver.mu.Lock()
	h.resolver.candidates = append([]core.Release{verified}, h.resolver.candidates...)
	h.resolver.mu.Unlock()
	ps, err := h.m.Prepare(context.Background(), owner, movieKey, caps)
	if err != nil || ps.Resolution.Release.ID != verified.ID {
		t.Fatalf("attendu le fichier vérifié: %v %+v", err, ps)
	}
}
