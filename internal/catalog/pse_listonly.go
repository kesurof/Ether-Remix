package catalog

import (
	"context"

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
