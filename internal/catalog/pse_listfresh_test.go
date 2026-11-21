package catalog

import (
	"testing"
	"time"

	"github.com/braedonsaunders/ether/internal/pse/listonly"
)

func TestPSEListFreshness(t *testing.T) {
	list := listonly.With(t.Context())
	if !pseListCountFresh(list, time.Now().Add(-9*time.Minute)) || pseListCountFresh(list, time.Now().Add(-11*time.Minute)) {
		t.Fatal("total d'une liste légère : 10 minutes attendues")
	}
	if pseListCountFresh(t.Context(), time.Now()) || pseListPageFresh(t.Context(), 100, 100) {
		t.Fatal("hors liste légère, le cache d'Ether ne change pas")
	}
	if !pseListPageFresh(list, 100, 109) || pseListPageFresh(list, 100, 110) || pseListPageFresh(list, 0, 5) {
		t.Fatal("page d'une liste légère : 10 minutes attendues")
	}
}
