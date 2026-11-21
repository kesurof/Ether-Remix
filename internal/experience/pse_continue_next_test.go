package experience

import (
	"context"
	"testing"
	"time"

	"github.com/braedonsaunders/ether/internal/core"
	"github.com/braedonsaunders/ether/internal/pse/continuewatch"
	"github.com/braedonsaunders/ether/internal/store"
)

// Ajout Remix : lecture de « Continuer à regarder » depuis la table « à suivre ».

func pseTable(t *testing.T) *continuewatch.Store {
	t.Helper()
	db, err := store.OpenMemory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	s := &continuewatch.Store{DB: db, Now: func() time.Time { return pseNow }}
	continuewatch.SetDefault(s)
	t.Cleanup(func() { continuewatch.SetDefault(nil); db.Close() })
	return s
}

func pseInsertRow(t *testing.T, s *continuewatch.Store, uid int64, p pseShow) {
	t.Helper()
	if _, err := s.DB.Exec(`INSERT INTO pse_continue_next(user_id,show_key,last_key,last_watched_at,next_key,next_released_ts,rank_ts) VALUES(?,?,?,?,?,?,?)`,
		uid, int64(core.ShowKey(p.tmdb)), int64(p.watched()), p.viewed.UnixNano(), int64(p.next()), p.released.Unix(), p.viewed.UnixNano()); err != nil {
		t.Fatal(err)
	}
}

func pseFill(t *testing.T, s *continuewatch.Store, uid int64) {
	t.Helper()
	if _, err := s.DB.Exec(`INSERT INTO pse_continue_next_filled(user_id,filled_at) VALUES(?,1)`, uid); err != nil {
		t.Fatal(err)
	}
}

func pseTableContinue(shows []pseShow) Continue {
	c, st := pseContinueFor(nil, pseDefaults())
	titles := map[core.RatingKey]core.Title{}
	for _, s := range shows {
		st.values[s.watched()] = core.WatchStatus{ViewCount: 1}
		titles[s.next()] = core.Title{Key: s.next(), Type: core.TypeEpisode, Season: 2, Episode: 1, Released: s.released}
	}
	c.GetTitles = func(_ context.Context, keys []core.RatingKey) []core.Title {
		var out []core.Title
		for _, k := range keys {
			if t, ok := titles[k]; ok {
				out = append(out, t)
			}
		}
		return out
	}
	return c
}

func TestPSEContinueTableReturnsEverySeriesSorted(t *testing.T) {
	s := pseTable(t)
	var shows []pseShow
	for i := int64(0); i < 250; i++ { // au-delà des 200 séries examinées
		p := pseShow{tmdb: 1000 + i, viewed: pseNow.Add(-time.Duration(i+1) * time.Hour), released: daysAgo(500)}
		shows = append(shows, p)
		pseInsertRow(t, s, 1, p)
	}
	pseFill(t, s, 1)
	c := pseTableContinue(shows) // aucun historique : seule la table peut répondre
	got, _ := c.InProgress(context.Background(), &core.User{ID: 1}, core.TypeShow, 0)
	if len(got) != 250 {
		t.Fatalf("séries : %d, attendu 250", len(got))
	}
	for i := range got {
		if got[i].Key != shows[i].next() {
			t.Fatalf("ordre rompu à %d : %v", i, got[i].Key)
		}
	}
	if n := len(pseKeys(first(c, 5))); n != 5 {
		t.Fatalf("limite du client : %d", n)
	}
}

func first(c Continue, n int) []core.Title {
	got, _ := c.InProgress(context.Background(), &core.User{ID: 1}, core.TypeShow, n)
	return got
}

func TestPSEContinueTableSkipsWatchedAndUnwatchedLast(t *testing.T) {
	s := pseTable(t)
	a := pseShow{tmdb: 1, viewed: daysAgo(3), released: daysAgo(500)}
	b := pseShow{tmdb: 2, viewed: daysAgo(4), released: daysAgo(500)}
	d := pseShow{tmdb: 3, viewed: daysAgo(5), released: daysAgo(500)}
	for _, p := range []pseShow{a, b, d} {
		pseInsertRow(t, s, 1, p)
	}
	pseFill(t, s, 1)
	c := pseTableContinue([]pseShow{a, b, d})
	st := c.State.(*pseHistoryState)
	st.values[a.next()] = core.WatchStatus{ViewCount: 1} // vu depuis le dernier calcul
	st.values[b.watched()] = core.WatchStatus{}          // remis à « non vu »
	if got := pseKeys(first(c, 10)); len(got) != 1 || got[0] != d.next() {
		t.Fatalf("attendu seulement la série 3 : %v", got)
	}
}

func TestPSEContinueFallsBackWhenTableNotFilled(t *testing.T) {
	s := pseTable(t)
	p := pseShow{tmdb: 1, viewed: daysAgo(3), released: daysAgo(500)}
	pseInsertRow(t, s, 1, p) // ligne présente mais spectateur pas marqué « rempli »
	c, _ := pseContinueFor([]pseShow{p}, pseDefaults())
	got, _ := c.InProgress(context.Background(), &core.User{ID: 1}, core.TypeShow, 5)
	if len(got) != 1 || got[0].Key != p.next() {
		t.Fatalf("l'ancien calcul doit répondre : %v", pseKeys(got))
	}
}
