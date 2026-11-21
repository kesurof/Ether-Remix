package catalog

import (
	"context"
	"time"

	"github.com/braedonsaunders/ether/internal/core"
	"github.com/braedonsaunders/ether/internal/pse/listonly"
	"github.com/braedonsaunders/ether/internal/pse/localmedia"
)

// Listes légères — voir docs/pse/FORK.fr.md.

// pseListOnly indique que ctx est celui d'une liste : lecture de la base seule.
func pseListOnly(ctx context.Context) bool { return listonly.On(ctx) }

// pseListGet renvoie la ligne stockée, même périmée, ou le résumé de liste
// en mémoire. Sans ligne : core.ErrNotFound. Aucun appel TMDB, aucune écriture.
func (c *Catalog) pseListGet(ctx context.Context, key core.RatingKey) (*core.Title, error) {
	if localmedia.IsKey(key) && !localmedia.Known(ctx, c.db, key) {
		return nil, core.ErrNotFound
	}
	st, err := c.loadByKey(ctx, key)
	if err != nil {
		if summary, ok := c.listSummary(key); ok {
			st = &storedTitle{t: summary}
		} else {
			return nil, err
		}
	}
	if !c.visibleTitle(&st.t) {
		return nil, core.ErrNotFound
	}
	return &st.t, nil
}

// pseListCountMaxAge borne l'âge d'un total réutilisé par une liste légère.
const pseListCountMaxAge = 10 * time.Minute

// pseListCountFresh accepte, pour une liste légère, un total compté il y a
// moins de pseListCountMaxAge même si le catalogue a été écrit depuis : le
// total ne sert qu'à la pagination, et le recompter à chaque écriture coûtait
// une lecture complète du catalogue par affichage (#39).
func pseListCountFresh(ctx context.Context, counted time.Time) bool {
	return pseListOnly(ctx) && !counted.IsZero() && time.Since(counted) < pseListCountMaxAge
}

// pseListPageFresh accepte, pour une liste légère, une page mise en cache il
// y a moins de pseListCountMaxAge minutes, même si le catalogue a été écrit
// depuis. Un nouveau titre apparaît donc au plus tard après ce délai.
func pseListPageFresh(ctx context.Context, cached, now int64) bool {
	return pseListOnly(ctx) && cached > 0 && now-cached >= 0 && now-cached < int64(pseListCountMaxAge/time.Minute)
}
