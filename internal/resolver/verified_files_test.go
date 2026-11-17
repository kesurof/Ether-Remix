package resolver

import (
	"context"
	"testing"

	"github.com/braedonsaunders/ether/internal/core"
)

// alkoRelease is a verified AllDebrid file as the StreamFusion Reborn
// importer builds it.
func alkoRelease(id, name string, sizeMB int64) core.Release {
	r := rel("sfr-alko:"+id, name, sizeMB)
	r.NZBURL = ""
	r.Protocol = core.ProtocolHTTP
	r.Indexer = "StreamFusion Reborn (Alkodiques)"
	r.StreamURL = "https://alldebrid.com/f/" + id
	r.StreamDebrid, r.StreamOnDemand = "alldebrid", true
	r.DebridCached = []string{"alldebrid"}
	return r
}

// remuxTorrent est le torrent de meilleure résolution (1080p) face à
// l'Alkodique 720p : le rang qualité du profil 1080p le place devant.
func remuxTorrent() core.Release {
	r := rel("torrent-remux", "The.Signal.1999.1080p.WEB-DL.DDP5.1.H.264-GRP", 8000)
	r.NZBURL, r.Protocol, r.InfoHash = "", core.ProtocolTorrent, hashA
	return r
}

func TestVerifiedFilesRanking(t *testing.T) {
	const web = "The.Signal.1999.720p.WEB-DL.DDP5.1.H.264-ALKO"
	tests := []struct {
		name     string
		mode     string
		releases []core.Release
		want     string
	}{
		{"first: l'Alkodique passe devant un meilleur rang", "first", []core.Release{remuxTorrent(), alkoRelease("a1", web, 3000)}, "sfr-alko:a1"},
		{"défaut vide = first", "", []core.Release{remuxTorrent(), alkoRelease("a1", web, 3000)}, "sfr-alko:a1"},
		{"valeur inconnue = first", "bizarre", []core.Release{remuxTorrent(), alkoRelease("a1", web, 3000)}, "sfr-alko:a1"},
		{"off: comportement actuel", "off", []core.Release{remuxTorrent(), alkoRelease("a1", web, 3000)}, "torrent-remux"},
		{"tie: le meilleur rang garde la tête", "tie", []core.Release{remuxTorrent(), alkoRelease("a1", web, 3000)}, "torrent-remux"},
		{"hors résolution min du profil: jamais promu", "first", []core.Release{remuxTorrent(), alkoRelease("a1", "The.Signal.1999.480p.WEB-DL.DDP5.1.H.264-ALKO", 1500)}, "torrent-remux"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testCfg()
			cfg.Resolver.VerifiedFiles = tc.mode
			svc := New(&fakeIndexer{releases: tc.releases}, fakeHealth{}, nil, cfg, Options{})
			if got := resolveOrder(t, svc, matrixTitle()); got[0] != tc.want {
				t.Fatalf("ordre %v, tête attendue %s", got, tc.want)
			}
		})
	}
}

func TestVerifiedFilesTieOrdersEqualQuality(t *testing.T) {
	web := "The.Signal.1999.1080p.WEB-DL.DDP5.1.H.264-GRP"
	tor := rel("torrent-web", web, 6000)
	tor.NZBURL, tor.Protocol, tor.InfoHash = "", core.ProtocolTorrent, hashA
	tor.Grabs = 5000
	cfg := testCfg()
	cfg.Resolver.VerifiedFiles = "tie"
	svc := New(&fakeIndexer{releases: []core.Release{tor, alkoRelease("a1", web, 3000)}}, fakeHealth{}, nil, cfg, Options{})
	if got := resolveOrder(t, svc, matrixTitle()); got[0] != "sfr-alko:a1" {
		t.Fatalf("ordre %v, l'Alkodique devait mener à qualité égale", got)
	}
}

func TestVerifiedFileFailureFallsBack(t *testing.T) {
	cfg := testCfg()
	svc := New(&fakeIndexer{releases: []core.Release{remuxTorrent(), alkoRelease("a1", "The.Signal.1999.1080p.WEB-DL.DDP5.1.H.264-ALKO", 6000)}},
		fakeHealth{unhealthy: map[string]bool{"sfr-alko:a1": true}}, nil, cfg, Options{})
	res, err := svc.Resolve(context.Background(), core.ResolveRequest{Title: matrixTitle(), Client: webCaps()})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if res.Release.ID != "torrent-remux" {
		t.Fatalf("lecture %s, attendu le candidat suivant après l'échec de l'Alkodique", res.Release.ID)
	}
}

func TestMarkVerifiedFileGates(t *testing.T) {
	good := candidate{rel: alkoRelease("a1", "x", 1), langOK: true}
	tests := []struct {
		name       string
		mode       string
		mutate     func(*candidate)
		onDemand   bool
		first, tie bool
	}{
		{"éligible first", "first", func(*candidate) {}, true, true, false},
		{"éligible tie", "tie", func(*candidate) {}, true, false, true},
		{"off", "off", func(*candidate) {}, true, false, false},
		{"hors langue", "first", func(c *candidate) { c.langOK = false }, true, false, false},
		{"sous le score minimal", "first", func(c *candidate) { c.belowScore = true }, true, false, false},
		{"sous la qualité préférée", "first", func(c *candidate) { c.fallback = true }, true, false, false},
		{"règle avoid", "first", func(c *candidate) { c.policy.Fallback = 1 }, true, false, false},
		{"identité faible", "first", func(c *candidate) { c.identityWeak = true }, true, false, false},
		{"débit insuffisant", "first", func(c *candidate) { c.bandwidth = -1 }, true, false, false},
		{"travail spéculatif", "first", func(*candidate) {}, false, false, false},
		{"torrent ordinaire", "first", func(c *candidate) { c.rel = remuxTorrent() }, true, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := good
			tc.mutate(&c)
			markVerifiedFile(&c, tc.mode, tc.onDemand)
			if c.vfFirst != tc.first || c.vfTie != tc.tie {
				t.Fatalf("vfFirst=%v vfTie=%v, attendu %v %v", c.vfFirst, c.vfTie, tc.first, tc.tie)
			}
		})
	}
}
